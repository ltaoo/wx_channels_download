package wxchannels

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/evanw/esbuild/pkg/api"
	"github.com/ltaoo/echo"
)

func TestInterceptorServiceEvents(t *testing.T) {
	// Include nested syntax and text resembling the old method delimiters.
	source := "class Service{async finderGetInteractionedFeedList(payload){" +
		"const text='}async finderGetFeedH5Url(t){';const pattern=/[{}]/;" +
		"const label=`${payload.flag}:}}const`;/* }async */" +
		"if(payload.fail){throw this.error;}return {data:{object:[this.marker,payload.flag]}};}" +
		"async finderGetFeedH5Url(payload){return {data:{object:this.marker+payload.id}};}}" +
		"const service=new Service;service.marker='ok';service.error=new Error('failed');" +
		"export{service,service as alias};"
	runtime := goja.New()
	_, err := runtime.RunString(`
		var events = [];
		var WXU = {
			emit(name, data) {
				if (name === "channels:APILoaded") {
					this.services = data;
					this.api_loaded(data);
				} else {
					events.push([name, data]);
				}
			},
			onAPILoaded(handler) {this.api_loaded = handler;}
		};
	`)
	if err != nil {
		t.Fatal(err)
	}
	// Load the actual API registration code without requiring a DOM.
	utils, err := InjectAssets().Open("channels.utils.js")
	if err != nil {
		t.Fatal(err)
	}
	defer utils.Close()
	utils_bytes, err := io.ReadAll(utils)
	if err != nil {
		t.Fatal(err)
	}
	utils_source := string(utils_bytes)
	start := strings.Index(utils_source, "  var WXAPI = {};")
	end := strings.Index(utils_source, "  WXU.onUtilsLoaded")
	if start < 0 || end <= start {
		t.Fatal("API registration code not found")
	}
	if _, err := runtime.RunString(utils_source[start:end]); err != nil {
		t.Fatal(err)
	}
	load_services := func(source string) {
		t.Helper()
		rewritten := rewrite_interceptor_response(t, 1, "https://res.wx.qq.com/virtual_svg-icons-register.publish.test.js", "application/javascript", source)
		if !strings.HasPrefix(rewritten, strings.Split(source, "export{")[0]) {
			t.Fatal("interceptor changed original method bodies")
		}
		result := api.Transform(rewritten, api.TransformOptions{Loader: api.LoaderJS, Format: api.FormatIIFE})
		if len(result.Errors) != 0 {
			t.Fatalf("invalid injected JavaScript: %+v", result.Errors)
		}
		if _, err := runtime.RunString(string(result.Code)); err != nil {
			t.Fatal(err)
		}
	}
	load_services(source)
	// The same service can be re-exported by another intercepted module.
	load_services(`const service = WXU.services.service;export{service};`)
	value, err := runtime.RunString(`
		(async () => {
			const service = WXU.services.service;
			const first = await service.finderGetInteractionedFeedList({flag:7});
			const second = await WXAPI4.finderGetInteractionedFeedList({flag:8});
			const h5 = await WXAPI4.finderGetFeedH5Url({id:9});
			if (first.data.object.join(":") !== "ok:7" || second.data.object.join(":") !== "ok:8" || h5.data.object !== "ok9") {
				throw new Error("response or receiver changed");
			}
			try {
				await WXAPI4.finderGetInteractionedFeedList({fail:true});
				throw new Error("request should reject");
			} catch (error) {
				if (error !== service.error) throw error;
			}
			if (JSON.stringify(events) !== JSON.stringify([
				["channels:InteractionedFeedsLoaded", first.data.object],
				["channels:InteractionedFeedsLoaded", second.data.object],
				["channels:GetFeedH5Url", h5.data.object]
			])) {
				throw new Error("missing, duplicate, or incorrect event");
			}
		})()
	`)
	if err != nil {
		t.Fatal(err)
	}
	promise, ok := value.Export().(*goja.Promise)
	if !ok || promise.State() != goja.PromiseStateFulfilled {
		t.Fatalf("service event checks failed: %v", value)
	}
}

