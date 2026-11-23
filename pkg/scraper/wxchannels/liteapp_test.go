package wxchannels

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/ltaoo/echo"
	"github.com/rs/zerolog"
)

func TestLiteAppLoaderOrigin(t *testing.T) {
	logger := zerolog.Nop()
	ctx := &echo.Context{
		Req: httptest.NewRequest(http.MethodGet, "https://res.wx.qq.com/t/wx_fed/cdn_libs/res/libpag/4.0/libpag.min.js", nil),
		Res: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/javascript"}},
			Body:       io.NopCloser(strings.NewReader("globalThis.sdk_loaded = true;")),
		},
	}
	if !inject_liteapp_runtime(ctx, &logger, false) {
		t.Fatal("SDK response was not handled")
	}
	source, err := ctx.GetResponseBody()
	if err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"https://liteapp.weixin.qq.com", "https://channels.weixin.qq.com", "https://example.com"} {
		t.Run(origin, func(t *testing.T) {
			vm := goja.New()
			if err := vm.Set("location", map[string]string{"origin": origin}); err != nil {
				t.Fatal(err)
			}
			if _, err := vm.RunString(`
var loaded_scripts = [];
var document = {
  createElement() { return { setAttribute() {} }; },
  head: { appendChild(script) { loaded_scripts.push(script); } }
};` + source); err != nil {
				t.Fatal(err)
			}
			if !vm.Get("sdk_loaded").ToBoolean() {
				t.Fatal("original SDK did not execute")
			}
			loaded, err := vm.RunString("loaded_scripts.length")
			if err != nil {
				t.Fatal(err)
			}
			want := int64(0)
			if origin == "https://liteapp.weixin.qq.com" {
				want = 1
			}
			if loaded.ToInteger() != want {
				t.Fatalf("loaded %d runtime scripts, want %d", loaded.ToInteger(), want)
			}
		})
	}
}
