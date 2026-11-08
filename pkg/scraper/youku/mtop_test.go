package youku

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
	"time"

	"wx_channel/pkg/scraper"
)

const mtop_page_fixture = `<script src="//g.alicdn.com/AWSC/AWSC/awsc.js"></script><script src="https://g.alicdn.com/kplayer/kuplayer/9.8.7/index.js"></script>`
const mtop_player_fixture = `function Player(){this.version="9.8.7",this.interactiveMode=false} var defaults={ccode:"0502",autoplay:true};`
const mtop_uab_fixture = `AWSCInner.register("uabModule","1",{getUA:function(){return "computed-in-goja:"+location.pathname+":"+navigator.userAgent.length}});`

func TestMTOPPlaybackRunsFetchedJSAndRefreshesToken(t *testing.T) {
	for _, homepage_fallback := range []bool{false, true} {
		t.Run(fmt.Sprint(homepage_fallback), func(t *testing.T) {
			jar, _ := cookiejar.New(nil)
			api_url, _ := url.Parse(playback_api)
			jar.SetCookies(api_url, []*http.Cookie{{Name: "caller", Value: "original"}})
			mtop_calls, homepage_calls := 0, 0
			http_client := &http.Client{Jar: jar, Transport: round_trip_func(func(req *http.Request) (*http.Response, error) {
				body, header := "", make(http.Header)
				switch req.URL.Hostname() {
				case "log.mmstat.com":
					header.Set("ETag", `"fresh-utid"`)
				case "ups.youku.com":
					body = `{"data":{"error":{"code":-6004}}}`
				case "v.youku.com":
					body = mtop_page_fixture
					if homepage_fallback {
						body = `<script>throw new Error("this page script must never execute")</script>`
					}
				case "www.youku.com":
					homepage_calls++
					body = mtop_page_fixture
				case "g.alicdn.com":
					if req.Header.Get("Cookie") != "" {
						t.Fatal("credentials leaked to the script CDN")
					}
					switch req.URL.Path {
					case "/AWSC/AWSC/awsc.js":
						body = `var config={uabModule:{grey:["AWSC/uab/0.0.1/collina.js"],stable:["AWSC/uab/9.8.7/collina.js"]}};`
					case "/kplayer/kuplayer/9.8.7/index.js":
						body = mtop_player_fixture
					case "/AWSC/uab/9.8.7/collina.js":
						body = mtop_uab_fixture
					default:
						t.Fatalf("unexpected script address: %s", req.URL.Path)
					}
				case "acs.youku.com":
					mtop_calls++
					query := req.URL.Query()
					var input map[string]string
					if json.Unmarshal([]byte(query.Get("data")), &input) != nil {
						t.Fatal("MTOP parameters were not encoded")
					}
					var steal map[string]any
					_ = json.Unmarshal([]byte(input["steal_params"]), &steal)
					if steal["version"] != "9.8.7" || steal["ccode"] != "0502" || steal["utid"] != "saved-cna" || !strings.HasPrefix(fmt.Sprint(steal["ckey"]), "computed-in-goja:/v_show/id_"+test_youku_id) {
						t.Fatal("fetched JS was not used for playback parameters")
					}
					token := "stale"
					if mtop_calls > 1 {
						token = "refreshed"
					}
					if strings.Count(req.Header.Get("Cookie"), "_m_h5_tk=") != 1 || !strings.Contains(req.Header.Get("Cookie"), "_m_h5_tk="+token+"_") || !strings.Contains(req.Header.Get("Cookie"), "P_sck=configured-account") {
						t.Fatal("session lost its account or retained a stale token")
					}
					expected := fmt.Sprintf("%x", md5.Sum([]byte(token+"&"+query.Get("t")+"&24679788&"+query.Get("data"))))
					if query.Get("sign") != expected {
						t.Fatal("MTOP signature did not use the current session token")
					}
					if mtop_calls == 1 {
						header.Add("Set-Cookie", "_m_h5_tk=refreshed_9999999999999; Domain=.youku.com; Path=/; Secure")
						body = `{"ret":["FAIL_SYS_TOKEN_EXOIRED::expired"],"data":{}}`
					} else {
						body = fmt.Sprintf(`{"ret":["SUCCESS::ok"],"data":{"data":{"video":{"encodeid":%q,"title":"主动提取","seconds":2},"stream":[{"m3u8_url":"https://pl-ali.youku.com/index.m3u8","width":32,"height":32,"milliseconds_video":2000}]}}}`, test_youku_id)
						if homepage_fallback {
							var envelope struct {
								Data struct {
									Data json.RawMessage `json:"data"`
								} `json:"data"`
							}
							if err := json.Unmarshal([]byte(body), &envelope); err != nil {
								t.Fatal(err)
							}
							encoded, _ := json.Marshal(string(envelope.Data.Data))
							body = `{"ret":["SUCCESS::ok"],"data":{"data":` + string(encoded) + `}}`
						}
					}
				case "pl-ali.youku.com":
					if req.Header.Get("Cookie") != "" || req.Header.Get("User-Agent") != youku_user_agent {
						t.Fatal("media request leaked cookies or changed its source user agent")
					}
					body = "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXTINF:2,\nsegment.ts\n#EXT-X-ENDLIST\n"
				default:
					t.Fatalf("unexpected host: %s", req.URL.Hostname())
				}
				return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
			})}
			client := NewClient("cna=saved-cna; _m_h5_tk=stale_9999999999999; P_sck=configured-account")
			client.SetHTTPClient(http_client)
			video, err := client.ResolveContext(context.Background(), "https://v.youku.com/v_show/id_"+test_youku_id+".html", scraper.Episode{ID: test_youku_id})
			if err != nil || video.Title != "主动提取" || video.Info.Duration != 2 || video.CheckMediaAddress == nil || mtop_calls != 2 || homepage_calls != map[bool]int{false: 0, true: 1}[homepage_fallback] {
				t.Fatalf("active extraction failed: calls=%d homepage=%d error=%v", mtop_calls, homepage_calls, err)
			}
			if http_client.Jar != jar || cookie_value(jar, api_url, "caller") != "original" || cookie_value(jar, api_url, "_m_h5_tk") != "" {
				t.Fatal("extraction mutated the caller's shared cookie jar")
			}
		})
	}
}

