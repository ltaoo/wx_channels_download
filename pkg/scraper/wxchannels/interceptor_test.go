package wxchannels

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/evanw/esbuild/pkg/api"
	"github.com/ltaoo/echo"
)

func TestInterceptorAPI4(t *testing.T) {
	method_names := []string{
		"finderGetFollowList",
		"finderGetPlayHistory",
		"finderGetInteractionedFeedList",
		"finderGetFeedH5Url",
	}
	cases := []struct {
		name       string
		groups     [][]int
		separator  string
		parameter  string
		whitespace string
		preamble   string
	}{
		{name: "interactioned_last", groups: [][]int{{0, 1, 2}}, separator: "const", parameter: "t"},
		{name: "interactioned_before_h5", groups: [][]int{{0, 1, 2, 3}}, separator: "const", parameter: "t"},
		{name: "follow_last", groups: [][]int{{2, 1, 0}}, separator: "let", parameter: "t"},
		{name: "history_last", groups: [][]int{{0, 2, 1}}, separator: ";const", parameter: "t"},
		{name: "split_services", groups: [][]int{{2}, {1}, {0}}, separator: ";var", parameter: "t"},
		{name: "multiline_dollar_parameter", groups: [][]int{{0, 2, 1, 3}}, separator: "const", parameter: "$t", whitespace: "\n"},
		{name: "early_return_before_local_const", groups: [][]int{{0, 2, 1, 3}}, separator: "const", parameter: "t", preamble: `if(t.skip){return {data:{object:[]}}}const extra = 1;`},
		{name: "nested_literals", groups: [][]int{{0, 2, 1, 3}}, separator: "const", parameter: "t", preamble: "const text = '}async finderGetFeedH5Url(t){'; const pattern = /[{}]/; const label = `${t.tabFlag}:}}const`; /* }async */"},
	}
	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			var source strings.Builder
			var exports []string
			for group_index, group := range test_case.groups {
				fmt.Fprintf(&source, "class Service%d{", group_index)
				for _, method_index := range group {
					fmt.Fprintf(&source, "async %s(%s){%s%sreturn {method:%q,payload:%s,data:{object:[this.marker,%s.tabFlag]}};%s}",
						method_names[method_index], test_case.parameter, test_case.whitespace, test_case.preamble,
						method_names[method_index], test_case.parameter, test_case.parameter, test_case.whitespace)
				}
				fmt.Fprintf(&source, "}%s service%d=new Service%d;service%d.marker='ok';", test_case.separator, group_index, group_index, group_index)
				exports = append(exports, fmt.Sprintf("service%d as api%d", group_index, group_index))
			}
			service_source := source.String()
			fmt.Fprintf(&source, "export{%s};", strings.Join(exports, ","))

			runtime := new_api_test_runtime(t)
			rewritten := rewrite_api_test_script(t, source.String())
			if !strings.HasPrefix(rewritten, service_source) {
				t.Fatal("rewrite changed the original service definitions")
			}
			transformed := api.Transform(rewritten, api.TransformOptions{Loader: api.LoaderJS, Format: api.FormatIIFE})
			if len(transformed.Errors) > 0 {
				t.Fatalf("rewritten JavaScript is invalid: %+v\n%s", transformed.Errors, rewritten)
			}
			if _, err := runtime.RunString(string(transformed.Code)); err != nil {
				t.Fatalf("load rewritten services: %v", err)
			}
			// Aliases and repeated announcements must not wrap a service twice.
			if _, err := runtime.RunString(`var service_exports = WXU.last_services; WXU.emit("channels:APILoaded", service_exports);`); err != nil {
				t.Fatal(err)
			}
			// A later service announcement must not erase previously registered methods.
			if _, err := runtime.RunString(`WXU.emit("channels:APILoaded", {later: {finderGetFollowList: WXU.API4.finderGetFollowList}});`); err != nil {
				t.Fatal(err)
			}
			available_methods := method_names[:3]
			if strings.Contains(source.String(), "async finderGetFeedH5Url(") {
				available_methods = method_names
			}
			for _, method_name := range available_methods {
				value, err := runtime.RunString(fmt.Sprintf(`WXU.API4.%s({tabFlag:7})`, method_name))
				if err != nil {
					t.Fatalf("call %s: %v", method_name, err)
				}
				promise, ok := value.Export().(*goja.Promise)
				if !ok || promise.State() != goja.PromiseStateFulfilled {
					t.Fatalf("%s did not fulfill: %v", method_name, value)
				}
				result := promise.Result().ToObject(runtime)
				if result.Get("method").String() != method_name || result.Get("payload").ToObject(runtime).Get("tabFlag").ToInteger() != 7 {
					t.Fatalf("%s changed the response or payload: %v", method_name, result)
				}
				object := result.Get("data").ToObject(runtime).Get("object").ToObject(runtime)
				if object.Get("0").String() != "ok" {
					t.Fatalf("%s lost its service receiver", method_name)
				}
			}
			value, err := runtime.RunString(`events.filter(event => event[0] === "channels:InteractionedFeedsLoaded").map(event => event[1].join(":")).join(",")`)
			if err != nil || value.String() != "ok:7" {
				t.Fatalf("interactioned feeds event: value=%v err=%v", value, err)
			}
			_, err = runtime.RunString(`
				Object.values(service_exports).find(service => typeof service.finderGetInteractionedFeedList === "function").finderGetInteractionedFeedList({tabFlag:8});
			`)
			if err != nil {
				t.Fatal(err)
			}
			value, err = runtime.RunString(`events.filter(event => event[0] === "channels:InteractionedFeedsLoaded").map(event => event[1].join(":")).join(",")`)
			if err != nil || value.String() != "ok:7,ok:8" {
				t.Fatalf("native service call event: value=%v err=%v", value, err)
			}
		})
	}
}

