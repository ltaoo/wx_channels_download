package wxchannels

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/fatih/color"

	"wx_channel/frontend"
	"wx_channel/internal/interceptor"
	"wx_channel/internal/interceptor/proxy"
	"wx_channel/pkg/util"
)

var (
	// HTML processing related regex patterns
	script_src_reg  = regexp.MustCompile(`src="([^"]{1,})\.js"`)
	script_href_reg = regexp.MustCompile(`href="([^"]{1,})\.js"`)

	// JavaScript processing related regex patterns
	js_dep_reg          = regexp.MustCompile(`"js/([^"]{1,})\.js"`)
	js_from_reg         = regexp.MustCompile(`from {0,1}"([^"]{1,})\.js"`)
	js_lazy_import_reg  = regexp.MustCompile(`import\("([^"]{1,})\.js"\)`)
	js_import_reg       = regexp.MustCompile(`import {0,1}"([^"]{1,})\.js"`)
	js_export_reg       = regexp.MustCompile(`exports?\s*\{`)
	js_export_block_reg = regexp.MustCompile(`exports?\s*\{([^}]*)\}`)

	// Regex patterns for specific paths
	js_source_buffer_reg                      = regexp.MustCompile(`this.sourceBuffer.appendBuffer\(([a-zA-Z]{1,})\),`)
	js_init_reg                               = regexp.MustCompile(`async finderInit\(\)\{(.*?)\}async`)
	js_feed_profile_reg                       = regexp.MustCompile(`async finderGetCommentDetail\((\w+)\)\{(.*?)\}async`)
	js_comment_list_reg                       = regexp.MustCompile(`async finderGetCommentList\((\w+)\)\{(.*?)\}async`)
	js_pc_flow_reg                            = regexp.MustCompile(`async finderPcFlow\((\w+)\)\{(.*?)\}async`)
	js_live_info_reg                          = regexp.MustCompile(`async finderGetLiveInfo\((\w+)\)\{(.*?)\}async`)
	js_live_feed_list_reg                     = regexp.MustCompile(`async finderLiveUserPage\((\w+)\)\{(.*?)\}async`)
	js_join_live_reg                          = regexp.MustCompile(`async joinLive\((\w+)\)\{(.*?)\}async`)
	js_recommend_feeds_reg                    = regexp.MustCompile(`async finderGetRecommend\((\w+)\)\{(.*?)\}async`)
	js_user_feeds_reg                         = regexp.MustCompile(`async finderUserPage\((\w+)\)\{(.*?)\}async`)
	js_finder_pc_search_reg                   = regexp.MustCompile(`async finderPCSearch\((\w+)\)\{(.*?)\}async`)
	js_finder_search_reg                      = regexp.MustCompile(`async finderSearch\((\w+)\)\{(.*?)\}async`)
	js_finder_get_follow_list_reg             = regexp.MustCompile(`async finderGetFollowList\((\w+)\)\{(.*?)\}async`)
	js_finder_get_play_history_reg            = regexp.MustCompile(`async finderGetPlayHistory\((\w+)\)\{(.*?)\}async`)
	js_finder_get_interactioned_feed_list_reg = regexp.MustCompile(`async finderGetInteractionedFeedList\((\w+)\)\{(.*?)\}\}const`)
	js_finder_get_feed_h5_url                 = regexp.MustCompile(`async finderGetFeedH5Url\((\w+)\)\{(.*?)\}\}const`)
	js_go_to_prev_flow_reg                    = regexp.MustCompile(`goToPrevFlowFeed:([a-zA-Z_$]{1,})`)
	js_go_to_next_flow_reg                    = regexp.MustCompile(`goToNextFlowFeed:([a-zA-Z_$]{1,})`)
	js_flow_tab_reg                           = regexp.MustCompile(`flowTab:([a-zA-Z_$]{1,})`)
	js_local_flow_tab_reg                     = regexp.MustCompile(`localFlowTab:([a-zA-Z]{1,})`)
	js_load_local_playlist_reg                = regexp.MustCompile(`loadLocalPlaylist:([a-zA-Z]{1,})`)
)

