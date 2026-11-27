package instagram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"wx_channel/pkg/cookies"

	"github.com/tidwall/gjson"
)

type homepage_test_transport func(*http.Request) (*http.Response, error)

func (transport homepage_test_transport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

const homepage_test_account = `{"pk":"123","username":"example","full_name":"Example"}`
const homepage_test_node = `{"pk":"9007199254740993","code":"Post_1","media_type":2,"caption":{"text":"Post"},"image_versions2":{"candidates":[{"url":"https://cdninstagram.com/cover.jpg"}]},"user":` + homepage_test_account + `}`
const homepage_test_connection = "xdt_api__v1__feed__user_timeline_graphql_connection"
const homepage_test_query = `{"data":{"` + homepage_test_connection + `":{"edges":[{"node":` + homepage_test_node + `},{"node":` + homepage_test_node + `}],"page_info":{"has_next_page":false,"end_cursor":null}}}}`

func TestHomeContentsGraphQL(t *testing.T) {
	for _, test_case := range []struct {
		name       string
		dtsg       string
		cookie_set []cookies.Cookie
	}{
		{name: "anonymous"},
		{name: "empty_dtsg", dtsg: `["DTSGInitialData",[],{}];`},
		{name: "expired_cookie", cookie_set: []cookies.Cookie{{Name: "sessionid", Value: "expired", Domain: ".instagram.com", Path: "/", Expires: 1}}},
		{name: "authenticated", dtsg: `["DTSGInitialData",[],{"token":"dtsg"}];`, cookie_set: []cookies.Cookie{
			{Name: "sessionid", Value: "session", Domain: ".instagram.com", Path: "/"},
			{Name: "csrftoken", Value: "csrf", Domain: ".instagram.com", Path: "/"},
		}},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			work_dir := t.TempDir()
			if test_case.cookie_set != nil {
				if err := cookies.SaveJSON(test_case.cookie_set, filepath.Join(work_dir, "cookies.json")); err != nil {
					t.Fatal(err)
				}
			}
			client, err := NewClient(cookies.NewPersistentReader(work_dir))
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			calls := 0
			client.http_client.Transport = homepage_test_transport(func(request *http.Request) (*http.Response, error) {
				calls++
				if (request.Header.Get("Cookie") != "") != (test_case.name == "authenticated") {
					t.Fatal("unexpected request credentials")
				}
				body := `<script type="application/json">` + homepage_test_account + `</script>` + test_case.dtsg + `[ "LSD", [], {"token":"anonymous-lsd"} ]`
				if calls == 1 {
					if request.Method != http.MethodGet || request.URL.String() != "https://www.instagram.com/example/" {
						t.Fatalf("unexpected profile request: %s %s", request.Method, request.URL)
					}
				} else {
					if request.Method != http.MethodPost || request.URL.String() != "https://www.instagram.com/graphql/query" {
						t.Fatalf("unexpected query: %s %s", request.Method, request.URL)
					}
					if err := request.ParseForm(); err != nil {
						t.Fatal(err)
					}
					if request.Header.Get("X-FB-LSD") != request.Form.Get("lsd") ||
						request.Header.Get("Sec-Fetch-Site") != "same-origin" || request.Header.Get("Sec-Fetch-Mode") != "cors" || request.Header.Get("Sec-Fetch-Dest") != "empty" {
						t.Fatal("homepage query is missing page token or browser request context")
					}
					if request.Form.Get("lsd") != "anonymous-lsd" || request.Form.Get("doc_id") != "30543471925243353" ||
						gjson.Get(request.Form.Get("variables"), "username").String() != "example" ||
						gjson.Get(request.Form.Get("variables"), "data.count").Int() != 12 {
						t.Fatalf("invalid query form: %v", request.Form)
					}
					if test_case.name == "authenticated" {
						if request.Header.Get("X-CSRFToken") != "csrf" || request.Form.Get("fb_dtsg") != "dtsg" || request.Form.Get("jazoest") != "2434" {
							t.Fatal("authenticated query lost its tokens")
						}
					} else if request.Form.Has("fb_dtsg") || request.Form.Get("jazoest") != "21369" || request.Header.Get("X-CSRFToken") != "" {
						t.Fatal("anonymous query must not require authentication tokens")
					}
					body = "for (;;);" + homepage_test_query
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
			})
			result, err := client.FetchHomeContentsContext(nil, "@Example")
			if err != nil {
				t.Fatal(err)
			}
			if calls != 2 || result.Account.Username != "example" || len(result.Items) != 1 || result.Items[0].ExternalID != "9007199254740993" ||
				result.Items[0].CoverURL != "https://cdninstagram.com/cover.jpg" || result.Items[0].SourceURL != "https://www.instagram.com/p/Post_1/" {
				t.Fatalf("incorrect homepage result: %+v", result)
			}
		})
	}
}

