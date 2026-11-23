package wxchannels

import (
	"encoding/json"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"

	"github.com/ltaoo/echo"
)

func scriptContext(t *testing.T, method, url, body string) *echo.Context {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return &echo.Context{Req: req, Res: &http.Response{StatusCode: http.StatusOK,
		Header: http.Header{"Content-Type": {"application/javascript"}}, Body: io.NopCloser(strings.NewReader(body))}}
}

func assertScriptBehavior(t *testing.T, source, mode string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required to execute the browser JavaScript regression fixtures")
	}
	input, err := json.Marshal(map[string]string{"source": source, "mode": mode})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, "testdata/interceptor.cjs")
	cmd.Stdin = strings.NewReader(string(input))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("JavaScript regression failed: %v\n%s", err, output)
	}
}

// These hooks intentionally keep access to module-local refs. Execute the actual
// proxy output to guard against dropping navigation when simplifying LiteApp APIs.
func TestInterceptorJavaScriptBehavior(t *testing.T) {
	const navigation = `
const tab = {value:{feeds:[{id:"first"},{id:"second"}],currentFeedIndex:0}};
const local = {value:{feeds:[{id:"playlist"}],currentFeedIndex:0}};
async function next(v){await Promise.resolve();if(v.fail)throw v.fail;tab.value.currentFeedIndex=1;v.done=true}
async function prev(v){await Promise.resolve();if(v.fail)throw v.fail;tab.value.currentFeedIndex=0;v.done=true}
async function load(...args){await Promise.resolve();globalThis.loadArgs=args}
function decodeBase64ToUint64String(){return "decoded"}
function createAdapterFromGlobalMapper(){}
function finderJoinLiveMapper(){}
globalThis.handlers={flowTab:tab,localFlowTab:local,goToNextFlowFeed:next,goToPrevFlowFeed:prev,loadLocalPlaylist:load};
export{decodeBase64ToUint64String};`
	const api = `class API{async finderInit(){return this.request("init")}async finderGetCommentDetail(payload){return this.request("detail",payload)}async untouched(){return "untouched"}};export{API};`
	const preload = `const bridge={async invoke(payload){return {data:{object:payload}}}};const payload=[{id:"preloaded"}];function preload(){ bridge.invoke(payload);(()=>{})()}globalThis.preload=preload;`
	for _, tc := range []struct{ name, path, source, mode string }{
		{"connect navigation", "connect.publish.js", navigation, "navigation"},
		{"applyMic navigation", "applyMic.publish.js", navigation, "navigation"},
		{"API module navigation", "virtual_svg-icons-register.publish.js", navigation, "navigation"},
		{"API init and detail", "virtual_svg-icons-register.publish.js", api, "api"},
		{"API preload", "virtual_svg-icons-register.publish.js", preload, "preload"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plugins := NewInterceptorPlugins(InterceptorConfig{}, nil)
			ctx := scriptContext(t, http.MethodGet, "https://res.wx.qq.com/assets/"+tc.path, tc.source)
			plugins[1].OnResponse(ctx)
			source, err := ctx.GetResponseBody()
			if err != nil {
				t.Fatal(err)
			}
			assertScriptBehavior(t, source, tc.mode)
		})
	}
}
