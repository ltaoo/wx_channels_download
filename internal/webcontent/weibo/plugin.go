package weibo

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/spf13/viper"

	"wx_channel/internal/interceptor"
	"wx_channel/internal/interceptor/proxy"
)

var weiboCookieMu sync.Mutex

func CreatePlugin(onLoaded func(profile *Profile)) *proxy.Plugin {
	base := interceptor.CreatePlatformBrowserPluginWithScript(Match, Script(), func(profile *interceptor.PlatformBrowserProfile) {
		if onLoaded != nil {
			onLoaded(FormatProfile(profile))
		}
	})
	return &proxy.Plugin{
		Match: Match,
		OnRequest: func(ctx proxy.Context) {
			captureWeiboRequestCookie(ctx)
			if base != nil && base.OnRequest != nil {
				base.OnRequest(ctx)
			}
		},
		OnResponse: func(ctx proxy.Context) {
			captureWeiboCookie(ctx)
			if base != nil && base.OnResponse != nil {
				base.OnResponse(ctx)
			}
		},
	}
}

func captureWeiboRequestCookie(ctx proxy.Context) {
	if ctx == nil || ctx.Req() == nil || ctx.Req().URL == nil || ctx.Req().Header == nil {
		return
	}
	if !isWeiboHost(ctx.Req().URL.Hostname()) {
		return
	}
	setWeiboCookie(ctx.Req().Header.Get("Cookie"))
}

func captureWeiboCookie(ctx proxy.Context) {
	if ctx == nil || ctx.Req() == nil || ctx.Req().URL == nil {
		return
	}
	if !isWeiboHost(ctx.Req().URL.Hostname()) {
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
	setWeiboCookie(cookieHeaders...)
}

func setWeiboCookie(cookieHeaders ...string) {
	weiboCookieMu.Lock()
	defer weiboCookieMu.Unlock()

	current := strings.TrimSpace(viper.GetString("weibo.cookie"))
	merged := mergeCookieHeaders(current, cookieHeaders...)
	if merged != "" && merged != current {
		viper.Set("weibo.cookie", merged)
		if err := viper.WriteConfig(); err != nil {
			fmt.Printf("[WEIBO] save cookie config failed: %v\n", err)
		}
	}
}

func isWeiboHost(hostname string) bool {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	return hostname == "weibo.com" || hostname == "www.weibo.com" || strings.HasSuffix(hostname, ".weibo.com")
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
