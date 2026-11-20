package wxchannels

import (
	"fmt"
	"strings"
)

func rewrite_channels_api_js(js_script string, expose_exports bool) string {
	js_script = js_preload_feed_reg.ReplaceAllString(js_script, ` (async()=>{const _r = await $1; typeof WXU !== "undefined" && WXU.emit("channels:PreloadFeeds", _r.data.object); return _r;})();(`)
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
						typeof WXU !== "undefined" && WXU.emit("channels:GotoNextFeed", feed);
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
						typeof WXU !== "undefined" && WXU.emit("channels:GotoPrevFeed", feed);
					}`, flow_list_variable_name)
		js_script = js_go_to_prev_flow_reg.ReplaceAllString(js_script, js_go_prev_feed)
	}
	{
		js_init := `async finderInit() {
					var result = await (async () => {
						$1;
					})();
					var data = result.data;
					console.log("finderInit result", result);
					typeof WXU !== "undefined" && WXU.emit("channels:Init", data);
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
					console.log("finderPcFlow result", result, $1);
					typeof WXU !== "undefined" && WXU.emit("channels:PCFlowLoaded", feeds);
					return result;
				}async`
		js_script = js_pc_flow_reg.ReplaceAllString(js_script, js_pc_flow)
	}
	{
		// This may be the last method in the class. Preserve the matched
		// boundary instead of consuming the next class's async method.
		js_get_recommend_tabs_from_service := `async getRecommendTabsFromService() {
					var result = await (async () => {
						$1;
					})();
					var response = result.resp || result;
					var feeds = response.data ? response.data.object : [];
					console.log("getRecommendTabsFromService", result);
					typeof WXU !== "undefined" && WXU.emit("channels:RecommendFeedsLoaded", feeds);
					return result;
					}${2}`
		js_script = js_get_recommend_tabs_from_service_reg.ReplaceAllString(js_script, js_get_recommend_tabs_from_service)
	}
	{
		js_recommend_feeds := `async finderGetRecommend($1) {
					var result = await (async () => {
						$2;
					})();
					var feeds = result.data.object;
					console.log("finderGetRecommend result", result, $1);
					typeof WXU !== "undefined" && WXU.emit("channels:RecommendFeedsLoaded", feeds);
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
					console.log("finderGetCommentDetail", result.data, $1);
					typeof WXU !== "undefined" && WXU.emit("channels:OnFeedProfileLoaded", feed);
					return result;
				}async`
		js_script = js_feed_profile_reg.ReplaceAllString(js_script, js_feed_profile)
	}
	{
		js_comment_list := `async finderGetCommentList($1) {
					var result = await (async () => {
						$2;
					})();
					console.log("finderGetCommentList result", result, $1);
					typeof WXU !== "undefined" && WXU.emit("channels:FeedCommentListLoaded", result.data);
					return result;
				}async`
		js_script = js_comment_list_reg.ReplaceAllString(js_script, js_comment_list)
	}
	{
		js_finder_pc_search := `async finderPCSearch($1) {
					var result = await (async () => {
						$2;
					})();
					console.log("finderPCSearch result", result, $1);
					return result;
				}async`
		js_script = js_finder_pc_search_reg.ReplaceAllString(js_script, js_finder_pc_search)
	}
	{
		js_finder_search := `async finderSearch($1) {
					var result = await (async () => {
						$2;
					})();
					console.log("finderSearch result", result, $1);
					return result;
				}async`
		js_script = js_finder_search_reg.ReplaceAllString(js_script, js_finder_search)
	}

	{
		js_finder_get_follow_list := `async finderGetFollowList($1) {
						var result = await (async () => {
							$2;
						})();
						console.log("finderGetFollowList result", result, $1);
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
						console.log("finderGetPlayHistory result", result, $1);
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
						console.log("finderGetInteractionedFeedList result", result, $1);
						typeof WXU !== "undefined" && WXU.emit("channels:InteractionedFeedsLoaded", feeds);
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
						console.log("finderGetFeedH5Url result", result, $1);
						typeof WXU !== "undefined" && WXU.emit("channels:GetFeedH5Url", data);
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
					console.log("finderUserPage", result, $1);
					typeof WXU !== "undefined" && WXU.emit("channels:UserFeedsLoaded", feeds);
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
						console.log("finderLiveUserPage", result, $1);
						typeof WXU !== "undefined" && WXU.emit("channels:LiveUserFeedsLoaded", feeds);
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
					console.log("finderGetLiveInfo result", result, $1);
					typeof WXU !== "undefined" && WXU.emit("channels:OnLiveProfileLoaded", live);
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
					console.log("JoinLive result", result, $1);
					typeof WXU !== "undefined" && WXU.emit("channels:JoinLive", data);
					return result;
				}async`
		js_script = js_join_live_reg.ReplaceAllString(js_script, js_join_live)
	}
	if expose_exports {
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
		js_wxapi := `;typeof WXU !== "undefined" && WXU.emit("channels:APILoaded",` + api_methods_escaped + `);export{`
		js_script = js_export_reg.ReplaceAllString(js_script, js_wxapi)
	}
	return js_script
}
