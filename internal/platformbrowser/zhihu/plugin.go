package zhihu

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/viper"

	"wx_channel/internal/interceptor"
	"wx_channel/internal/interceptor/proxy"
)

var (
	cspNonceReg = regexp.MustCompile(`'nonce-([^']+)'`)
	titleTagReg = regexp.MustCompile(`(?i)<title[^>]*>`)

	zhihuCookieMu sync.Mutex
)

func CreatePlugin(onLoaded func(profile *Profile)) *proxy.Plugin {
	return &proxy.Plugin{
		Match: Match,
		OnRequest: func(ctx proxy.Context) {
			captureZhihuRequestCookie(ctx)
			if ctx.Req().URL.Path != "/__wx_channels_api/platform/browser" {
				return
			}
			body, err := io.ReadAll(ctx.Req().Body)
			if err != nil {
				fmt.Println("[ECHO]handler", err.Error())
			}
			profile, err := interceptor.NewPlatformBrowserProfile(body)
			if err != nil {
				fmt.Println("[ECHO]handler", err.Error())
			}
			if profile != nil {
				profile = FormatProfile(profile)
			}
			if profile != nil && onLoaded != nil {
				go onLoaded(profile)
			}
			if profile != nil {
				fmt.Printf("\n打开了%s内容\n%s\n", profile.PlatformName, profile.ContentTitle)
			}
			ctx.Mock(200, map[string]string{
				"Content-Type": "application/json",
			}, "{}")
		},
		OnResponse: func(ctx proxy.Context) {
			hostname := ctx.Req().URL.Hostname()
			if hostname == "www.zhihu.com" {
				ctx.SetResponseHeader("X-WX-Zhihu-Plugin", "matched")
			}
			captureZhihuCookie(ctx)
			respContentType := strings.ToLower(ctx.GetResponseHeader("Content-Type"))
			if hostname != "www.zhihu.com" || !strings.Contains(respContentType, "text/html") {
				return
			}
			ctx.SetResponseHeader("X-WX-Zhihu-Plugin-HTML", "matched")
			respBody, err := ctx.GetResponseBody()
			if err != nil {
				return
			}
			html := string(respBody)
			if !strings.Contains(strings.ToLower(html), "</body>") {
				return
			}
			csp := firstValue(
				ctx.GetResponseHeader("Content-Security-Policy-Report-Only"),
				ctx.GetResponseHeader("Content-Security-Policy"),
			)
			scriptAttr := ""
			if match := cspNonceReg.FindStringSubmatch(csp); len(match) > 1 {
				scriptAttr = fmt.Sprintf(` nonce="%s" reportloaderror`, match[1])
			}
			insertedScripts := ""
			insertedScripts += fmt.Sprintf(`<script%s>
(function(){
  var prefix = "[WX-ZHIHU-PLUGIN] ";
  function markTitle() {
    if (document.title && document.title.indexOf(prefix) !== 0) {
      document.title = prefix + document.title;
    }
  }
  markTitle();
  setTimeout(markTitle, 100);
  setTimeout(markTitle, 500);
  setTimeout(markTitle, 1500);
})();
</script>`, scriptAttr)
			for _, script := range Scripts() {
				insertedScripts += fmt.Sprintf(`<script%s>%s</script>`, scriptAttr, script)
			}
			if insertedScripts == "" {
				return
			}
			ctx.SetResponseHeader("X-WX-Zhihu-Plugin-Injected", "true")
			html = titleTagReg.ReplaceAllStringFunc(html, func(tag string) string {
				return tag + "[WX-ZHIHU-PLUGIN] "
			})
			html = "<!-- WX-ZHIHU-PLUGIN-INJECTED -->\n" + html
			html = strings.Replace(html, "</body>", insertedScripts+"</body>", 1)
			html = strings.Replace(html, "</BODY>", insertedScripts+"</BODY>", 1)
			ctx.SetResponseHeader("X-WX-Zhihu-Plugin-Body-Marker", strconv.FormatBool(strings.Contains(html, "WX-ZHIHU-PLUGIN-INJECTED")))
			ctx.SetResponseHeader("X-WX-Zhihu-Plugin-Body-Length", strconv.Itoa(len([]byte(html))))
			ctx.SetResponseBody(html)
		},
	}
}