func CreateInterceptorPlugins(cfg *ChannelsConfig) []*proxy.Plugin {
	if cfg == nil {
		cfg = &ChannelsConfig{}
	}
	logger := cfg.Logger
	version := cfg.Version
	asset_version := version
	if asset_version == "" {
		asset_version = "static"
	}
	version_query := url.Values{"v": []string{asset_version}}
	variables := cfg.FrontendVariables
	if variables == nil {
		variables = map[string]any{}
	}
	asset_base_url := "/__assets"
	url_build := frontend.NewURLBuild(asset_base_url, nil)
	v := "?t=" + version
	global_script_asset_path := ""
	if cfg.GlobalScriptPath != "" {
		global_script_asset_path = frontend.UserGlobalScriptAssetPath(cfg.GlobalScriptPath)
		logger.Info().
			Str("file", "pkg/scraper/wxchannels/plugin.go").
			Str("path", cfg.GlobalScriptPath).
			Str("asset_path", global_script_asset_path).
			Msg("wxchannels interceptor initialized with global script")
	}
	plugins := make([]*proxy.Plugin, 0, 5)
	plugin_1 := &proxy.Plugin{
		Match: "channels.weixin.qq.com",
		OnRequest: func(ctx proxy.Context) {
			pathname := ctx.Req().URL.Path
			if interceptor.MockFrontendStaticAsset(ctx, pathname, interceptor.FrontendStaticAssetMockOptions{
				PlatformPrefix: StaticAssetsPath + "/",
				PlatformFS:     Assets.InjectFS,
				UserScriptPath: cfg.GlobalScriptPath,
				Logger:         logger,
			}) {
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
					if resp_2, err_2 := client.Do(req); err_2 == nil {
						defer resp_2.Body.Close()
						body_2, _ := io.ReadAll(resp_2.Body)
						ct := resp_2.Header.Get("Content-Type")
						lct := strings.ToLower(ct)
						if ct == "" || strings.Contains(lct, "text/html") {
							ct = "text/html; charset=utf-8"
						}
						ctx.SetStatusCode(200)
						ctx.Res().Header.Del("Content-Encoding")
						ctx.Res().Header.Del("Content-Length")
						ctx.SetResponseHeader("Content-Type", ct)
						ctx.SetResponseBody(string(body_2))
						resp_content_type = strings.ToLower(ct)
					}
				}
			}
			if hostname == "channels.weixin.qq.com" && strings.Contains(resp_content_type, "text/html") {
				logger.Info().
					Str("file", "pkg/scraper/wxchannels/plugin.go").
					Str("hostname", hostname).
					Str("pathname", pathname).
					Msg("match channels.weixin.qq.com")
				resp_body, err := ctx.GetResponseBody()
				if err != nil {
					fmt.Println("[error]get response body failed,", err)
					return
				}
				html := string(resp_body)
				html = script_src_reg.ReplaceAllString(html, `src="$1.js`+v+`"`)
				html = script_href_reg.ReplaceAllString(html, `href="$1.js`+v+`"`)

				var injected strings.Builder
				crossorigin_attr := ` crossorigin="anonymous"`
				if cfg.DebugShowError {
					/** Global error capture and show dialog */
					frontend.AppendScripts(&injected, crossorigin_attr, url_build("/inject/error.js", version_query))
				}
				frontend.AppendStylesheets(&injected, "", url_build("/inject/components.css", version_query))
				frontend.AppendStylesheets(&injected, "", url_build("/public/timeless/0.30.0/timeless.weui.css"))
				frontend.AppendScripts(
					&injected,
					crossorigin_attr,
					url_build("/public/timeless/0.30.0/timeless.umd.min.js"),
					url_build("/public/timeless/0.30.0/timeless.utils.umd.min.js"),
					url_build("/public/timeless/0.30.0/timeless.weui.umd.min.js"),
					url_build("/public/timeless/0.30.0/timeless.dom.umd.min.js"),
					url_build("/public/timeless/0.30.0/timeless.web.umd.min.js"),
				)
				frontend_config := make(map[string]any, len(variables)+2)
				for key, value := range variables {
					frontend_config[key] = value
				}
				frontend_config["version"] = version
				frontend_config["assets_base_url"] = asset_base_url
				logger.Info().
					Str("file", "pkg/scraper/wxchannels/plugin.go").
					Interface("variables", variables).
					Interface("frontend_config", frontend_config).
					Msg("wxchannels frontend config built")
				frontend_config_byte, _ := json.Marshal(frontend_config)
				frontend.AppendInlineScript(
					&injected,
					"",
					fmt.Sprintf(`window.__d_config = %s;`, frontend_config_byte),
				)
				frontend.AppendScripts(
					&injected,
					crossorigin_attr,
					url_build("/public/mitt.umd.js"),
					url_build("/inject/eventbus.js", version_query),
					url_build("/inject/env.js", version_query),
					url_build("/inject/utils.js", version_query),
					url_build("/inject/components.js", version_query),
					url_build("/inject/virtual-list-view.js", version_query),
				)
				if cfg.PagespyEnabled {
					frontend.AppendScripts(
						&injected,
						crossorigin_attr,
						url_build("/public/pagespy.min.js"),
						url_build("/inject/pagespy.js", version_query),
					)
				}
				frontend.AppendScripts(
					&injected,
					crossorigin_attr,
					url_build("/inject/download/model.js", version_query),
					url_build("/inject/download/view.js", version_query),
					url_build("/inject/download/panel.js", version_query),
				)
				frontend.AppendScripts(
					&injected,
					crossorigin_attr,
					AssetURL(asset_base_url, "/inject/channels.events.js", version_query),
					AssetURL(asset_base_url, "/inject/channels.env.js", version_query),
					AssetURL(asset_base_url, "/inject/channels.utils.js", version_query),
					AssetURL(asset_base_url, "/inject/channels.ws.js", version_query),
				)
				if global_script_asset_path != "" {
					frontend.AppendScripts(&injected, crossorigin_attr, global_script_asset_path)
					logger.Info().
						Str("file", "pkg/scraper/wxchannels/plugin.go").
						Str("path", cfg.GlobalScriptPath).
						Str("asset_path", global_script_asset_path).
						Msg("after append global script")
				}
				if cfg.InjectContentScript != "" {
					frontend.AppendInlineScript(&injected, "", cfg.InjectContentScript)
				}
				if pathname == "/web/pages/home" {
					frontend.AppendScripts(&injected, crossorigin_attr, AssetURL(asset_base_url, "/inject/channels.home.js", version_query))
				}
				if pathname == "/web/pages/feed" {
					frontend.AppendScripts(&injected, crossorigin_attr, AssetURL(asset_base_url, "/inject/channels.feed.js", version_query))
				}
				if pathname == "/web/pages/live" {
					frontend.AppendScripts(&injected, crossorigin_attr, AssetURL(asset_base_url, "/inject/channels.live.js", version_query))
				}
				if pathname == "/web/pages/profile" {
					frontend.AppendScripts(&injected, crossorigin_attr, AssetURL(asset_base_url, "/inject/channels.profile.js", version_query))
				}
				html = strings.Replace(html, "<head>", "<head>\n"+injected.String(), 1)
				ctx.SetResponseBody(html)
				return
			}
		},
	}
	plugin_2 := &proxy.Plugin{
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
				js_script = js_from_reg.ReplaceAllString(js_script, `from"$1.js`+v+`"`)
				js_script = js_dep_reg.ReplaceAllString(js_script, `"js/$1.js`+v+`"`)
				js_script = js_lazy_import_reg.ReplaceAllString(js_script, `import("$1.js`+v+`")`)
				js_script = js_import_reg.ReplaceAllString(js_script, `import"$1.js`+v+`"`)

				if strings.Contains(pathname, "virtual_svg-icons-register.publish") {
					{
						js_init := `async finderInit() {
					var result = await (async () => {
						$1;
					})();
					var data = result.data;
					// console.log("before Init", data);
					WXU.emit("channels:Init", data);
					return result;
				}async`
						js_script = js_init_reg.ReplaceAllString(js_script, js_init)
					}
					{
						js_pc_flow := `async finderPcFlow($1) {
					var result = await (async () => {
						$2;
					})();
					var feeds = result.data.object;
					// console.log("before PCFlowLoaded", result.data);
					WXU.emit("channels:PCFlowLoaded", feeds);
					return result;
				}async`
						js_script = js_pc_flow_reg.ReplaceAllString(js_script, js_pc_flow)
					}
					{
						js_recommend_feeds := `async finderGetRecommend($1) {
					var result = await (async () => {
						$2;
					})();
					var feeds = result.data.object;
					// console.log("before RecommendFeedsLoaded", result.data);
					WXU.emit("channels:RecommendFeedsLoaded", feeds);
					return result;
				}async`
						js_script = js_recommend_feeds_reg.ReplaceAllString(js_script, js_recommend_feeds)
					}
					{
						js_feed_profile := `async finderGetCommentDetail($1) {
					var result = await (async () => {
						$2;
					})();
					var feed = result.data.object;
					// console.log("before FeedProfileLoaded", result.data);
					WXU.emit("channels:OnFeedProfileLoaded", feed);
					return result;
				}async`
						js_script = js_feed_profile_reg.ReplaceAllString(js_script, js_feed_profile)
					}
					{
						js_comment_list := `async finderGetCommentList($1) {
					var result = await (async () => {
						$2;
					})();
					// console.log("before CommentListLoaded", result.data, $1);
					WXU.emit("channels:FeedCommentListLoaded", result.data);
					return result;
				}async`
						js_script = js_comment_list_reg.ReplaceAllString(js_script, js_comment_list)
					}
					{
						js_finder_pc_search := `async finderPCSearch($1) {
					var result = await (async () => {
						$2;
					})();
					// console.log("before finderPCSearch", result, $1);
					return result;
				}async`
						js_script = js_finder_pc_search_reg.ReplaceAllString(js_script, js_finder_pc_search)
					}
					{
						js_finder_search := `async finderSearch($1) {
					var result = await (async () => {
						$2;
					})();
					// console.log("before finderSearch", result, $1);
					return result;
				}async`
						js_script = js_finder_search_reg.ReplaceAllString(js_script, js_finder_search)
					}

					{
						js_finder_get_follow_list := `async finderGetFollowList($1) {
						console.log("finderGetFollowList payload", $1);
						var result = await (async () => {
							$2;
						})();
						console.log("finderGetFollowList result", result);
						return result;
					}async`
						js_script = js_finder_get_follow_list_reg.ReplaceAllString(js_script, js_finder_get_follow_list)
					}
					{
						js_finder_get_play_history := `async finderGetPlayHistory($1) {
						console.log("finderGetPlayHistory payload", $1);
						var result = await (async () => {
							$2;
						})();
						console.log("finderGetPlayHistory result", $1, result);
						return result;
					}async`
						js_script = js_finder_get_play_history_reg.ReplaceAllString(js_script, js_finder_get_play_history)
					}
					{
						js_finder_interactioned := `async finderGetInteractionedFeedList($1) {
						var result = await (async () => {
							$2;
						})();
						var feeds = result.data.object;
						// console.log("before finderGetInteractionedFeedList", result, $1);
						WXU.emit("channels:InteractionedFeedsLoaded", feeds);
						return result;
					}}const`
						js_script = js_finder_get_interactioned_feed_list_reg.ReplaceAllString(js_script, js_finder_interactioned)
					}
					{
						js_finder_feed_h5_url := `async finderGetFeedH5Url($1) {
						var result = await (async () => {
							$2;
						})();
						var data = result.data.object;
						// console.log("before finderGetFeedH5Url", result, $1);
						WXU.emit("channels:GetFeedH5Url", data);
						return result;
					}}const`
						js_script = js_finder_get_feed_h5_url.ReplaceAllString(js_script, js_finder_feed_h5_url)
					}
					{
						js_user_feed := `async finderUserPage($1) {
					var result = await (async () => {
						$2;
					})();
					var feeds = result.data.object;
					// console.log("before UserFeedsLoaded", result.data, $1);
					WXU.emit("channels:UserFeedsLoaded", feeds);
					return result;
				}async`
						js_script = js_user_feeds_reg.ReplaceAllString(js_script, js_user_feed)
					}
					{
						js_live_feed_list := `async finderLiveUserPage($1) {
						var result = await (async () => {
							$2;
						})();
						var feeds = result.data.object;
						// console.log("before LiveUserFeedsLoaded", result.data, $1);
						WXU.emit("channels:LiveUserFeedsLoaded", feeds);
						return result;
					}async`
						js_script = js_live_feed_list_reg.ReplaceAllString(js_script, js_live_feed_list)
					}
					{
						js_live_profile := `async finderGetLiveInfo($1) {
					var result = await (async () => {
						$2;
					})();
					var live = result.data;
					// console.log("before LiveProfileLoaded", result.data);
					WXU.emit("channels:OnLiveProfileLoaded", live);
					return result;
				}async`
						js_script = js_live_info_reg.ReplaceAllString(js_script, js_live_profile)
					}
					{
						js_join_live := `async joinLive($1) {
					var result = await (async () => {
						$2;
					})();
					var data = result.data;
					// console.log("before JoinLive", data);
					WXU.emit("channels:JoinLive", data);
					return result;
				}async`
						js_script = js_join_live_reg.ReplaceAllString(js_script, js_join_live)
					}
					{

						api_methods := "{}"
						if m := js_export_block_reg.FindStringSubmatch(js_script); len(m) >= 2 {
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
						js_wxapi := `;WXU.emit("channels:APILoaded",` + api_methods_escaped + `);export{`
						js_script = js_export_reg.ReplaceAllString(js_script, js_wxapi)
					}
					ctx.SetResponseBody(js_script)
					return
				}
				if strings.Contains(pathname, "connect.publish") || strings.Contains(pathname, "applyMic.publish") {
					flow_list_variable_name := "yt"
					if m := js_flow_tab_reg.FindStringSubmatch(js_script); len(m) >= 2 {
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
						WXU.emit("channels:GotoNextFeed", feed);
					}`, flow_list_variable_name)
						js_script = js_go_to_next_flow_reg.ReplaceAllString(js_script, js_go_next_feed)
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
						WXU.emit("channels:GotoPrevFeed", feed);
					}`, flow_list_variable_name)
						js_script = js_go_to_prev_flow_reg.ReplaceAllString(js_script, js_go_prev_feed)
					}
					{
						js_wxutil := `;WXU.emit("channels:UtilsLoaded",{decodeBase64ToUint64String:decodeBase64ToUint64String,createAdapterFromGlobalMapper:createAdapterFromGlobalMapper,finderJoinLiveMapper:finderJoinLiveMapper});export{`
						js_script = js_export_reg.ReplaceAllString(js_script, js_wxutil)
					}
					{
						local_feed_list_variable_name := "vn"
						if m := js_local_flow_tab_reg.FindStringSubmatch(js_script); len(m) >= 2 {
							local_feed_list_variable_name = m[1]
						}
						js_load_local := fmt.Sprintf(`loadLocalPlaylist:async function(...args){
						await $1(...args);
						console.log('loadLocalPlaylist', %[1]s);
						if (!%[1]s || !%[1]s.value || !%[1]s.value.feeds) {
							return;
						}
						var feed = %[1]s.value.feeds[%[1]s.value.currentFeedIndex];
						WXU.emit("channels:HomeFeedChanged", feed);
					}`, local_feed_list_variable_name)
						js_script = js_load_local_playlist_reg.ReplaceAllString(js_script, js_load_local)
					}
					ctx.SetResponseBody(js_script)
					return
				}
				ctx.SetResponseBody(js_script)
			}
		},
	}
	plugins = append(plugins, plugin_1, plugin_2)
	return plugins
}
