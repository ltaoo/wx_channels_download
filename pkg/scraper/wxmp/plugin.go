package wxmp

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"wx_channel/frontend"
	"wx_channel/internal/interceptor/proxy"
)

var cspNonceReg = regexp.MustCompile(`'nonce-([^']+)'`)

type OfficialAccountArticleProfile struct {
	UniqueMark    string          `json:"unique_mark"`
	Title         string          `json:"title"`
	URL           string          `json:"url"`
	SourceURL     string          `json:"source_url"`
	CoverURL      string          `json:"cover_url"`
	Biz           string          `json:"biz"`
	Username      string          `json:"username"`
	Nickname      string          `json:"nickname"`
	AvatarURL     string          `json:"avatar_url"`
	Mid           string          `json:"mid"`
	Idx           string          `json:"idx"`
	Sn            string          `json:"sn"`
	RawCgiDataNew json.RawMessage `json:"cgiDataNew"`
}

func NewOfficialAccountArticleProfile(raw json.RawMessage) (*OfficialAccountArticleProfile, error) {
	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}

	profile := &OfficialAccountArticleProfile{
		Title:         jsonString(data, "title"),
		URL:           html.UnescapeString(jsonString(data, "link")),
		SourceURL:     html.UnescapeString(jsonString(data, "source_url")),
		CoverURL:      html.UnescapeString(jsonString(data, "cdn_url")),
		Biz:           jsonString(data, "bizuin"),
		Username:      jsonString(data, "user_name"),
		Nickname:      jsonString(data, "nick_name"),
		AvatarURL:     html.UnescapeString(firstOfficialAccountValue(jsonString(data, "round_head_img"), jsonString(data, "ori_head_img_url"), jsonString(data, "hd_head_img"))),
		Mid:           jsonScalarString(data, "mid"),
		Idx:           jsonScalarString(data, "idx"),
		Sn:            jsonString(data, "sn"),
		RawCgiDataNew: raw,
	}
	fillOfficialAccountArticleFromURL(profile)
	profile.UniqueMark = buildOfficialAccountArticleUniqueMark(profile)
	return profile, nil
}

func fillOfficialAccountArticleFromURL(profile *OfficialAccountArticleProfile) {
	if profile == nil || profile.URL == "" {
		return
	}
	u, err := url.Parse(profile.URL)
	if err != nil {
		return
	}
	query := u.Query()
	if profile.Biz == "" {
		profile.Biz = query.Get("__biz")
	}
	if profile.Mid == "" {
		profile.Mid = query.Get("mid")
	}
	if profile.Idx == "" {
		profile.Idx = query.Get("idx")
	}
	if profile.Sn == "" {
		profile.Sn = query.Get("sn")
	}
}

func buildOfficialAccountArticleUniqueMark(profile *OfficialAccountArticleProfile) string {
	parts := []string{profile.Biz, profile.Mid, profile.Idx, profile.Sn}
	allPresent := true
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			allPresent = false
			break
		}
	}
	if allPresent {
		return strings.Join(parts, "_")
	}
	return firstOfficialAccountValue(profile.URL, profile.SourceURL, profile.Title)
}

func jsonString(data map[string]json.RawMessage, key string) string {
	raw, ok := data[key]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	return ""
}

func jsonScalarString(data map[string]json.RawMessage, key string) string {
	if s := jsonString(data, key); s != "" {
		return s
	}
	raw, ok := data[key]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return n.String()
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return fmt.Sprintf("%t", b)
	}
	return ""
}

