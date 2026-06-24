package interceptor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"

	"wx_channel/internal/interceptor/proxy"
	"wx_channel/pkg/util"
)

var (
	// HTML处理相关正则
	scriptSrcReg  = regexp.MustCompile(`src="([^"]{1,})\.js"`)
	scriptHrefReg = regexp.MustCompile(`href="([^"]{1,})\.js"`)

	// JavaScript处理相关正则
	jsDepReg         = regexp.MustCompile(`"js/([^"]{1,})\.js"`)
	jsFromReg        = regexp.MustCompile(`from {0,1}"([^"]{1,})\.js"`)
	jsLazyImportReg  = regexp.MustCompile(`import\("([^"]{1,})\.js"\)`)
	jsImportReg      = regexp.MustCompile(`import {0,1}"([^"]{1,})\.js"`)
	jsExportReg      = regexp.MustCompile(`exports?\s*\{`)
	jsExportBlockReg = regexp.MustCompile(`exports?\s*\{([^}]*)\}`)

	// 特定路径的正则
	jsSourceBufferReg                   = regexp.MustCompile(`this.sourceBuffer.appendBuffer\(([a-zA-Z]{1,})\),`)
	jsInitReg                           = regexp.MustCompile(`async finderInit\(\)\{(.*?)\}async`)
	jsFeedProfileReg                    = regexp.MustCompile(`async finderGetCommentDetail\((\w+)\)\{(.*?)\}async`)
	jsCommentListReg                    = regexp.MustCompile(`async finderGetCommentList\((\w+)\)\{(.*?)\}async`)
	jsPCFlowReg                         = regexp.MustCompile(`async finderPcFlow\((\w+)\)\{(.*?)\}async`)
	jsLiveInfoReg                       = regexp.MustCompile(`async finderGetLiveInfo\((\w+)\)\{(.*?)\}async`)
	jsLiveFeedListReg                   = regexp.MustCompile(`async finderLiveUserPage\((\w+)\)\{(.*?)\}async`)
	jsJoinLiveReg                       = regexp.MustCompile(`async joinLive\((\w+)\)\{(.*?)\}async`)
	jsRecommendFeedsReg                 = regexp.MustCompile(`async finderGetRecommend\((\w+)\)\{(.*?)\}async`)
	jsUserFeedsReg                      = regexp.MustCompile(`async finderUserPage\((\w+)\)\{(.*?)\}async`)
	jsFinderPCSearchReg                 = regexp.MustCompile(`async finderPCSearch\((\w+)\)\{(.*?)\}async`)
	jsFinderSearchReg                   = regexp.MustCompile(`async finderSearch\((\w+)\)\{(.*?)\}async`)
	jsFinderGetInteractionedFeedListReg = regexp.MustCompile(`async finderGetInteractionedFeedList\((\w+)\)\{(.*?)\}\}const`)
	jsFinderGetFeedH5Url                = regexp.MustCompile(`async finderGetFeedH5Url\((\w+)\)\{(.*?)\}\}const`)
	jsGoToPrevFlowReg                   = regexp.MustCompile(`goToPrevFlowFeed:([a-zA-Z_$]{1,})`)
	jsGoToNextFlowReg                   = regexp.MustCompile(`goToNextFlowFeed:([a-zA-Z_$]{1,})`)
	jsFlowTabReg                        = regexp.MustCompile(`flowTab:([a-zA-Z_$]{1,})`)
	jsLocalFlowTabReg                   = regexp.MustCompile(`localFlowTab:([a-zA-Z]{1,})`)
	jsLoadLocalPlaylistReg              = regexp.MustCompile(`loadLocalPlaylist:([a-zA-Z]{1,})`)
)

const channelAssetsPath = "/__wx_channels_assets"
const ChannelLibAssetCacheControl = "public, max-age=2592000, immutable"
const ChannelSrcAssetCacheControl = "no-cache"
const channelLibAssetCacheControl = ChannelLibAssetCacheControl
const channelSrcAssetCacheControl = ChannelSrcAssetCacheControl

