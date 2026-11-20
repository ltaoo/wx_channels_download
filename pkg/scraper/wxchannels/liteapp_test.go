package wxchannels

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/ltaoo/echo"
)

const liteapp_test_factory = `function(module,exports,require){class API{async finderInit(){return this.post("init")}async finderGetCommentDetail(e){return this.post(e)}async finderGetCommentList(e){return this.post(e)}async end(){}}exports.Z=API;}`

func TestLiteAppRewrittenBusinessMethod(t *testing.T) {
	script, revision, err := rewrite_liteapp_module("123", liteapp_test_factory)
	if err != nil {
		t.Fatal(err)
	}
	runtime := goja.New()
	if _, err := runtime.RunString(`var window=globalThis, factory, events=[],console={log:function(){}}; window.__wxchannels_liteapp_rewrite__={register:function(id,value){factory=value}}; var WXU={emit:function(name,data){events.push([name,data])}};` + script + `
var replacement={};factory({},replacement,function(){});
var api=new replacement.Z();api.token="existing-state";api.post=async function(payload){return {data:{object:payload,token:this.token}}};
api.finderGetCommentDetail("real-request").then(function(result){if(result.data.token!=="existing-state")throw Error("state lost");});`); err != nil {
		t.Fatal(err)
	}
	events, err := runtime.RunString(`JSON.stringify(events)`)
	if err != nil || events.String() != `[["channels:OnFeedProfileLoaded","real-request"]]` {
		t.Fatalf("rewritten API did not emit the real response: %v %v", events, err)
	}
	_, second_revision, err := rewrite_liteapp_module("123", strings.Replace(liteapp_test_factory, `this.post(e)`, `this.post({changed:e})`, 1))
	if err != nil || second_revision == revision {
		t.Fatal("business source changes must change the asset URL digest", err)
	}
	for _, invalid := range []string{liteapp_test_factory + ";sideEffect()", "(" + liteapp_test_factory + ")()", strings.Replace(liteapp_test_factory, "async finderGetCommentDetail", "async renamed", 1)} {
		if _, _, err := rewrite_liteapp_module("123", invalid); err == nil {
			t.Fatal("accepted unsupported or executable module expression")
		}
	}
}

func TestLiteAppCompileAndCache(t *testing.T) {
	plugins := NewInterceptorPlugins(InterceptorConfig{}, nil)
	request := func(method, pathname, body string) *echo.MockResponse {
		ctx := &echo.Context{Req: httptest.NewRequest(method, "https://res.wx.qq.com"+pathname, strings.NewReader(body))}
		ctx.Req.Header.Set("Origin", "https://liteapp.weixin.qq.com")
		plugins[1].OnRequest(ctx)
		return ctx.GetMockResponse()
	}
	input, _ := json.Marshal(map[string]string{"module_id": "123", "source": liteapp_test_factory})
	response := request(http.MethodPost, liteapp_asset_path+"compile", string(input))
	if response == nil || response.StatusCode != http.StatusOK {
		t.Fatalf("compile failed: %+v", response)
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(response.Body.(string)), &result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result["url"], "?v="+result["revision"]) || !strings.Contains(result["bootstrap"], "channels.feed.js") || !strings.Contains(result["bootstrap"], `"assets_base_url":"https://res.wx.qq.com/__assets"`) {
		t.Fatal("missing versioned business URL or existing project injection assets")
	}
	ctx := &echo.Context{Req: httptest.NewRequest(http.MethodGet, result["url"], nil)}
	plugins[1].OnRequest(ctx)
	module_response := ctx.GetMockResponse()
	if module_response == nil || module_response.StatusCode != http.StatusOK || !strings.Contains(module_response.Body.(string), "channels:OnFeedProfileLoaded") || module_response.Headers["Cache-Control"] != "no-store" {
		t.Fatal("rewritten business script was not served without caching")
	}
	ctx = &echo.Context{Req: httptest.NewRequest(http.MethodGet, "https://res.wx.qq.com/__assets/wxchannels/inject/channels.events.js", nil)}
	ctx.Req.Header.Set("If-None-Match", "stale")
	plugins[1].OnRequest(ctx)
	if mock := ctx.GetMockResponse(); mock == nil || mock.StatusCode != http.StatusOK || mock.Headers["Cache-Control"] != "no-store" || mock.Headers["ETag"] != "" || ctx.Req.Header.Get("If-None-Match") != "" {
		t.Fatalf("project asset response was not usable: %+v", mock)
	}
	if response := request(http.MethodPost, liteapp_asset_path+"compile", strings.Repeat("x", (512<<10)+1)); response.StatusCode != http.StatusBadRequest {
		t.Fatal("oversized compilation input accepted")
	}
	ctx = &echo.Context{Req: httptest.NewRequest(http.MethodPost, "https://res.wx.qq.com"+liteapp_asset_path+"compile", strings.NewReader(string(input)))}
	ctx.Req.Header.Set("Origin", "https://example.com")
	plugins[1].OnRequest(ctx)
	if ctx.GetMockResponse().StatusCode != http.StatusForbidden {
		t.Fatal("unexpected origin accepted")
	}
}

func TestLiteAppSDKPreservation(t *testing.T) {
	plugins := NewInterceptorPlugins(InterceptorConfig{}, nil)
	original := "window.libpag={};\n//# sourceMappingURL=sdk.map"
	for _, pathname := range []string{"/t/wx_fed/cdn_libs/res/libpag/4.3.48/libpag.min.js", "/other/sdk.js"} {
		ctx := &echo.Context{Req: httptest.NewRequest(http.MethodGet, "https://res.wx.qq.com"+pathname, nil), Res: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/javascript"}, "Etag": {"old"}}, Body: io.NopCloser(strings.NewReader(original))}}
		ctx.Req.Header.Set("If-None-Match", "old")
		plugins[1].OnRequest(ctx)
		plugins[1].OnResponse(ctx)
		body, _ := ctx.GetResponseBody()
		if strings.Contains(pathname, "libpag") {
			if body != original+"\n;"+liteapp_runtime_js || ctx.GetResponseHeader("Cache-Control") != "no-store" || ctx.GetResponseHeader("ETag") != "" || ctx.Req.Header.Get("If-None-Match") != "" {
				t.Fatal("SDK prefix or cache invalidation changed")
			}
		} else if body != original || ctx.GetResponseHeader("ETag") != "old" {
			t.Fatal("unrelated SDK modified")
		}
	}
}
