package youku

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	sourceutil "wx_channel/pkg/scraper/internal/source"
)

var signing_script_path = regexp.MustCompile(`^/(?:AWSC/AWSC/awsc\.js|AWSC/uab/[0-9]+(?:\.[0-9]+){1,3}/collina\.js|kplayer/kuplayer/[A-Za-z0-9.-]+/index\.js)$`)
var uab_module_path = regexp.MustCompile(`\buabModule\s*:\s*\{[^{}]*\bstable\s*:\s*\[\s*"([^"]+)"`)
var player_version_pattern = regexp.MustCompile(`this\.version="([0-9]+(?:\.[0-9]+){2,3})",this\.(?:interactiveMode|system)`)
var player_ccode_pattern = regexp.MustCompile(`\b[A-Za-z_$][A-Za-z0-9_$]*=\{ccode:"([0-9]{4})",autoplay:`)

// Official SDK addresses verified from the play page. Used only when neither
// public page publishes its assets (for example, while returning a gate page).
// Version, client code and ckey are still read/computed from fetched JavaScript.
const fallback_awsc_address = "https://g.alicdn.com/AWSC/AWSC/awsc.js"
const fallback_player_address = "https://g.alicdn.com/kplayer/kuplayer/4.23.3-xmedia/index.js"

func check_script_address(address string) error {
	link, err := url.Parse(address)
	if err != nil || link.Scheme != "https" || link.Hostname() != "g.alicdn.com" || link.User != nil || link.Fragment != "" || (link.Port() != "" && link.Port() != "443") || link.RawPath != "" || len(address) > 2048 || !signing_script_path.MatchString(link.Path) {
		return errors.New("优酷签名脚本地址不属于官方允许的 SDK")
	}
	return nil
}

func player_script_addresses(page, text string) (string, string, error) {
	base, err := url.Parse(page)
	if err != nil {
		return "", "", err
	}
	var awsc, player string
	tokens := html.NewTokenizer(strings.NewReader(text))
	for {
		token_type := tokens.Next()
		if token_type == html.ErrorToken {
			if tokens.Err() != io.EOF {
				return "", "", errors.New("优酷播放页 HTML 无效")
			}
			break
		}
		if token_type != html.StartTagToken {
			continue
		}
		token := tokens.Token()
		if token.Data != "script" {
			continue
		}
		for _, attribute := range token.Attr {
			if attribute.Key != "src" {
				continue
			}
			link, err := url.Parse(attribute.Val)
			if err != nil {
				continue
			}
			link = base.ResolveReference(link)
			address := link.String()
			if check_script_address(address) != nil {
				continue
			}
			if link.Path == "/AWSC/AWSC/awsc.js" {
				awsc = address
			} else if strings.HasPrefix(link.Path, "/kplayer/kuplayer/") {
				player = address
			}
		}
	}
	if awsc == "" || player == "" {
		return "", "", errors.New("优酷播放页未提供签名 SDK 或播放器 JS 地址")
	}
	return awsc, player, nil
}

// Script traffic is always anonymous and cannot redirect outside the exact SDK
// host/path boundary, even when the source client carries an authenticated jar.
func (client *backend) fetch_script(ctx context.Context, address string) (string, error) {
	if err := check_script_address(address); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return "", errors.New("优酷签名脚本地址无效")
	}
	req.Header.Set("User-Agent", sourceutil.FirstText(client.UserAgent, sourceutil.UserAgent))
	req.Header.Set("Referer", client.Referer)
	http_client := *client.HTTPClient
	http_client.Jar = nil
	http_client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		next.Header.Del("Cookie")
		if len(via) >= 5 {
			return errors.New("优酷签名脚本跳转次数过多")
		}
		return check_script_address(next.URL.String())
	}
	response, err := http_client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", errors.New("无法获取优酷官网签名脚本")
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return "", errors.New("优酷官网签名脚本请求失败")
	}
	return sourceutil.ReadVideoResponse(response)
}
