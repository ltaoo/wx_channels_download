package zhihu

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestParseAnswerURL(t *testing.T) {
	got, ok := ParseAnswerURL("https://www.zhihu.com/question/14844743711/answer/2040749293515043854")
	if !ok {
		t.Fatal("ParseAnswerURL returned false")
	}
	if got.QuestionID != "14844743711" || got.AnswerID != "2040749293515043854" {
		t.Fatalf("ParseAnswerURL = %#v", got)
	}
}

func TestBuildHTMLFromInitialData(t *testing.T) {
	body := []byte(`<!doctype html><html><body><script id="js-initialData" type="text/json">{"initialState":{"entities":{"questions":{"14844743711":{"id":"14844743711","title":"网文写作怎么样把控情绪节奏啊","detail":"<p>问题详情</p>","author":{"name":"金默语"}}},"answers":{"2040749293515043854":{"id":"2040749293515043854","content":"<p>回答内容</p>\u003Cscript>bad()\u003C/script>","commentCount":0,"author":{"name":"身体里三千个暴君","urlToken":"zhang-gong-zi-40-74","headline":"一个怕死鬼","avatarUrl":"https://picx.zhimg.com/avatar.jpg"}}}}}}</script><script src="https://static.zhihu.com/heifetz/main.js"></script></body></html>`)
	answerURL, ok := ParseAnswerURL("https://www.zhihu.com/question/14844743711/answer/2040749293515043854")
	if !ok {
		t.Fatal("ParseAnswerURL returned false")
	}
	page, err := parseAnswerPage(body, answerURL)
	if err != nil {
		t.Fatal(err)
	}
	out := BuildHTML(page)
	for _, want := range []string{
		"问题作者：金默语",
		"问题原始链接：<a href=\"https://www.zhihu.com/question/14844743711\">",
		"回答作者：<a class=\"author-name\" href=\"https://www.zhihu.com/people/zhang-gong-zi-40-74\">身体里三千个暴君</a>",
		"<img class=\"avatar\" src=\"https://picx.zhimg.com/avatar.jpg\"",
		"一个怕死鬼",
		"网文写作怎么样把控情绪节奏啊",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q", want)
		}
	}
	for _, unwanted := range []string{"WX-ZHIHU-PLUGIN-INJECTED", "static.zhihu.com/heifetz", "<script"} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("output contains %q", unwanted)
		}
	}
}

func TestInlineRemoteImages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "image/png")
		_, _ = w.Write([]byte{0x89, 0x50, 0x4e, 0x47})
	}))
	defer server.Close()

	input := `<!doctype html><html><body><img class="avatar" src="` + server.URL + `/avatar.png"><div class="content"><img src="/inline.png"></div></body></html>`
	client := &Client{HTTPClient: server.Client()}
	out, err := client.inlineRemoteImages(input, server.URL+"/answer")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, server.URL) || strings.Contains(out, `src="/inline.png"`) {
		t.Fatalf("remote image src was not replaced: %s", out)
	}
	if count := strings.Count(out, "data:image/png;base64,"); count != 2 {
		t.Fatalf("embedded image count = %d, want 2: %s", count, out)
	}
}

