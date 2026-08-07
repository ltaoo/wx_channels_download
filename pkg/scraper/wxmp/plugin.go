package wxmp

import (
	"encoding/json"
	"fmt"
	"html"
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

func CreateOfficialAccountInterceptorPlugin(cfg *OfficialAccountConfig, version string) *proxy.Plugin {
	assetBaseURL := frontend.AssetsBaseURLFromConfig(cfg.Protocol, cfg.Hostname, cfg.Port)
	urlBuild := frontend.NewURLBuild(assetBaseURL, nil)
	assetVersion := version
	if assetVersion == "" {
		assetVersion = "static"
	}
	versionQuery := url.Values{"v": []string{assetVersion}}
	return &proxy.Plugin{
		Match: "qq.com",
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
					/** Global error capture and show dialog */
					frontend.AppendScripts(&injected, script_attr, urlBuild("/inject/error.js"))
				}
				frontend.AppendScripts(&injected, script_attr, urlBuild("/public/timeless/0.30.0/timeless.umd.min.js", versionQuery))
				frontend.AppendScripts(&injected, script_attr, urlBuild("/public/timeless/0.30.0/timeless.utils.umd.min.js", versionQuery))
				frontend.AppendStylesheets(&injected, style_attr, urlBuild("/public/timeless/0.30.0/timeless.weui.css", versionQuery))
				frontend.AppendScripts(&injected, script_attr, urlBuild("/public/timeless/0.30.0/timeless.weui.umd.min.js", versionQuery))
				frontend.AppendScripts(&injected, script_attr, urlBuild("/public/timeless/0.30.0/timeless.dom.umd.min.js", versionQuery))
				frontend.AppendScripts(&injected, script_attr, urlBuild("/public/timeless/0.30.0/timeless.web.umd.min.js", versionQuery))
				frontend.AppendStylesheets(&injected, style_attr, urlBuild("/inject/components.css"))
				frontend_config := make(map[string]any, len(variables)+2)
				cfg_byte, _ := json.Marshal(cfg)
				_ = json.Unmarshal(cfg_byte, &frontend_config)
				for key, value := range variables {
					frontend_config[key] = value
				}
				frontend_config["version"] = version
				frontend_config["assets_base_url"] = assetBaseURL
				frontend_config_byte, _ := json.Marshal(frontend_config)
				frontend.AppendInlineScript(
					&injected,
					script_attr,
					fmt.Sprintf(`window.__d_config = %s;`, frontend_config_byte),
				)
				frontend.AppendScripts(
					&injected,
					script_attr,
					urlBuild("/inject/eventbus.js"),
					urlBuild("/inject/env.js"),
					urlBuild("/inject/utils.js"),
					urlBuild("/inject/components.js"),
					urlBuild("/inject/virtual-list-view.js"),
					urlBuild("/inject/download/model.js"),
					urlBuild("/inject/download/view.js"),
					ChannelInjectAssetURL(assetBaseURL, "mp.ws.js"),
				)
				if cfg.PagespyEnabled {
					/** Online debugging */
					frontend.AppendScripts(&injected, script_attr, urlBuild("/lib/pagespy.min.js", versionQuery), urlBuild("/inject/pagespy.js"))
				}
				if cfg.InjectContentScript != "" {
					frontend.AppendInlineScript(&injected, script_attr, cfg.InjectContentScript)
				}
				html = strings.Replace(html, "</body>", injected.String()+"</body>", 1)
				ctx.SetResponseBody(html)
				return
			}
		},
	}
}
