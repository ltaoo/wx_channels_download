package douyin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"wx_channel/pkg/cookies"
)

func TestHomeAPIKeepsMediaAndRejectsOldCache(t *testing.T) {
	raw := json.RawMessage(`{"aweme_id":"123","video":{"play_addr":{"url_list":["https://media.example/123.mp4"]}},"author":{"sec_uid":"owner"}}`)
	contents, err := home_contents_from_api_response([]json.RawMessage{raw})
	if err != nil || len(contents) != 1 || string(contents[0].Raw) != string(raw) {
		t.Fatalf("original media data lost: contents=%+v err=%v", contents, err)
	}
	home := &HomeResult{Contents: contents, PaginationKnown: true}
	if !home_cache_has_raw_items(home) {
		t.Fatal("complete cached API items must be reusable")
	}
	home.Contents[0].Raw = nil
	if home_cache_has_raw_items(home) {
		t.Fatal("old cached items without media must be refreshed")
	}
}

const test_home_bdms = `var bdms={init:function(){
if(Uint8Array.from('A',function(c){return c.charCodeAt(0);})[0]!==65)throw new Error('TypedArray.from string support required');
var send=XMLHttpRequest.prototype.send;
XMLHttpRequest.prototype.send=function(){this.url+='&a_bogus=fixture';send.call(this);};
}};`

const test_home_security = `function use(){return function(url){return {url:url,headers:{
'uifid':_secsdk_uifid,'x-secsdk-web-signature':'fixture-signature',
'x-secsdk-web-expire':String(signing_time),'ignored':'must-not-forward'}};};}`

type home_test_transport func(*http.Request) (*http.Response, error)

func (transport home_test_transport) RoundTrip(req *http.Request) (*http.Response, error) {
	return transport(req)
}