func captureZhihuRequestCookie(ctx proxy.Context) {
	if ctx == nil || ctx.Req() == nil || ctx.Req().URL == nil || ctx.Req().Header == nil {
		return
	}
	hostname := ctx.Req().URL.Hostname()
	if hostname != "zhihu.com" && !strings.HasSuffix(hostname, ".zhihu.com") {
		return
	}
	setZhihuCookie(ctx.Req().Header.Get("Cookie"))
}

func captureZhihuCookie(ctx proxy.Context) {
	if ctx == nil || ctx.Req() == nil || ctx.Req().URL == nil {
		return
	}
	hostname := ctx.Req().URL.Hostname()
	if hostname != "zhihu.com" && !strings.HasSuffix(hostname, ".zhihu.com") {
		return
	}

	var cookieHeaders []string
	if res := ctx.Res(); res != nil && res.Header != nil {
		cookieHeaders = append(cookieHeaders, cookiesFromSetCookieHeaders(res.Header.Values("Set-Cookie"))...)
		cookieHeaders = append(cookieHeaders, cookiesFromSetCookieHeaders(res.Header.Values("SetCookie"))...)
		cookieHeaders = appendCookieHeader(cookieHeaders, res.Header.Get("Cookie"))
	}
	cookieHeaders = append(cookieHeaders, cookiesFromSetCookieHeaders([]string{ctx.GetResponseHeader("Set-Cookie")})...)
	cookieHeaders = append(cookieHeaders, cookiesFromSetCookieHeaders([]string{ctx.GetResponseHeader("SetCookie")})...)
	cookieHeaders = appendCookieHeader(cookieHeaders, ctx.GetResponseHeader("Cookie"))
	if len(cookieHeaders) == 0 {
		return
	}
	setZhihuCookie(cookieHeaders...)
}

func setZhihuCookie(cookieHeaders ...string) {
	zhihuCookieMu.Lock()
	defer zhihuCookieMu.Unlock()

	current := strings.TrimSpace(viper.GetString("zhihu.cookie"))
	merged := mergeCookieHeaders(current, cookieHeaders...)
	if merged != "" && merged != current {
		viper.Set("zhihu.cookie", merged)
		if err := viper.WriteConfig(); err != nil {
			fmt.Printf("[ZHIHU] save cookie config failed: %v\n", err)
		}
	}
}

func appendCookieHeader(headers []string, cookieHeader string) []string {
	cookieHeader = strings.TrimSpace(cookieHeader)
	if cookieHeader == "" {
		return headers
	}
	return append(headers, cookieHeader)
}

func cookiesFromSetCookieHeaders(setCookieHeaders []string) []string {
	header := http.Header{}
	for _, setCookie := range setCookieHeaders {
		setCookie = strings.TrimSpace(setCookie)
		if setCookie != "" {
			header.Add("Set-Cookie", setCookie)
		}
	}
	if len(header) == 0 {
		return nil
	}

	resp := http.Response{Header: header}
	cookies := resp.Cookies()
	cookieHeaders := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie.Name == "" {
			continue
		}
		cookieHeaders = append(cookieHeaders, cookie.Name+"="+cookie.Value)
	}
	return cookieHeaders
}

func mergeCookieHeaders(base string, cookieHeaders ...string) string {
	merged := make(map[string]string)
	var order []string
	for _, cookieHeader := range append([]string{base}, cookieHeaders...) {
		for _, cookie := range parseCookieHeader(cookieHeader) {
			if _, ok := merged[cookie.Name]; !ok {
				order = append(order, cookie.Name)
			}
			merged[cookie.Name] = cookie.Value
		}
	}

	parts := make([]string, 0, len(order))
	for _, name := range order {
		if value, ok := merged[name]; ok {
			parts = append(parts, name+"="+value)
		}
	}
	return strings.Join(parts, "; ")
}

func parseCookieHeader(cookieHeader string) []*http.Cookie {
	cookieHeader = strings.TrimSpace(cookieHeader)
	if cookieHeader == "" {
		return nil
	}
	req := http.Request{Header: http.Header{"Cookie": []string{cookieHeader}}}
	return req.Cookies()
}