const timelessBridgeScript = `Object.assign(Timeless, Timeless.shadcn.kit);
Timeless.ui = Timeless.shadcn.ui;
Object.assign(window, Timeless);
Object.assign(window, Timeless.shadcn);`

func ChannelAssetsBaseURL(protocol string, hostname string, port int) string {
	protocol = strings.TrimSpace(protocol)
	protocol = strings.TrimSuffix(protocol, "://")
	protocol = strings.TrimSuffix(protocol, ":")
	if protocol == "" {
		protocol = "http"
	}
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		hostname = "127.0.0.1"
	}
	host, embeddedPort, err := net.SplitHostPort(hostname)
	if err == nil {
		host = normalizeChannelAssetHostname(host)
		host = net.JoinHostPort(host, embeddedPort)
	} else {
		host = normalizeChannelAssetHostname(hostname)
		if port > 0 {
			host = net.JoinHostPort(host, strconv.Itoa(port))
		}
	}
	return (&url.URL{
		Scheme: protocol,
		Host:   host,
		Path:   channelAssetsPath,
	}).String()
}

func normalizeChannelAssetHostname(hostname string) string {
	hostname = strings.TrimSpace(hostname)
	if strings.HasPrefix(hostname, "[") && strings.HasSuffix(hostname, "]") {
		hostname = strings.TrimPrefix(strings.TrimSuffix(hostname, "]"), "[")
	}
	switch hostname {
	case "", "0.0.0.0", "::":
		return "127.0.0.1"
	default:
		return hostname
	}
}

func ChannelAssetsBaseURLFromConfig(cfg *InterceptorConfig) string {
	if cfg == nil {
		return ChannelAssetsBaseURL("", "", 0)
	}
	return ChannelAssetsBaseURL(cfg.APIServerProtocol, cfg.APIServerHostname, cfg.APIServerPort)
}

func ChannelLibAssetURL(baseURL string, version string, rel string) string {
	if version == "" {
		version = "static"
	}
	return strings.TrimRight(baseURL, "/") + "/lib/" + rel + "?v=" + url.QueryEscape(version)
}

func ChannelSrcAssetURL(baseURL string, rel string) string {
	return strings.TrimRight(baseURL, "/") + "/src/" + rel
}

func AppendScriptSrcs(b *strings.Builder, attr string, srcs ...string) {
	for _, src := range srcs {
		if src == "" {
			continue
		}
		b.WriteString(fmt.Sprintf(`<script%s src="%s"></script>`, attr, src))
	}
}

func AppendStylesheetHrefs(b *strings.Builder, attr string, hrefs ...string) {
	for _, href := range hrefs {
		if href == "" {
			continue
		}
		b.WriteString(fmt.Sprintf(`<link%s rel="stylesheet" href="%s">`, attr, href))
	}
}

func AppendInlineScript(b *strings.Builder, attr string, script string) {
	if script == "" {
		return
	}
	b.WriteString(fmt.Sprintf(`<script%s>%s</script>`, attr, script))
}

func AppendSharedLibAssets(b *strings.Builder, baseURL string, version string, scriptAttr string, styleAttr string) {
	AppendScriptSrcs(
		b,
		scriptAttr,
		ChannelLibAssetURL(baseURL, version, "mitt.umd.js"),
		ChannelLibAssetURL(baseURL, version, "timeless/0.26.3/timeless.umd.min.js"),
		ChannelLibAssetURL(baseURL, version, "timeless/0.26.3/timeless.utils.umd.min.js"),
	)
	AppendStylesheetHrefs(b, styleAttr, ChannelLibAssetURL(baseURL, version, "timeless/0.26.3/timeless.shadcn.css"))
	AppendScriptSrcs(b, scriptAttr, ChannelLibAssetURL(baseURL, version, "timeless/0.26.3/timeless.shadcn.umd.min.js"))
	AppendInlineScript(b, scriptAttr, timelessBridgeScript)
	AppendScriptSrcs(
		b,
		scriptAttr,
		ChannelLibAssetURL(baseURL, version, "timeless/0.26.3/timeless.dom.umd.min.js"),
		ChannelLibAssetURL(baseURL, version, "timeless/0.26.3/timeless.web.umd.min.js"),
	)
}