func TestMTOPSDKFallbackBoundsTokenRetriesAndDoesNotRetryAccountDenials(t *testing.T) {
	for _, token_error := range []bool{false, true} {
		t.Run(fmt.Sprint(token_error), func(t *testing.T) {
			client := new_backend("P_sck=configured-account").(*backend)
			calls, scripts := 0, 0
			client.SetHTTPClient(&http.Client{Transport: round_trip_func(func(req *http.Request) (*http.Response, error) {
				body := ""
				switch req.URL.Hostname() {
				case "v.youku.com", "www.youku.com":
					body = `<script>throw new Error("do not execute this page")</script>`
				case "g.alicdn.com":
					scripts++
					switch req.URL.String() {
					case fallback_player_address:
						body = mtop_player_fixture
					case fallback_awsc_address:
						body = `var config={uabModule:{stable:["AWSC/uab/9.8.7/collina.js"]}};`
					default:
						body = mtop_uab_fixture
					}
				case "acs.youku.com":
					calls++
					body = `{"ret":["SUCCESS::ok"],"data":{"data":{"error":{"code":-6005,"note":"private upstream account information"}}}}`
					if token_error {
						body = `{"ret":["FAIL_SYS_TOKEN_EMPTY::private upstream details"],"data":{}}`
					}
				default:
					t.Fatal("unexpected source request")
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
			})})
			_, err := client.load_mtop_playback(context.Background(), test_youku_id, "fixture-utid")
			want_calls := 1
			if token_error {
				want_calls = 3
			}
			if err == nil || calls != want_calls || scripts != 3 || strings.Contains(err.Error(), "private upstream") {
				t.Fatalf("incorrect denial handling: calls=%d scripts=%d error=%v", calls, scripts, err)
			}
		})
	}
}

func TestCKEYExecutionRejectsFailureAndHonorsCancellation(t *testing.T) {
	for _, script := range []string{
		`AWSCInner.register("otherModule","1",{getUA:function(){return "unexpected-module"}})`,
		`AWSCInner.register("uabModule","1",{getUA:function(){return "defaultUA_failed"}})`,
		`AWSCInner.register("uabModule","1",{getUA:function(){return null}})`,
		`throw new Error("private Cookie or signed URL detail")`,
	} {
		_, err := calculate_ckey(context.Background(), script, "https://v.youku.com/v_show/id_"+test_youku_id+".html", "")
		if err == nil || strings.Contains(err.Error(), "private Cookie") {
			t.Fatalf("invalid result accepted or script details exposed: %v", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := calculate_ckey(ctx, `AWSCInner.register("uabModule","1",{getUA:function(){while(true){}}})`, "https://v.youku.com/", "")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Goja execution ignored cancellation: %v", err)
	}
}

func TestSigningScriptsRejectUntrustedAddressesAndRedirects(t *testing.T) {
	for _, address := range []string{
		"http://g.alicdn.com/AWSC/AWSC/awsc.js",
		"https://g.alicdn.com.evil.example/AWSC/AWSC/awsc.js",
		"https://g.alicdn.com/other/script.js",
		"https://g.alicdn.com:8443/AWSC/AWSC/awsc.js",
		"https://g.alicdn.com/AWSC/uab/../AWSC/awsc.js",
	} {
		if check_script_address(address) == nil {
			t.Fatal("untrusted script address accepted")
		}
	}
	client := new_backend("P_sck=private").(*backend)
	requests := 0
	client.SetHTTPClient(&http.Client{Transport: round_trip_func(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Header.Get("Cookie") != "" || req.URL.Hostname() != "g.alicdn.com" {
			t.Fatal("SDK redirect leaked a cookie or left its trusted host")
		}
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": {"https://evil.example/sdk.js"}}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
	})})
	_, err := client.fetch_script(context.Background(), "https://g.alicdn.com/AWSC/AWSC/awsc.js")
	if err == nil || requests != 1 {
		t.Fatalf("untrusted redirect followed: requests=%d error=%v", requests, err)
	}
}