func firstOfficialAccountValue(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func CreateOfficialAccountArticleLoadedPlugin(onArticleLoaded func(profile *OfficialAccountArticleProfile)) *proxy.Plugin {
	return &proxy.Plugin{
		Match: "mp.weixin.qq.com",
		OnRequest: func(ctx proxy.Context) {
			if ctx.Req().URL.Path != "/__wx_channels_api/officialaccount/article" {
				return
			}
			body, err := io.ReadAll(ctx.Req().Body)
			if err != nil {
				fmt.Println("[ECHO]handler", err.Error())
			}
			profile, err := NewOfficialAccountArticleProfile(json.RawMessage(body))
			if err != nil {
				fmt.Println("[ECHO]handler", err.Error())
			}
			if profile != nil && onArticleLoaded != nil {
				go onArticleLoaded(profile)
			}
			if profile != nil {
				fmt.Printf("\n打开了公众号文章\n%s\n", profile.Title)
			}
			ctx.Mock(200, map[string]string{
				"Content-Type": "application/json",
			}, "{}")
		},
	}
}

func CreateOfficialAccountInterceptorPlugin(cfg *OfficialAccountConfig, files *frontend.ChannelInjectedFiles, version string) *proxy.Plugin {
	assetBaseURL := frontend.ChannelAssetsSameOriginBaseURL()
	return &proxy.Plugin{
		Match: "qq.com",
		OnRequest: func(ctx proxy.Context) {
			if ctx.Req().URL.Hostname() == "mp.weixin.qq.com" && (frontend.MockChannelStaticAsset(ctx, ctx.Req().URL.Path, files) || MockStaticAsset(ctx, ctx.Req().URL.Path)) {
				return
			}
		},
		OnResponse: func(ctx proxy.Context) {
			resp_content_type := strings.ToLower(ctx.GetResponseHeader("Content-Type"))
			hostname := ctx.Req().URL.Hostname()
			// pathname := ctx.Req().URL.Path
			if cfg.Enabled && hostname == "mp.weixin.qq.com" && strings.Contains(resp_content_type, "text/html") {
				resp_body, err := ctx.GetResponseBody()
				if err != nil {
					return
				}
				html := string(resp_body)
				csp := ctx.GetResponseHeader("Content-Security-Policy") + " " + ctx.GetResponseHeader("Content-Security-Policy-Report-Only")
				variables := buildOfficialAccountVariables(html)
				script_attr := ""
				style_attr := ""
				if match := cspNonceReg.FindStringSubmatch(csp); len(match) > 1 {
					script_attr = fmt.Sprintf(` nonce="%s" reportloaderror`, match[1])
					style_attr = fmt.Sprintf(` nonce="%s"`, match[1])
				}
				var injected strings.Builder
				if cfg.DebugShowError {
					/** 全局错误捕获并展示弹窗 */
					frontend.AppendScriptSrcs(&injected, script_attr, frontend.InjectAssetURL(assetBaseURL, "error.js"))
				}
				var shadcnCSS []byte
				if files != nil {
					shadcnCSS = files.CSSTimelessShadcn
				}
				frontend.AppendSharedLibAssetsWithInlineShadcnCSS(&injected, assetBaseURL, version, script_attr, style_attr, shadcnCSS)
				frontend.AppendStylesheetHrefs(&injected, style_attr, frontend.InjectAssetURL(assetBaseURL, "components.css"))
				cfg_byte, _ := json.Marshal(cfg)
				frontend.AppendInlineScript(&injected, script_attr, fmt.Sprintf(`var __wx_channels_config__ = %s; var __wx_channels_version__ = "%s";`, string(cfg_byte), version))
				frontend.AppendInlineScript(&injected, script_attr, fmt.Sprintf(`window.__wx_channels_env__ = Object.assign(window.__wx_channels_env__ || {}, { assetsBaseURL: %q });`, assetBaseURL))
				variable_byte, _ := json.Marshal(variables)
				frontend.AppendInlineScript(&injected, script_attr, fmt.Sprintf(`var WXVariable = %s;`, string(variable_byte)))
				frontend.AppendScriptSrcs(
					&injected,
					script_attr,
					frontend.InjectAssetURL(assetBaseURL, "eventbus.js"),
					frontend.InjectAssetURL(assetBaseURL, "env.js"),
					frontend.InjectAssetURL(assetBaseURL, "utils.js"),
					frontend.InjectAssetURL(assetBaseURL, "components.js"),
					frontend.InjectAssetURL(assetBaseURL, "virtual-list-view.js"),
					frontend.InjectAssetURL(assetBaseURL, "download/core.js"),
					ChannelInjectAssetURL(assetBaseURL, "mp.ws.js"),
				)
				if cfg.PagespyEnabled {
					/** 在线调试 */
					frontend.AppendScriptSrcs(&injected, script_attr, frontend.ChannelLibAssetURL(assetBaseURL, version, "pagespy.min.js"), frontend.InjectAssetURL(assetBaseURL, "pagespy.js"))
				}
				html = strings.Replace(html, "</body>", injected.String()+"</body>", 1)
				ctx.SetResponseBody(html)
				return
			}
		},
	}
}