func mockChannelStaticAsset(ctx proxy.Context, pathname string, files *ChannelInjectedFiles) bool {
	if rel, ok := channelStaticAssetRel(pathname, "lib"); ok {
		data, err := files.ReadLib(rel)
		if err != nil {
			return false
		}
		ctx.Mock(200, map[string]string{
			"Content-Type":  channelStaticAssetContentType(rel),
			"Cache-Control": channelLibAssetCacheControl,
		}, string(data))
		return true
	}
	if rel, ok := channelStaticAssetRel(pathname, "src"); ok {
		data, err := files.ReadSrc(rel)
		if err != nil {
			return false
		}
		etag := channelStaticAssetETag(data)
		headers := map[string]string{
			"Content-Type":  channelStaticAssetContentType(rel),
			"Cache-Control": channelSrcAssetCacheControl,
			"ETag":          etag,
		}
		if req := ctx.Req(); req != nil && req.Header != nil {
			if strings.Contains(req.Header.Get("If-None-Match"), etag) {
				ctx.Mock(304, headers, "")
				return true
			}
		}
		ctx.Mock(200, headers, string(data))
		return true
	}
	return false
}

func channelStaticAssetRel(pathname string, dir string) (string, bool) {
	marker := "/" + dir + "/"
	idx := strings.LastIndex(pathname, marker)
	if idx < 0 {
		trimmed := strings.TrimPrefix(pathname, "/")
		prefix := dir + "/"
		if !strings.HasPrefix(trimmed, prefix) {
			return "", false
		}
		rel := strings.TrimPrefix(trimmed, prefix)
		if rel == "" || strings.Contains(rel, "..") {
			return "", false
		}
		return rel, true
	}
	rel := pathname[idx+len(marker):]
	if rel == "" || strings.Contains(rel, "..") {
		return "", false
	}
	return rel, true
}

func channelStaticAssetContentType(rel string) string {
	return ChannelStaticAssetContentType(rel)
}