func TestInterceptorPreservesRecommendTabsAtClassEnd(t *testing.T) {
	tabs_and_config := `class Tabs{async getRecommendTabsFromService(){return {data:{object:[{label:"tabs"}]},nested:()=>({value:1})};}}const API_CONFIG={version:2};`
	source := tabs_and_config + `class Service{constructor(){this.config=API_CONFIG;}async finderGetCommentDetail(payload){return {data:{object:{id:payload.id}}};}async joinLive(payload){return {data:{object:payload}};}async untouched(){return this.config;}}const tabs=new Tabs;const service=new Service;export{tabs,service};`
	rewritten := rewrite_interceptor_response(t, 1, "https://res.wx.qq.com/virtual_svg-icons-register.publish.test.js", "application/javascript", source)
	if !strings.HasPrefix(rewritten, tabs_and_config) {
		t.Fatal("recommend tabs method or following class boundary was changed")
	}
	for _, event := range []string{"channels:OnFeedProfileLoaded", "channels:JoinLive", "channels:APILoaded"} {
		if !strings.Contains(rewritten, event) {
			t.Fatalf("missing existing capture rule: %s", event)
		}
	}
	result := api.Transform(rewritten, api.TransformOptions{Loader: api.LoaderJS, Format: api.FormatIIFE})
	if len(result.Errors) != 0 {
		t.Fatalf("invalid rewritten JavaScript: %+v", result.Errors)
	}
	runtime := goja.New()
	if _, err := runtime.RunString(`var events=[];var console={log(){}};var WXU={emit(name,data){events.push(name);if(name==="channels:APILoaded")this.services=data;}};`); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.RunString(string(result.Code)); err != nil {
		t.Fatal(err)
	}
	value, err := runtime.RunString(`(async()=>{
		if ((await WXU.services.tabs.getRecommendTabsFromService()).data.object[0].label !== "tabs") throw new Error("tabs response changed");
		if ((await WXU.services.service.finderGetCommentDetail({id:7})).data.object.id !== 7) throw new Error("detail response changed");
		await WXU.services.service.joinLive({id:8});
		if ((await WXU.services.service.untouched()).version !== 2) throw new Error("constructor config changed");
		if (events.join(",") !== "channels:APILoaded,channels:OnFeedProfileLoaded,channels:JoinLive") throw new Error("capture events changed");
	})()`)
	if err != nil {
		t.Fatal(err)
	}
	promise, ok := value.Export().(*goja.Promise)
	if !ok || promise.State() != goja.PromiseStateFulfilled {
		t.Fatalf("class boundary checks failed: %v", value)
	}
}

func TestInterceptorUpstreamScriptCacheKey(t *testing.T) {
	script_path := "https://res.wx.qq.com/app.js"
	html := `<head><script src="` + script_path + `"></script><link rel="modulepreload" href="` + script_path + `"></head>`
	rewritten := rewrite_interceptor_response(t, 0, "https://channels.weixin.qq.com/web/pages/home", "text/html", html)
	if strings.Count(rewritten, script_path+"?wx_channels_script_v2=test") != 2 {
		t.Fatal("entry script and preload do not share the new cache key")
	}
	for _, source := range []string{
		`import value from "./dep.js";`,
		`import "./dep.js";`,
		`const load=()=>import("./dep.js");`,
		`const deps=["js/dep.js"];`,
	} {
		rewritten := rewrite_interceptor_response(t, 1, script_path, "application/javascript", source)
		if !strings.Contains(rewritten, ".js?wx_channels_script_v2=test") || strings.Contains(rewritten, ".js?t=") {
			t.Fatalf("dependency still uses an old cache key: %s", rewritten)
		}
	}
}

func rewrite_interceptor_response(t *testing.T, plugin_index int, raw_url, content_type, source string) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, raw_url, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := &echo.Context{Req: req, Res: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{content_type}},
		Body:       io.NopCloser(strings.NewReader(source)),
	}}
	NewInterceptorPlugins(InterceptorConfig{Version: "test"}, nil)[plugin_index].OnResponse(ctx)
	rewritten, err := ctx.GetResponseBody()
	if err != nil {
		t.Fatal(err)
	}
	return rewritten
}