func TestHomeSigning(t *testing.T) {
	for _, cookie_header := range []string{"UIFID=preferred; UIFID_TEMP=temporary", "UIFID_TEMP=temporary"} {
		signed, err := sign_home_request(douyin_home_api_url+"?sec_user_id=owner&max_cursor=123", cookie_header+"; s_v_web_id=verify", []string{test_home_bdms, test_home_security})
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest(http.MethodGet, signed.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		query := req.URL.Query()
		if signed.Headers["uifid"] != home_uifid(cookie_header) || len(signed.Headers) != 3 || query.Get("max_cursor") != "123" || query.Get("sec_user_id") != "owner" || query.Get("verifyFp") != "verify" || query.Get("fp") != "verify" || query.Get("a_bogus") != "fixture" || query.Get("timestamp") != signed.Headers["x-secsdk-web-expire"] {
			t.Fatalf("invalid signed request: %+v", signed)
		}
	}
	for _, source := range []string{
		`function use(){return function(){return {url:'https://other.example/?a_bogus=x',headers:{}};};}`,
		strings.ReplaceAll(test_home_security, "fixture-signature", `bad\r\nheader`),
		`throw new Error(document.cookie);`,
	} {
		if _, err := sign_home_request(douyin_home_api_url, "UIFID_TEMP=private-cookie", []string{test_home_bdms, source}); err == nil || strings.Contains(err.Error(), "private-cookie") {
			t.Fatalf("unsafe SDK output must fail without leaking cookies: %v", err)
		}
	}
}

func TestHomeSigningFailureReportsStageWithoutCookies(t *testing.T) {
	for _, test_case := range []struct {
		bdms     string
		security string
		stage    string
	}{
		{`throw new Error(document.cookie);`, test_home_security, "bdms_load"},
		{`var bdms={init:function(){throw new Error(document.cookie);}};`, test_home_security, "bdms_sign"},
		{test_home_bdms, `throw new Error(document.cookie);`, "security_load"},
		{test_home_bdms, `function use(){return function(){throw new Error(document.cookie);};}`, "security_sign"},
	} {
		_, err := sign_home_request(douyin_home_api_url, "UIFID=private-credential", []string{test_case.bdms, test_case.security})
		if err == nil || !strings.Contains(err.Error(), test_case.stage) || strings.Contains(err.Error(), "private-credential") || strings.Contains(err.Error(), "超时") {
			t.Fatalf("incorrect failure diagnostic: %v", err)
		}
	}
}

func TestHomeSDKHTMLResponseUsesNextCDN(t *testing.T) {
	douyin_home_sdk_cache.sources = nil
	t.Cleanup(func() { douyin_home_sdk_cache.sources = nil })
	client := NewClient("UIFID=fixture")
	requests := 0
	client.web.http_client = &http.Client{Transport: home_test_transport(func(req *http.Request) (*http.Response, error) {
		requests++
		body := test_home_bdms
		if req.URL.String() == douyin_home_bdms_urls[0] {
			body = "<!DOCTYPE html><html>CDN challenge</html>"
		}
		if req.URL.String() == douyin_home_security_sdk_url {
			body = test_home_security
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	sources, err := client.load_home_sdk(false)
	if err != nil || requests != 3 || sources[0] != test_home_bdms {
		t.Fatalf("HTML must not enter the SDK cache: requests=%d err=%v", requests, err)
	}
}

func TestHomeAPISigningAndRetry(t *testing.T) {
	for _, test_case := range []struct {
		name          string
		first_status  int
		first_body    string
		want_attempts int
		want_error    bool
	}{
		{"success", 200, `{"status_code":0,"aweme_list":[{"aweme_id":"123"}],"has_more":1,"max_cursor":456}`, 1, false},
		{"rejected_signature", 403, "Blocked by ArgusSecurityPlugin Sign Invalid", 2, false},
		{"rejected_twice", 403, "Blocked by ArgusSecurityPlugin Sign Invalid", 2, true},
		{"empty_response", 200, "", 2, false},
		{"ordinary_403", 403, "Forbidden", 1, true},
		{"login_error", 200, `{"status_code":8,"status_msg":"login required"}`, 1, true},
		{"missing_list", 200, `{"status_code":0}`, 1, true},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			douyin_home_sdk_cache.sources = nil
			t.Cleanup(func() { douyin_home_sdk_cache.sources = nil })
			client := NewClient("UIFID_TEMP=temporary; s_v_web_id=verify")
			api_calls, sdk_calls := 0, 0
			client.web.http_client = &http.Client{Transport: home_test_transport(func(req *http.Request) (*http.Response, error) {
				status, body := 200, ""
				if req.URL.Host == "www-hj.douyin.com" {
					api_calls++
					query := req.URL.Query()
					if query.Get("max_cursor") != "123" || query.Get("a_bogus") != "fixture" || query.Get("uifid") != "temporary" || req.Header.Get("uifid") != "temporary" || req.Header.Get("Cookie") != "UIFID_TEMP=temporary; s_v_web_id=verify" || query.Get("timestamp") != req.Header.Get("x-secsdk-web-expire") || req.Header.Get("x-secsdk-web-signature") != "fixture-signature" {
						t.Fatal("API must receive signed query, timestamp, headers and original cookie names")
					}
					status, body = test_case.first_status, test_case.first_body
					if api_calls > 1 && !test_case.want_error {
						status, body = 200, `{"status_code":0,"aweme_list":[{"aweme_id":"123"}],"has_more":1,"max_cursor":456}`
					}
				} else {
					sdk_calls++
					if req.Header.Get("Cookie") != "" {
						t.Fatal("SDK CDN must not receive login cookies")
					}
					body = test_home_bdms
					if req.URL.String() == douyin_home_security_sdk_url {
						body = test_home_security
					}
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: req}, nil
			})}
			list, err := client.FetchHomeContentsPage("https://www.douyin.com/user/owner", "123")
			if (err != nil) != test_case.want_error || api_calls != test_case.want_attempts || sdk_calls != 2*test_case.want_attempts {
				t.Fatalf("api=%d sdk=%d err=%v", api_calls, sdk_calls, err)
			}
			if err == nil && (list.NextMarker != "456" || len(list.Items) != 1 || list.Items[0].ID != "123") {
				t.Fatalf("invalid pagination: %+v", list)
			}
		})
	}
}

func TestHomeCookieSources(t *testing.T) {
	work_dir := t.TempDir()
	reader := cookies.NewPersistentReader(work_dir)
	client := NewClientWithLoggerAndCookieReader("UIFID=configured", reader, nil)
	if err := cookies.SaveJSON([]cookies.Cookie{{Name: "UIFID_TEMP", Value: "stored", Domain: ".douyin.com", Path: "/"}}, filepath.Join(work_dir, "cookies.json")); err != nil {
		t.Fatal(err)
	}
	if got := client.home_cookie_header(); got != "UIFID=configured" {
		t.Fatalf("explicit cookie must take precedence: %q", got)
	}
	client.web.cookie = ""
	if got := home_uifid(client.home_cookie_header()); got != "stored" {
		t.Fatalf("persistent UIFID_TEMP must be supported: %q", got)
	}
	client.cookie_reader = nil
	client.web.http_client = &http.Client{Transport: home_test_transport(func(req *http.Request) (*http.Response, error) {
		t.Error("missing cookies must fail before network access")
		return nil, fmt.Errorf("unexpected request")
	})}
	if _, err := client.FetchHomeContentsPage("https://www.douyin.com/user/owner", "123"); err == nil || !strings.Contains(err.Error(), "UIFID_TEMP") {
		t.Fatalf("missing cookie must return an actionable error: %v", err)
	}
}