func ChannelStaticAssetContentType(rel string) string {
	switch {
	case strings.HasSuffix(rel, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(rel, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(rel, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(rel, ".json"):
		return "application/json; charset=utf-8"
	default:
		return "text/plain; charset=utf-8"
	}
}

func channelStaticAssetETag(data []byte) string {
	return ChannelStaticAssetETag(data)
}

func ChannelStaticAssetETag(data []byte) string {
	hash := sha256.Sum256(data)
	return `"` + hex.EncodeToString(hash[:]) + `"`
}

func CreateChannelInterceptorPlugins(interceptor *Interceptor, files *ChannelInjectedFiles) []*proxy.Plugin {
	version := interceptor.Version
	cfg := interceptor.Settings
	variables := interceptor.FrontendVariables
	assetBaseURL := ChannelAssetsBaseURLFromConfig(cfg)
	v := "?t=" + version
	plugin1 := &proxy.Plugin{
		Match: "channels.weixin.qq.com",
		OnRequest: func(ctx proxy.Context) {
			pathname := ctx.Req().URL.Path
			if mockChannelStaticAsset(ctx, pathname, files) {
				return
			}
			if pathname == "/__wx_channels_api/profile" {
				var data ChannelMediaProfile
				if err := json.NewDecoder(ctx.Req().Body).Decode(&data); err != nil {
					fmt.Println("[ECHO]handler", err.Error())
				}
				fmt.Printf("\n打开了视频\n%s\n", data.Title)
				ctx.Mock(200, map[string]string{
					"Content-Type": "application/json",
				}, "{}")
				return
			}
			if pathname == "/__wx_channels_api/tip" {
				var data FrontendTip
				if err := json.NewDecoder(ctx.Req().Body).Decode(&data); err != nil {
					fmt.Println("[ECHO]handler", err.Error())
				}
				prefix_text := "[FRONTEND]"
				prefix := data.Prefix
				if prefix == nil {
					prefix = &prefix_text
				}
				if data.End == 1 {
					fmt.Println()
				} else if data.Replace == 1 {
					fmt.Printf("\r\033[K%v%s", *prefix, data.Msg)
				} else if data.IgnorePrefix == 1 {
					fmt.Printf("%s\n", data.Msg)
				} else {
					fmt.Printf("%v%s\n", *prefix, data.Msg)
				}
				ctx.Mock(200, map[string]string{
					"Content-Type": "application/json",
				}, "{}")
				return
			}
			if pathname == "/__wx_channels_api/error" {
				var data FrontendErrorTip
				if err := json.NewDecoder(ctx.Req().Body).Decode(&data); err != nil {
					fmt.Println("[ECHO]handler", err.Error())
				}
				prefix_text := "[FRONTEND ERROR]"
				color.Red(fmt.Sprintf("%v%s\n", prefix_text, data.Msg))
				ctx.Mock(200, map[string]string{
					"Content-Type": "application/json",
				}, "{}")
				return
			}
		},
		OnResponse: func(ctx proxy.Context) {
			resp_content_type := strings.ToLower(ctx.GetResponseHeader("Content-Type"))
			hostname := ctx.Req().URL.Hostname()
			pathname := ctx.Req().URL.Path
			// fmt.Println("response1", hostname, pathname)
			if pathname == "/web/pages/feed" && cfg.ChannelsDisableLocationToHome && ctx.Res().StatusCode == 302 {
				original_req := ctx.Req()
				u := &url.URL{Scheme: "https", Host: original_req.URL.Hostname(), Path: pathname, RawQuery: original_req.URL.RawQuery}
				q := u.Query()
				q.Set("flow", "2")
				q.Set("fpid", "FinderLike")
				q.Set("bus", util.NowSecondsStr())
				q.Set("entrance_id", "1002")
				q.Set("wx_header", "0")
				u.RawQuery = q.Encode()
				req, err := http.NewRequest(http.MethodGet, u.String(), original_req.Body)
				if err == nil {
					for k, v := range original_req.Header {
						for _, vv := range v {
							req.Header.Add(k, vv)
						}
					}
					req.Header.Del("Accept-Encoding")
					client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 10 * time.Second}
					if resp2, err2 := client.Do(req); err2 == nil {
						defer resp2.Body.Close()
						body2, _ := io.ReadAll(resp2.Body)
						ct := resp2.Header.Get("Content-Type")
						lct := strings.ToLower(ct)
						if ct == "" || strings.Contains(lct, "text/html") {
							ct = "text/html; charset=utf-8"
						}
						ctx.SetStatusCode(200)
						ctx.Res().Header.Del("Content-Encoding")
						ctx.Res().Header.Del("Content-Length")
						ctx.SetResponseHeader("Content-Type", ct)
						ctx.SetResponseBody(string(body2))
						resp_content_type = strings.ToLower(ct)
					}
				}
			}
			if hostname == "channels.weixin.qq.com" && strings.Contains(resp_content_type, "text/html") {
				resp_body, err := ctx.GetResponseBody()
				if err != nil {
					fmt.Println("[error]get response body failed,", err)
					return
				}
				html := string(resp_body)
				html = scriptSrcReg.ReplaceAllString(html, `src="$1.js`+v+`"`)
				html = scriptHrefReg.ReplaceAllString(html, `href="$1.js`+v+`"`)

				var injected strings.Builder
				if cfg.DebugShowError {
					/** 全局错误捕获并展示弹窗 */
					AppendScriptSrcs(&injected, "", ChannelSrcAssetURL(assetBaseURL, "error.js"))
				}
				AppendSharedLibAssets(&injected, assetBaseURL, version, "", "")
				cfg_byte, _ := json.Marshal(cfg)
				AppendInlineScript(&injected, "", fmt.Sprintf(`var __wx_channels_config__ = %s; var __wx_channels_version__ = "%s";`, string(cfg_byte), version))
				variable_byte, _ := json.Marshal(variables)
				AppendInlineScript(&injected, "", fmt.Sprintf(`var WXVariable = %s;`, string(variable_byte)))
				AppendScriptSrcs(
					&injected,
					"",
					ChannelSrcAssetURL(assetBaseURL, "eventbus.js"),
					ChannelSrcAssetURL(assetBaseURL, "env.js"),
					ChannelSrcAssetURL(assetBaseURL, "env.channels.js"),
					ChannelSrcAssetURL(assetBaseURL, "utils.js"),
					ChannelSrcAssetURL(assetBaseURL, "components.js"),
				)
				AppendScriptSrcs(
					&injected,
					"",
					ChannelSrcAssetURL(assetBaseURL, "download/core.js"),
					ChannelSrcAssetURL(assetBaseURL, "download/panel.js"),
				)
				if cfg.InjectGlobalScript != "" {
					AppendInlineScript(&injected, "", cfg.InjectGlobalScript)
				}
				// 必须放在 JSUtils 后面
				if cfg.PagespyEnabled {
					/** 在线调试 */
					AppendScriptSrcs(&injected, "", ChannelLibAssetURL(assetBaseURL, version, "pagespy.min.js"), ChannelSrcAssetURL(assetBaseURL, "pagespy.js"))
				}
				if pathname == "/web/pages/home" {
					AppendScriptSrcs(&injected, "", ChannelSrcAssetURL(assetBaseURL, "home.js"))
					if cfg.InjectExtraScriptAfterJSMain != "" {
						AppendInlineScript(&injected, "", cfg.InjectExtraScriptAfterJSMain)
					}
				}
				if pathname == "/web/pages/feed" {
					AppendScriptSrcs(&injected, "", ChannelSrcAssetURL(assetBaseURL, "feed.js"))
					if cfg.InjectExtraScriptAfterJSMain != "" {
						AppendInlineScript(&injected, "", cfg.InjectExtraScriptAfterJSMain)
					}
				}
				if pathname == "/web/pages/live" {
					AppendScriptSrcs(&injected, "", ChannelSrcAssetURL(assetBaseURL, "live.js"))
					if cfg.InjectExtraScriptAfterJSMain != "" {
						AppendInlineScript(&injected, "", cfg.InjectExtraScriptAfterJSMain)
					}
				}
				if pathname == "/web/pages/profile" {
					AppendScriptSrcs(&injected, "", ChannelSrcAssetURL(assetBaseURL, "profile.js"))
					if cfg.InjectExtraScriptAfterJSMain != "" {
						AppendInlineScript(&injected, "", cfg.InjectExtraScriptAfterJSMain)
					}
				}
				html = strings.Replace(html, "<head>", "<head>\n"+injected.String(), 1)
				ctx.SetResponseBody(html)
				return
			}
		},
	}
	plugin2 := &proxy.Plugin{
		Match: "res.wx.qq.com",
		OnResponse: func(ctx proxy.Context) {
			resp_content_type := strings.ToLower(ctx.GetResponseHeader("Content-Type"))
			hostname := ctx.Req().URL.Hostname()
			pathname := ctx.Req().URL.Path
			if hostname == "res.wx.qq.com" && strings.Contains(resp_content_type, "application/javascript") {
				if util.Includes(pathname, "wasm_video_decode") {
					return
				}
				resp_body, err := ctx.GetResponseBody()
				if err != nil {
					fmt.Println("[error]GetResponseBody error", err)
					return
				}
				// fmt.Println("response2", hostname, pathname, resp_content_type, ctx.Res().StatusCode)
				js_script := string(resp_body)
				js_script = jsFromReg.ReplaceAllString(js_script, `from"$1.js`+v+`"`)
				js_script = jsDepReg.ReplaceAllString(js_script, `"js/$1.js`+v+`"`)
				js_script = jsLazyImportReg.ReplaceAllString(js_script, `import("$1.js`+v+`")`)
				js_script = jsImportReg.ReplaceAllString(js_script, `import"$1.js`+v+`"`)

				if strings.Contains(pathname, "index.publish") {
					// 已经废弃了
					buffer_js := `(() => {
					WXU.append_media_buf($1);
					})(),this.sourceBuffer.appendBuffer($1),`
					js_script = jsSourceBufferReg.ReplaceAllString(js_script, buffer_js)
					ctx.SetResponseBody(js_script)
					return
				}
				if strings.Contains(pathname, "virtual_svg-icons-register.publish") {
					{
						js_init := `async finderInit() {
					var result = await (async () => {
						$1;
					})();
					var data = result.data;
					// console.log("before Init", data);
					WXU.emit(WXU.Events.Init, data);
					return result;
				}async`
						js_script = jsInitReg.ReplaceAllString(js_script, js_init)
					}
					{
						js_pc_flow := `async finderPcFlow($1) {
					var result = await (async () => {
						$2;
					})();
					var feeds = result.data.object;
					// console.log("before PCFlowLoaded", result.data);
					WXU.emit(WXU.Events.PCFlowLoaded, feeds);
					return result;
				}async`
						js_script = jsPCFlowReg.ReplaceAllString(js_script, js_pc_flow)
					}
					{
						js_recommend_feeds := `async finderGetRecommend($1) {
					var result = await (async () => {
						$2;
					})();
					var feeds = result.data.object;
					// console.log("before RecommendFeedsLoaded", result.data);
					WXU.emit(WXU.Events.RecommendFeedsLoaded, feeds);
					return result;
				}async`
						js_script = jsRecommendFeedsReg.ReplaceAllString(js_script, js_recommend_feeds)
					}
					{
						js_feed_profile := `async finderGetCommentDetail($1) {
					var result = await (async () => {
						$2;
					})();
					var feed = result.data.object;
					// console.log("before FeedProfileLoaded", result.data);
					WXU.emit(WXU.Events.FeedProfileLoaded, feed);
					return result;
				}async`
						js_script = jsFeedProfileReg.ReplaceAllString(js_script, js_feed_profile)
					}
					{
						js_comment_list := `async finderGetCommentList($1) {
					var result = await (async () => {
						$2;
					})();
					// console.log("before CommentListLoaded", result.data, $1);
					WXU.emit(WXU.Events.FeedCommentListLoaded, result.data);
					return result;
				}async`
						js_script = jsCommentListReg.ReplaceAllString(js_script, js_comment_list)
					}
					{
						js_finder_pc_search := `async finderPCSearch($1) {
					var result = await (async () => {
						$2;
					})();
					// console.log("before finderPCSearch", result, $1);
					return result;
				}async`
						js_script = jsFinderPCSearchReg.ReplaceAllString(js_script, js_finder_pc_search)
					}
					{
						js_finder_search := `async finderSearch($1) {
					var result = await (async () => {
						$2;
					})();
					// console.log("before finderSearch", result, $1);
					return result;
				}async`
						js_script = jsFinderSearchReg.ReplaceAllString(js_script, js_finder_search)
					}
					{
						js_finder_interactioned := `async finderGetInteractionedFeedList($1) {
						var result = await (async () => {
							$2;
						})();
						var feeds = result.data.object;
						// console.log("before finderGetInteractionedFeedList", result, $1);
						WXU.emit(WXU.Events.InteractionedFeedsLoaded, feeds);
						return result;
					}}const`
						js_script = jsFinderGetInteractionedFeedListReg.ReplaceAllString(js_script, js_finder_interactioned)
					}
					{
						js_finder_feed_h5_url := `async finderGetFeedH5Url($1) {
						var result = await (async () => {
							$2;
						})();
						var data = result.data.object;
						// console.log("before finderGetFeedH5Url", result, $1);
						WXU.emit(WXU.Events.GetFeedH5Url, data);
						return result;
					}}const`
						js_script = jsFinderGetFeedH5Url.ReplaceAllString(js_script, js_finder_feed_h5_url)
					}
					{
						js_user_feed := `async finderUserPage($1) {
					var result = await (async () => {
						$2;
					})();
					var feeds = result.data.object;
					// console.log("before UserFeedsLoaded", result.data, $1);
					WXU.emit(WXU.Events.UserFeedsLoaded, feeds);
					return result;
				}async`
						js_script = jsUserFeedsReg.ReplaceAllString(js_script, js_user_feed)
					}
					{
						js_live_feed_list := `async finderLiveUserPage($1) {
						var result = await (async () => {
							$2;
						})();
						var feeds = result.data.object;
						// console.log("before LiveUserFeedsLoaded", result.data, $1);
						WXU.emit(WXU.Events.LiveUserFeedsLoaded, feeds);
						return result;
					}async`
						js_script = jsLiveFeedListReg.ReplaceAllString(js_script, js_live_feed_list)
					}
					{
						js_live_profile := `async finderGetLiveInfo($1) {
					var result = await (async () => {
						$2;
					})();
					var live = result.data;
					// console.log("before LiveProfileLoaded", result.data);
					WXU.emit(WXU.Events.LiveProfileLoaded, live);
					return result;
				}async`
						js_script = jsLiveInfoReg.ReplaceAllString(js_script, js_live_profile)
					}
					{
						js_join_live := `async joinLive($1) {
					var result = await (async () => {
						$2;
					})();
					var data = result.data;
					// console.log("before JoinLive", data);
					WXU.emit(WXU.Events.JoinLive, data);
					return result;
				}async`
						js_script = jsJoinLiveReg.ReplaceAllString(js_script, js_join_live)
					}
					{

						api_methods := "{}"
						if m := jsExportBlockReg.FindStringSubmatch(js_script); len(m) >= 2 {
							items := strings.Split(m[1], ",")
							locals := make([]string, 0, len(items))
							for _, it := range items {
								p := strings.TrimSpace(it)
								if p == "" {
									continue
								}
								idx := strings.Index(p, " as ")
								local := p
								if idx != -1 {
									local = strings.TrimSpace(p[:idx])
								}
								if local != "" && local != " " {
									locals = append(locals, local)
								}
							}
							if len(locals) > 0 {
								api_methods = "{" + strings.Join(locals, ",") + "}"
							}
						}
						api_methods_escaped := strings.ReplaceAll(api_methods, "$", "$$")
						js_wxapi := ";WXU.emit(WXU.Events.APILoaded," + api_methods_escaped + ");export{"
						js_script = jsExportReg.ReplaceAllString(js_script, js_wxapi)
					}
					ctx.SetResponseBody(js_script)
					return
				}
				if strings.Contains(pathname, "connect.publish") || strings.Contains(pathname, "applyMic.publish") {
					flow_list_variable_name := "yt"
					if m := jsFlowTabReg.FindStringSubmatch(js_script); len(m) >= 2 {
						flow_list_variable_name = m[1]
					}
					{

						js_go_next_feed := fmt.Sprintf(`goToNextFlowFeed:async function(v){
						await $1(v);
						// console.log('goToNextFlowFeed', %[1]s);
						if (!%[1]s || !%[1]s.value.feeds) {
							return;
						}
						var feed = %[1]s.value.feeds[%[1]s.value.currentFeedIndex];
						// console.log("before GotoNextFeed", %[1]s, feed);
						WXU.emit(WXU.Events.GotoNextFeed, feed);
					}`, flow_list_variable_name)
						js_script = jsGoToNextFlowReg.ReplaceAllString(js_script, js_go_next_feed)
					}
					{
						js_go_prev_feed := fmt.Sprintf(`goToPrevFlowFeed:async function(v){
						await $1(v);
						// console.log('goToPrevFlowFeed', %[1]s);
						if (!%[1]s || !%[1]s.value.feeds) {
							return;
						}
						var feed = %[1]s.value.feeds[%[1]s.value.currentFeedIndex];
						// console.log("before GotoPrevFeed", %[1]s, feed);
						WXU.emit(WXU.Events.GotoPrevFeed, feed);
					}`, flow_list_variable_name)
						js_script = jsGoToPrevFlowReg.ReplaceAllString(js_script, js_go_prev_feed)
					}
					{
						js_wxutil := ";WXU.emit(WXU.Events.UtilsLoaded,{decodeBase64ToUint64String:decodeBase64ToUint64String,createAdapterFromGlobalMapper:createAdapterFromGlobalMapper,finderJoinLiveMapper:finderJoinLiveMapper});export{"
						js_script = jsExportReg.ReplaceAllString(js_script, js_wxutil)
					}
					{
						local_feed_list_variable_name := "vn"
						if m := jsLocalFlowTabReg.FindStringSubmatch(js_script); len(m) >= 2 {
							local_feed_list_variable_name = m[1]
						}
						js_load_local := fmt.Sprintf(`loadLocalPlaylist:async function(...args){
						await $1(...args);
						console.log('loadLocalPlaylist', %[1]s);
						if (!%[1]s || !%[1]s.value || !%[1]s.value.feeds) {
							return;
						}
						var feed = %[1]s.value.feeds[%[1]s.value.currentFeedIndex];
						WXU.emit(WXU.Events.HomeFeedChanged, feed);
					}`, local_feed_list_variable_name)
						js_script = jsLoadLocalPlaylistReg.ReplaceAllString(js_script, js_load_local)
					}
					ctx.SetResponseBody(js_script)
					return
				}
				ctx.SetResponseBody(js_script)
			}
		},
	}
	return []*proxy.Plugin{plugin1, plugin2}
}

// CreateYuanbaoTencentPlugin 拦截 yuanbao.tencent.com/api 的请求与响应，提取请求头中的 Cookie 和响应头中的 Set-Cookie 并通过回调传出。
// 回调中的 cookieStr 是 Cookie / Set-Cookie 值。
func CreateYuanbaoTencentPlugin(onCookieExtracted func(cookieStr string)) *proxy.Plugin {
	isAPIPath := func(ctx proxy.Context) bool {
		return ctx.Req().URL.Hostname() == "yuanbao.tencent.com" && strings.HasPrefix(ctx.Req().URL.Path, "/api")
	}
	return &proxy.Plugin{
		Match: "yuanbao.tencent.com",
		OnRequest: func(ctx proxy.Context) {
			if !isAPIPath(ctx) {
				return
			}
			cookie := ctx.Req().Header.Get("Cookie")
			if cookie != "" && onCookieExtracted != nil {
				onCookieExtracted(cookie)
			}
		},
		OnResponse: func(ctx proxy.Context) {
			if !isAPIPath(ctx) {
				return
			}
			cookies := ctx.Res().Header.Values("Set-Cookie")
			if len(cookies) > 0 {
				cookieValue := strings.Join(cookies, "; ")
				if onCookieExtracted != nil {
					onCookieExtracted(cookieValue)
				}
			}
		},
	}
}

func CreateSimpleChannelInterceptorPlugin(interceptor *Interceptor, files *ChannelInjectedFiles) *proxy.Plugin {
	version := interceptor.Version
	assetBaseURL := ChannelAssetsBaseURLFromConfig(interceptor.Settings)
	v := "?t=" + version
	return &proxy.Plugin{
		Match: "qq.com",
		OnRequest: func(ctx proxy.Context) {
			if mockChannelStaticAsset(ctx, ctx.Req().URL.Path, files) {
				return
			}
		},
		OnResponse: func(ctx proxy.Context) {
			resp_content_type := strings.ToLower(ctx.GetResponseHeader("Content-Type"))
			hostname := ctx.Req().URL.Hostname()
			pathname := ctx.Req().URL.Path
			// fmt.Println("response", hostname, pathname, resp_content_type, ctx.Res().StatusCode)
			if hostname == "channels.weixin.qq.com" && strings.Contains(resp_content_type, "text/html") {
				resp_body, err := ctx.GetResponseBody()
				if err != nil {
					return
				}
				html := string(resp_body)
				html = scriptSrcReg.ReplaceAllString(html, `src="$1.js`+v+`"`)
				html = scriptHrefReg.ReplaceAllString(html, `href="$1.js`+v+`"`)
				var injected strings.Builder
				if pathname == "/web/pages/feed" || pathname == "/web/pages/home" {
					/** 核心逻辑 */
					AppendScriptSrcs(&injected, "", ChannelSrcAssetURL(assetBaseURL, "home.js"))
				}
				html = strings.Replace(html, "<head>", "<head>\n"+injected.String(), 1)
				ctx.SetResponseBody(html)
				return
			}
		},
	}
}