func TestHomeContentsEmbeddedGrid(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	calls := 0
	client.http_client.Transport = homepage_test_transport(func(request *http.Request) (*http.Response, error) {
		calls++
		body := `<script type="application/json">[` + homepage_test_account + `,{"pk":"123","polaris_ordered_timeline_connection":{"edges":[{"node":` + homepage_test_node + `}],"page_info":{"has_next_page":true,"end_cursor":"opaque+/cursor=="}}}]</script>`
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	})
	result, err := client.FetchHomeContents("example")
	if err != nil || calls != 1 || len(result.Items) != 1 || result.NextMarker != "opaque+/cursor==" {
		t.Fatalf("embedded grid failed: result=%+v calls=%d err=%v", result, calls, err)
	}
}

func TestHomeContentsQueryFailures(t *testing.T) {
	for _, test_case := range []struct {
		name   string
		page   string
		body   string
		status int
		want   string
		calls  int
	}{
		{name: "missing_lsd", page: "<html>Login</html>", want: "embedded page data", calls: 1},
		{name: "http_rate_limit", status: 429, want: "HTTP 429", calls: 2},
		{name: "graphql_rate_limit", body: `{"errors":[{"code":1675004}]}`, want: "rate limited", calls: 2},
		{name: "login_page", body: "<html>Login</html>", want: "invalid JSON", calls: 2},
		{name: "graphql_error", body: `{"errors":[{"message":"Login required"}]}`, want: "query failed", calls: 2},
		{name: "upstream_error", body: `{"__ar":1,"error":1357054,"payload":null}`, want: "1357054", calls: 2},
		{name: "missing_edges", body: `{"data":{}}`, want: "posts are missing", calls: 2},
		{name: "invalid_item", body: strings.ReplaceAll(homepage_test_query, `"media_type":2`, `"media_type":99`), want: "invalid homepage post", calls: 2},
		{name: "wrong_account", body: strings.ReplaceAll(homepage_test_query, `"username":"example"`, `"username":"other"`), want: "account data", calls: 2},
		{name: "missing_page_info", body: strings.ReplaceAll(homepage_test_query, `"has_next_page":false`, `"unknown":false`), want: "pagination info", calls: 2},
		{name: "invalid_has_next_page", body: strings.ReplaceAll(homepage_test_query, `"has_next_page":false`, `"has_next_page":"false"`), want: "pagination info", calls: 2},
		{name: "missing_cursor", body: strings.ReplaceAll(homepage_test_query, `"has_next_page":false`, `"has_next_page":true`), want: "next page cursor", calls: 2},
		{name: "numeric_cursor", body: strings.ReplaceAll(homepage_test_query, `"has_next_page":false,"end_cursor":null`, `"has_next_page":true,"end_cursor":123`), want: "next page cursor", calls: 2},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			client, err := NewClient()
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			calls := 0
			client.http_client.Transport = homepage_test_transport(func(request *http.Request) (*http.Response, error) {
				calls++
				status := http.StatusOK
				body := test_case.page
				if calls == 1 && body == "" {
					body = `["LSD",[],{"token":"lsd"}]`
				}
				if calls == 2 {
					body = test_case.body
					if test_case.status != 0 {
						status = test_case.status
					}
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
			})
			_, err = client.FetchHomeContentsContext(context.Background(), "example")
			if err == nil || !strings.Contains(err.Error(), test_case.want) || calls != test_case.calls {
				t.Fatalf("calls=%d err=%v, want %d calls and %q", calls, err, test_case.calls, test_case.want)
			}
		})
	}
}

