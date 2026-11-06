package wxchannels

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/evanw/esbuild/pkg/api"
	"github.com/ltaoo/echo"
)

func TestInterceptorFeedProfile(t *testing.T) {
	fixture, err := os.ReadFile("testdata/feed_profile_service.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, last_method := range []bool{true, false} {
		name := "recommend_tabs_last_method"
		if !last_method {
			name = "recommend_tabs_before_async_method"
		}
		t.Run(name, func(t *testing.T) {
			source := string(fixture)
			if !last_method {
				source = strings.Replace(source, "}}const API_CONFIG", "}async next(){return 42}}const API_CONFIG", 1)
			}
			ctx := &echo.Context{
				Req: httptest.NewRequest(http.MethodGet, "https://res.wx.qq.com/t/wx_fed/finder/web/web-finder/res/js/virtual_svg-icons-register.publishCjCx5HhZH5.js", nil),
				Res: &http.Response{
					Header: http.Header{"Content-Type": []string{"application/javascript"}},
					Body:   io.NopCloser(strings.NewReader(source)),
				},
			}
			for _, plugin := range NewInterceptorPlugins(InterceptorConfig{}, nil) {
				if plugin.Match == "res.wx.qq.com" {
					plugin.OnResponse(ctx)
				}
			}
			injected, err := ctx.GetResponseBody()
			if err != nil {
				t.Fatal(err)
			}
			compiled := api.Transform(injected, api.TransformOptions{Format: api.FormatIIFE, GlobalName: "fixture", Target: api.ES2020})
			if len(compiled.Errors) != 0 {
				t.Fatalf("injected script is invalid: %+v", compiled.Errors)
			}
			runtime := goja.New()
			_, err = runtime.RunString(`
				var events = [];
				var WXU = {emit: function(name, data) { events.push([name, data]); }};
				var console = {log: function() {}};
				function generateFinderBaseRequestRequestId() { return "request-id"; }
				function getUrlParam() { return "native-buffer"; }
				function isResponseFailed() { return false; }
				var storage = {setLocal: function() {}};
				var StorageKey = {recTagItem: "tabs"};
			` + string(compiled.Code) + `
				var failure, completed = false;
				(async function() {
					var service = fixture.service;
					var feed = {id: "feed-id"}, result = {data: {object: feed}};
					service.finderBasereq = {token: "token"};
					service.post = function(request) {
						if (this !== service || request.data.objectId !== "feed-id" || request.data.finderBasereq.nativeInfo.nativeBuffer !== "native-buffer") throw Error("request changed");
						return Promise.resolve(result);
					};
					if (await service.finderGetCommentDetail({objectId: "feed-id"}) !== result) throw Error("result changed");
					var profile_events = events.filter(function(event) { return event[0] === "channels:OnFeedProfileLoaded"; });
					if (profile_events.length !== 1 || profile_events[0][1] !== feed) throw Error("missing or duplicate feed profile event");
					var expected_error = Error("request failed"), caught;
					service.post = function() { return Promise.reject(expected_error); };
					try { await service.finderGetCommentDetail({}); } catch (err) { caught = err; }
					if (caught !== expected_error || events.filter(function(event) { return event[0] === "channels:OnFeedProfileLoaded"; }).length !== 1) throw Error("rejection changed or emitted a profile event");
					service.post = function() { return Promise.resolve(result); };
					var tabs = await service.getRecommendTabsFromService();
					var recommend_events = events.filter(function(event) { return event[0] === "channels:RecommendFeedsLoaded"; });
					if (tabs.resp !== result || recommend_events.length !== 1 || recommend_events[0][1] !== feed) throw Error("recommendation response changed");
					completed = true;
				})().catch(function(err) { failure = String(err); });
			`)
			if err != nil {
				t.Fatal(err)
			}
			if failure := runtime.Get("failure"); !goja.IsUndefined(failure) {
				t.Fatal(failure)
			}
			if !runtime.Get("completed").ToBoolean() {
				t.Fatal("feed profile checks did not complete")
			}
		})
	}
}
