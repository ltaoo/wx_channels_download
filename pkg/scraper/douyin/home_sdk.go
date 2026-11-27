package douyin

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
)

//go:embed home_sdk.js
var douyin_home_sdk_environment string

// shortcut: These official SDK URLs match the working RSS signer; rediscover them if Douyin retires this version.
var douyin_home_bdms_urls = []string{
	"https://lf-c-flwb.bytetos.com/obj/rc-client-security/web/stable/1.0.1.19-fix.01/bdms.js",
	"https://p-pc-weboff.byteimg.com/tos-cn-i-9r5gewecjs/bdms_1.0.1.19_fix.js",
	"https://lf-headquarters-speed.yhgfb-cn-static.com/obj/rc-client-security/web/stable/1.0.1.19-fix.01/bdms.js",
}

const douyin_home_security_sdk_url = "https://lf-security.bytegoofy.com/obj/security-secsdk/runtime_bundler_34.js"

var douyin_home_sdk_cache struct {
	sync.Mutex
	sources []string
}

var err_home_signature_rejected = errors.New("douyin home API: signature rejected")

type douyin_home_signed_request struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

func (c *Client) load_home_sdk(refresh bool) ([]string, error) {
	douyin_home_sdk_cache.Lock()
	defer douyin_home_sdk_cache.Unlock()
	if refresh {
		douyin_home_sdk_cache.sources = nil
	}
	if douyin_home_sdk_cache.sources != nil {
		return douyin_home_sdk_cache.sources, nil
	}
	var bdms string
	var err error
	for _, sdk_url := range douyin_home_bdms_urls {
		bdms, err = c.download_home_sdk(sdk_url)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	security, err := c.download_home_sdk(douyin_home_security_sdk_url)
	if err != nil {
		return nil, err
	}
	douyin_home_sdk_cache.sources = []string{bdms, security}
	return douyin_home_sdk_cache.sources, nil
}

func (c *Client) download_home_sdk(sdk_url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sdk_url, nil)
	if err != nil {
		return "", fmt.Errorf("douyin home SDK: construct request: %w", err)
	}
	req.Header.Set("User-Agent", douyin_home_user_agent)
	req.Header.Set("Referer", "https://www.douyin.com/")
	// SDK downloads must never carry login cookies or follow redirects to other origins.
	http_client := *c.web.request_client()
	http_client.Jar = nil
	http_client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := http_client.Do(req)
	if err != nil {
		return "", fmt.Errorf("douyin home SDK: download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("douyin home SDK: HTTP %d", resp.StatusCode)
	}
	const max_sdk_bytes = 512000
	body, err := io.ReadAll(io.LimitReader(resp.Body, max_sdk_bytes+1))
	if err != nil {
		return "", fmt.Errorf("douyin home SDK: read: %w", err)
	}
	if len(body) == 0 || len(body) > max_sdk_bytes {
		return "", fmt.Errorf("douyin home SDK: invalid source size")
	}
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") || bytes.HasPrefix(bytes.TrimSpace(body), []byte("<")) {
		return "", fmt.Errorf("douyin home SDK: CDN returned HTML instead of JavaScript")
	}
	return string(body), nil
}

func sign_home_request(request_url string, cookie_header string, sources []string) (*douyin_home_signed_request, error) {
	if home_uifid(cookie_header) == "" {
		return nil, fmt.Errorf("抖音作品列表需要 UIFID 或 UIFID_TEMP Cookie，请同步 .douyin.com Cookie 或设置 douyin.cookie")
	}
	input, err := json.Marshal(map[string]string{
		"url": request_url, "cookie": cookie_header, "user_agent": douyin_home_user_agent,
	})
	if err != nil {
		return nil, fmt.Errorf("douyin home SDK: encode input: %w", err)
	}
	vm := goja.New()
	vm.SetMaxCallStackSize(1024)
	_ = vm.Set("__sdk_request", string(input))
	_ = vm.Set("__bdms_source", sources[0])
	_ = vm.Set("__security_source", sources[1])
	timer := time.AfterFunc(10*time.Second, func() { vm.Interrupt("SDK timeout") })
	defer timer.Stop()
	output, err := vm.RunScript("douyin-home-sdk.js", douyin_home_sdk_environment)
	if err != nil {
		phase := "environment"
		if value := vm.Get("__sdk_phase"); value != nil {
			switch value.String() {
			case "bdms_load", "bdms_sign", "security_load", "security_sign":
				phase = value.String()
			}
		}
		var interrupted *goja.InterruptedError
		if errors.As(err, &interrupted) {
			return nil, fmt.Errorf("douyin home SDK: %s 签名执行超时（10 秒）", phase)
		}
		// SDK exception messages may include cookies; expose only phase and source position.
		var exception *goja.Exception
		if errors.As(err, &exception) && len(exception.Stack()) > 0 {
			frame := exception.Stack()[0]
			position := frame.Position()
			return nil, fmt.Errorf("douyin home SDK: %s 签名执行失败（%T, %d:%d）", phase, err, position.Line, position.Column)
		}
		return nil, fmt.Errorf("douyin home SDK: %s 签名执行失败（%T）", phase, err)
	}
	var signed douyin_home_signed_request
	if err := json.Unmarshal([]byte(output.String()), &signed); err != nil {
		return nil, fmt.Errorf("douyin home SDK: invalid signature output")
	}
	target, err := url.Parse(request_url)
	if err != nil {
		return nil, fmt.Errorf("douyin home SDK: invalid request URL")
	}
	signed_url, err := url.Parse(signed.URL)
	if err != nil || signed_url.Scheme != "https" || signed_url.Host != target.Host || signed_url.Path != target.Path || signed_url.User != nil || signed_url.Fragment != "" || signed_url.Query().Get("a_bogus") == "" {
		return nil, fmt.Errorf("douyin home SDK: invalid signed URL")
	}
	allowed_headers := make(map[string]string, 3)
	for _, name := range []string{"uifid", "x-secsdk-web-signature", "x-secsdk-web-expire"} {
		value := signed.Headers[name]
		if value == "" || len(value) > 4096 || strings.ContainsAny(value, "\r\n\x00") {
			return nil, fmt.Errorf("douyin home SDK: invalid signature header %s", name)
		}
		allowed_headers[name] = value
	}
	signed.Headers = allowed_headers
	return &signed, nil
}

func home_uifid(cookie_header string) string {
	if uifid := home_cookie_value(cookie_header, "UIFID"); uifid != "" {
		return uifid
	}
	return home_cookie_value(cookie_header, "UIFID_TEMP")
}

func home_signature_rejected(status int, body []byte) bool {
	text := strings.TrimSpace(string(body))
	return status == http.StatusOK && text == "" ||
		(status == http.StatusOK || status == http.StatusForbidden) &&
			strings.EqualFold(strings.TrimRight(strings.TrimPrefix(text, "Blocked by "), ".!"), "ArgusSecurityPlugin Sign Invalid")
}