func TestSetHeadersMatchesBrowserNavigationHeaders(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://www.zhihu.com/question/1/answer/2", nil)
	if err != nil {
		t.Fatal(err)
	}
	setHeaders(req, "https://www.zhihu.com/question/1/answer/2")

	wants := map[string]string{
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"Priority":                  "u=0, i",
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "none",
		"Sec-Fetch-User":            "?1",
		"Upgrade-Insecure-Requests": "1",
	}
	for key, want := range wants {
		if got := req.Header.Get(key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestDoBytesErrorIncludesCookieDiagnostics(t *testing.T) {
	oldCookie := viper.GetString("zhihu.cookie")
	t.Cleanup(func() {
		viper.Set("zhihu.cookie", oldCookie)
	})
	viper.Set("zhihu.cookie", "z_c0=secret-z; d_c0=\"secret-d\"; __zse_ck=secret-ck; _xsrf=secret-xsrf")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("cookie") == "" {
			t.Fatal("request missing cookie header")
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<!doctype html><meta id="zh-zse-ck" content="challenge">`))
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client()}
	_, err := client.doBytes(http.MethodGet, server.URL, server.URL)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	idx := strings.Index(msg, "debug=")
	if idx < 0 {
		t.Fatalf("error missing debug json: %s", msg)
	}
	var debug map[string]any
	if err := json.Unmarshal([]byte(msg[idx+len("debug="):]), &debug); err != nil {
		t.Fatalf("parse debug json: %v: %s", err, msg)
	}
	cookie := debug["cookie"].(map[string]any)
	if cookie["present"] != true {
		t.Fatalf("cookie.present = %v, want true", cookie["present"])
	}
	keys := strings.Join(anyStrings(cookie["keys"]), ",")
	if keys != "__zse_ck,_xsrf,d_c0,z_c0" {
		t.Fatalf("cookie keys = %q", keys)
	}
	has := cookie["has"].(map[string]any)
	for _, key := range []string{"d_c0", "z_c0", "__zse_ck", "_xsrf"} {
		if has[key] != true {
			t.Fatalf("cookie.has.%s = %v, want true", key, has[key])
		}
	}
	bodyDebug := debug["body"].(map[string]any)
	if bodyDebug["zse_ck"] != true {
		t.Fatalf("body.zse_ck = %v, want true", bodyDebug["zse_ck"])
	}
	if debug["diagnosis"] != "zhihu_zse_ck_challenge" {
		t.Fatalf("diagnosis = %v", debug["diagnosis"])
	}
	if hint, _ := debug["hint"].(string); !strings.Contains(hint, "Refresh zhihu.cookie") {
		t.Fatalf("hint = %q", hint)
	}
	requestDebug := debug["request"].(map[string]any)
	if requestDebug["method"] != http.MethodGet || requestDebug["url"] != server.URL {
		t.Fatalf("request debug = %#v", requestDebug)
	}
	curl, _ := requestDebug["curl"].(string)
	for _, want := range []string{"curl -i --max-time 20", "Cookie: <paste zhihu.cookie from config>", "User-Agent:"} {
		if !strings.Contains(curl, want) {
			t.Fatalf("curl missing %q: %s", want, curl)
		}
	}
	for _, secret := range []string{"secret-z", "secret-d", "secret-ck", "secret-xsrf"} {
		if strings.Contains(msg, secret) {
			t.Fatalf("error leaked cookie value %q: %s", secret, msg)
		}
	}
}

func anyStrings(values any) []string {
	raw, ok := values.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, value := range raw {
		if s, ok := value.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func TestSanitizeFragmentUsesZhihuOriginalImage(t *testing.T) {
	fragment := `<p><img src="https://picx.zhimg.com/80/v2-thumb_1440w.webp?source=2c26e567" data-original="https://pic1.zhimg.com/v2-original_r.jpg?source=2c26e567" data-actualsrc="https://picx.zhimg.com/50/v2-actual_720w.jpg?source=2c26e567" width="638" height="481" class="origin_image lazy"></p>`
	out := sanitizeFragment(fragment)
	for _, want := range []string{
		`src="https://pic1.zhimg.com/v2-original_r.jpg?source=2c26e567"`,
		`width="638"`,
		`height="481"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q: %s", want, out)
		}
	}
	for _, unwanted := range []string{"data-original", "data-actualsrc", "class="} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("output contains %q: %s", unwanted, out)
		}
	}
}

func TestFirstImageURL(t *testing.T) {
	tests := []struct {
		name     string
		fragment string
		base     string
		want     string
	}{
		{
			name:     "uses original image",
			fragment: `<p><img src="https://picx.zhimg.com/80/v2-thumb_1440w.webp" data-original="https://pic1.zhimg.com/v2-original_r.jpg"></p>`,
			base:     "https://www.zhihu.com/question/1/answer/2",
			want:     "https://pic1.zhimg.com/v2-original_r.jpg",
		},
		{
			name:     "normalizes relative image",
			fragment: `<p><img src="/assets/image.png"></p>`,
			base:     "https://www.zhihu.com/question/1/answer/2",
			want:     "https://www.zhihu.com/assets/image.png",
		},
		{
			name:     "no image stays empty",
			fragment: `<p>回答内容</p>`,
			base:     "https://www.zhihu.com/question/1/answer/2",
			want:     "",
		},
		{
			name:     "placeholder stays empty",
			fragment: `<p><img src="data:image/svg+xml;base64,abc"></p>`,
			base:     "https://www.zhihu.com/question/1/answer/2",
			want:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FirstImageURL(tt.fragment, tt.base); got != tt.want {
				t.Fatalf("FirstImageURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