func TestHomeContentsPagination(t *testing.T) {
	for _, repeated_cursor := range []bool{false, true} {
		t.Run(fmt.Sprint("repeated_cursor=", repeated_cursor), func(t *testing.T) {
			client, err := NewClient()
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			const cursor = "opaque+/cursor=="
			calls := 0
			client.http_client.Transport = homepage_test_transport(func(request *http.Request) (*http.Response, error) {
				calls++
				// A grid in the bootstrap page must not replace the requested cursor page.
				body := `<script type="application/json">[` + homepage_test_account + `,{"pk":"123","id":"relay_123","polaris_ordered_timeline_connection":{"edges":[],"page_info":{"has_next_page":false}}},{"csrf_token":"anonymous-csrf"}]</script>["LSD",[],{"token":"lsd"}]`
				if request.Method == http.MethodPost {
					if err := request.ParseForm(); err != nil {
						t.Fatal(err)
					}
					variables := gjson.Parse(request.Form.Get("variables"))
					const query_name = "PolarisLoggedOutDesktopWWWProfilePostsTabContentQuery_connection"
					if request.Form.Get("fb_api_req_friendly_name") != query_name || request.Header.Get("X-FB-Friendly-Name") != query_name ||
						request.URL.String() != "https://www.instagram.com/api/graphql" || request.Form.Get("doc_id") != "28904005159232689" || variables.Get("after").String() != cursor ||
						variables.Get("first").Int() != 12 || variables.Get("id").String() != "relay_123" || len(variables.Map()) != 4 || variables.Get("__relay_internal__pv__PolarisSEOIsCrawlerrelayprovider").Bool() ||
						request.Header.Get("X-CSRFToken") != "anonymous-csrf" || request.Header.Get("Cookie") != "" || request.Form.Get("jazoest") != "2323" || request.Form.Has("fb_dtsg") {
						t.Fatalf("invalid pagination query: %v", request.Form)
					}
					body = `{"data":{"node":{"id":"relay_123","polaris_ordered_timeline_connection":{"edges":[{"node":` + homepage_test_node + `}],"page_info":{"has_next_page":false,"end_cursor":null}}}}}`
					if repeated_cursor {
						body = strings.ReplaceAll(body, `"has_next_page":false,"end_cursor":null`, `"has_next_page":true,"end_cursor":"`+cursor+`"`)
					}
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
			})
			result, err := client.FetchHomeContentsPage("example", cursor)
			if repeated_cursor {
				if err == nil || !strings.Contains(err.Error(), "repeated pagination cursor") {
					t.Fatalf("want repeated cursor error, got %v", err)
				}
			} else if err != nil || len(result.Items) != 1 || result.NextMarker != "" {
				t.Fatalf("invalid final page: result=%+v err=%v", result, err)
			}
			if calls != 2 {
				t.Fatalf("expected profile and cursor query, got %d requests", calls)
			}
		})
	}
}

func TestHomeContentsPaginationAccountIdentity(t *testing.T) {
	for _, test_case := range []struct {
		name        string
		page_id     string
		response_id string
		want_error  string
		want_calls  int
	}{
		{name: "missing Relay ID", response_id: "relay_123", want_error: "Relay account ID is missing", want_calls: 1},
		{name: "response belongs to another account", page_id: "relay_123", response_id: "another_account", want_error: "account ID does not match", want_calls: 2},
		{name: "response omits account ID", page_id: "relay_123", want_error: "account ID does not match", want_calls: 2},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			client, err := NewClient()
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			calls := 0
			client.http_client.Transport = homepage_test_transport(func(request *http.Request) (*http.Response, error) {
				calls++
				body := `<script type="application/json">[` + homepage_test_account + `,{"pk":"123","id":"` + test_case.page_id + `"}]</script>["LSD",[],{"token":"lsd"}]`
				if request.Method == http.MethodPost {
					body = `{"data":{"node":{"id":"` + test_case.response_id + `","polaris_ordered_timeline_connection":{"edges":[],"page_info":{"has_next_page":false}}}}}`
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
			})
			_, err = client.FetchHomeContentsPage("example", "cursor")
			if err == nil || !strings.Contains(err.Error(), test_case.want_error) || calls != test_case.want_calls {
				t.Fatalf("calls=%d, err=%v; want %d calls and %q", calls, err, test_case.want_calls, test_case.want_error)
			}
		})
	}
}

