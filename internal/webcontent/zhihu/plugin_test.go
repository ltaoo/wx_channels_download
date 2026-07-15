package zhihu

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"wx_channel/internal/interceptor/proxy"
)

func TestCookiesFromSetCookieHeaders(t *testing.T) {
	got := cookiesFromSetCookieHeaders([]string{
		`z_c0=abc123; Path=/; Domain=.zhihu.com; HttpOnly; Secure`,
		`SESSIONID=xyz; Max-Age=3600; SameSite=None`,
	})

	want := []string{"z_c0=abc123", "SESSIONID=xyz"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cookie[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMergeCookieHeadersOverwritesExistingValues(t *testing.T) {
	got := mergeCookieHeaders("a=1; b=2", "b=3; c=4")
	want := "a=1; b=3; c=4"
	if got != want {
		t.Fatalf("merged = %q, want %q", got, want)
	}
}

func TestMergeCookieHeadersUsesSetCookieOutput(t *testing.T) {
	setCookies := cookiesFromSetCookieHeaders([]string{
		`z_c0=new-token; Path=/; Domain=.zhihu.com; HttpOnly`,
	})

	got := mergeCookieHeaders("q_c1=old; z_c0=old-token", setCookies...)
	want := "q_c1=old; z_c0=new-token"
	if got != want {
		t.Fatalf("merged = %q, want %q", got, want)
	}
}

func TestCaptureZhihuCookieStoresMergedCookieInViper(t *testing.T) {
	configFile := useTempViperConfig(t)
	viper.Set("zhihu.cookie", "q_c1=old")

	ctx := &fakeZhihuContext{
		req: &proxy.ContextReq{
			URL: &proxy.ContextURL{
				Hostname: func() string { return "www.zhihu.com" },
			},
		},
		res: &proxy.ContextRes{
			Header: http.Header{
				"Set-Cookie": []string{
					`z_c0=new-token; Path=/; Domain=.zhihu.com; HttpOnly`,
				},
			},
		},
	}

	captureZhihuCookie(ctx)

	got := viper.GetString("zhihu.cookie")
	want := "q_c1=old; z_c0=new-token"
	if got != want {
		t.Fatalf("zhihu.cookie = %q, want %q", got, want)
	}
	assertPersistedCookie(t, configFile, want)
}

func TestCaptureZhihuRequestCookieStoresFullRequestCookie(t *testing.T) {
	configFile := useTempViperConfig(t)
	viper.Set("zhihu.cookie", "z_c0=old")

	ctx := &fakeZhihuContext{
		req: &proxy.ContextReq{
			URL: &proxy.ContextURL{
				Hostname: func() string { return "www.zhihu.com" },
			},
			Header: http.Header{
				"Cookie": []string{"q_c1=abc; z_c0=new-token; SESSIONID=session-value"},
			},
		},
	}

	captureZhihuRequestCookie(ctx)

	got := viper.GetString("zhihu.cookie")
	want := "z_c0=new-token; q_c1=abc; SESSIONID=session-value"
	if got != want {
		t.Fatalf("zhihu.cookie = %q, want %q", got, want)
	}
	assertPersistedCookie(t, configFile, want)
}

func useTempViperConfig(t *testing.T) string {
	t.Helper()

	viper.Reset()
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	viper.SetConfigFile(configFile)
	t.Cleanup(viper.Reset)
	return configFile
}

func assertPersistedCookie(t *testing.T, configFile, want string) {
	t.Helper()

	content, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}
	if !strings.Contains(string(content), want) {
		t.Fatalf("config file does not contain persisted cookie %q:\n%s", want, string(content))
	}
}

type fakeZhihuContext struct {
	req *proxy.ContextReq
	res *proxy.ContextRes
}

func (c *fakeZhihuContext) Req() *proxy.ContextReq {
	return c.req
}

func (c *fakeZhihuContext) Res() *proxy.ContextRes {
	return c.res
}

func (c *fakeZhihuContext) Mock(status int, headers map[string]string, body string) {}

func (c *fakeZhihuContext) GetResponseHeader(key string) string {
	if c.res == nil || c.res.Header == nil {
		return ""
	}
	return c.res.Header.Get(key)
}

func (c *fakeZhihuContext) SetResponseHeader(key, val string) {}

func (c *fakeZhihuContext) SetResponseBody(body string) {}

func (c *fakeZhihuContext) GetResponseBody() ([]byte, error) {
	return io.ReadAll(c.res.Body)
}

func (c *fakeZhihuContext) SetStatusCode(code int) {}