func TestAPI4PreservesRejection(t *testing.T) {
	runtime := new_api_test_runtime(t)
	value, err := runtime.RunString(`
		var expected_error = new Error("request failed");
		var service = {async finderGetInteractionedFeedList() {throw expected_error;}};
		WXU.emit("channels:APILoaded", {empty: null, service});
		WXU.API4.finderGetInteractionedFeedList({tabFlag:7});
	`)
	if err != nil {
		t.Fatal(err)
	}
	promise, ok := value.Export().(*goja.Promise)
	if !ok || promise.State() != goja.PromiseStateRejected || !promise.Result().SameAs(runtime.Get("expected_error")) {
		t.Fatalf("original request error was not preserved: %v", value)
	}
	value, err = runtime.RunString(`events.filter(event => event[0] === "channels:InteractionedFeedsLoaded").length`)
	if err != nil || value.ToInteger() != 0 {
		t.Fatalf("failed request emitted a feeds event: value=%v err=%v", value, err)
	}
}

func TestInterceptorInvalidRewriteFallsBack(t *testing.T) {
	// The flow replacement would insert a function inside this string literal.
	source := `const text="goToNextFlowFeed:next";export{text};`
	if rewritten := rewrite_api_test_script(t, source); rewritten != source {
		t.Fatalf("invalid rewrite was served instead of the original script: %s", rewritten)
	}
}

func new_api_test_runtime(t *testing.T) *goja.Runtime {
	t.Helper()
	runtime := goja.New()
	_, err := runtime.RunString(`
		var window = globalThis;
		window.__d_config = {version: "test"};
		var console = {log() {}};
		var events = [];
		var WXU = {
			onAPILoaded(handler) { this.api_loaded = handler; },
			onUtilsLoaded() {},
			onInit() {},
			emit(name, data) {
				events.push([name, data]);
				if (name === "channels:APILoaded") {
					this.last_services = data;
					this.api_loaded(data);
				}
			}
		};
	`)
	if err != nil {
		t.Fatal(err)
	}
	utils, err := fs.ReadFile(InjectAssets(), "channels.utils.js")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.RunString(string(utils)); err != nil {
		t.Fatalf("load channels.utils.js: %v", err)
	}
	return runtime
}

func rewrite_api_test_script(t *testing.T, source string) string {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, "https://res.wx.qq.com/test/virtual_svg-icons-register.publish.test.js", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := &echo.Context{
		Req: request,
		Res: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/javascript"}},
			Body:       io.NopCloser(strings.NewReader(source)),
		},
	}
	NewInterceptorPlugins(InterceptorConfig{}, nil)[1].OnResponse(ctx)
	rewritten, err := ctx.GetResponseBody()
	if err != nil {
		t.Fatal(err)
	}
	return rewritten
}