func TestHomeContentsPaginationLoggedInBootstrap(t *testing.T) {
	for _, account_in_shell := range []bool{false, true} {
		t.Run(fmt.Sprint("account_in_shell=", account_in_shell), func(t *testing.T) {
			work_dir := t.TempDir()
			if err := cookies.SaveJSON([]cookies.Cookie{{Name: "sessionid", Value: "session", Domain: ".instagram.com", Path: "/"}}, filepath.Join(work_dir, "cookies.json")); err != nil {
				t.Fatal(err)
			}
			previous_transport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = previous_transport })
			calls := 0
			http.DefaultTransport = homepage_test_transport(func(request *http.Request) (*http.Response, error) {
				calls++
				if (request.Header.Get("Cookie") != "") != (calls == 1) {
					t.Fatal("anonymous fallback must not send persisted login cookies")
				}
				body := `<script type="application/json">[` + homepage_test_account + `,{"pk":"123","id":"relay_123"}]</script>["LSD",[],{"token":"lsd"}]`
				if calls == 1 {
					body = `["LSD",[],{"token":"lsd"}]`
					if account_in_shell {
						body += `<script type="application/json">` + homepage_test_account + `</script>`
					}
				} else if calls == 3 {
					if request.Method != http.MethodPost || request.URL.Path != "/api/graphql" {
						t.Fatalf("unexpected pagination request %s %s", request.Method, request.URL)
					}
					if err := request.ParseForm(); err != nil {
						t.Fatal(err)
					}
					variables := gjson.Parse(request.Form.Get("variables"))
					if variables.Get("id").String() != "relay_123" || variables.Get("after").String() != "original_cursor" {
						t.Fatalf("anonymous fallback changed the account or cursor: %s", variables.Raw)
					}
					body = `{"data":{"node":{"id":"relay_123","polaris_ordered_timeline_connection":{"edges":[],"page_info":{"has_next_page":false}}}}}`
				} else if calls > 3 {
					t.Fatal("anonymous fallback must run at most once")
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
			})
			client, err := NewClient(cookies.NewPersistentReader(work_dir))
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			result, err := client.FetchHomeContentsPage("example", "original_cursor")
			if err != nil || result.Account.ExternalID != "123" || result.NextMarker != "" || calls != 3 {
				t.Fatalf("result=%+v, calls=%d, err=%v", result, calls, err)
			}
		})
	}
}

func TestHomeContentsLoggedInShellStartsAnonymousTraversal(t *testing.T) {
	work_dir := t.TempDir()
	if err := cookies.SaveJSON([]cookies.Cookie{{Name: "sessionid", Value: "session", Domain: ".instagram.com", Path: "/"}}, filepath.Join(work_dir, "cookies.json")); err != nil {
		t.Fatal(err)
	}
	previous_transport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous_transport })
	calls := 0
	http.DefaultTransport = homepage_test_transport(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.Method != http.MethodGet || (request.Header.Get("Cookie") != "") != (calls == 1) {
			t.Fatalf("unexpected first-page bootstrap: method=%s call=%d", request.Method, calls)
		}
		body := `["LSD",[],{"token":"lsd"}]`
		if calls == 2 {
			body += `<script type="application/json">[` + homepage_test_account + `,{"pk":"123","id":"relay_123","polaris_ordered_timeline_connection":{"edges":[{"node":` + homepage_test_node + `}],"page_info":{"has_next_page":true,"end_cursor":"anonymous_cursor"}}}]</script>`
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	})
	client, err := NewClient(cookies.NewPersistentReader(work_dir))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	result, err := client.FetchHomeContents("example")
	if err != nil || calls != 2 || result.NextMarker != "anonymous_cursor" || len(result.Items) != 1 {
		t.Fatalf("result=%+v, calls=%d, err=%v", result, calls, err)
	}
}
