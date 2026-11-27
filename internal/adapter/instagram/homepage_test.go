package instagramadapter

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/tidwall/gjson"

	"wx_channel/internal/database/model"
)

type instagram_home_test_transport func(*http.Request) (*http.Response, error)

func (transport instagram_home_test_transport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestInstagramHomepagePagination(t *testing.T) {
	previous_transport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous_transport })
	const account_record = `{"pk":"123","username":"example","full_name":"Example"}`
	const post = `{"pk":"9007199254740993","code":"Post_1","media_type":8}`
	const cursor = "opaque+/cursor=="
	queries := 0
	http.DefaultTransport = instagram_home_test_transport(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "www.instagram.com" {
			t.Fatalf("unexpected host %s", request.URL.Host)
		}
		body := `<script type="application/json">[` + account_record + `,{"pk":"123","id":"relay_123","polaris_ordered_timeline_connection":{"edges":[{"node":` + post + `}],"page_info":{"has_next_page":true,"end_cursor":"` + cursor + `"}}}]</script>["LSD",[],{"token":"lsd"}]`
		if request.Method == http.MethodPost {
			queries++
			if err := request.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if gjson.Get(request.Form.Get("variables"), "after").String() != cursor {
				t.Fatal("adapter did not forward the cursor")
			}
			body = `{"data":{"node":{"id":"relay_123","polaris_ordered_timeline_connection":{"edges":[],"page_info":{"has_next_page":false}}}}}`
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	})
	account := &model.Account{PlatformId: PlatformID, ProfileURL: "https://www.instagram.com/example/"}
	adapter := &handler{}
	initial, err := adapter.BuildHomeContents(account, "posts")
	if err != nil {
		t.Fatal(err)
	}
	if initial.NextMarker != cursor || len(initial.Contents) != 1 || initial.Contents[0].Id != "instagram:9007199254740993" || initial.Contents[0].Type != model.ContentTypeAlbum {
		t.Fatalf("incorrect first page: %+v", initial)
	}
	final_page, err := adapter.FetchHomeDetails(account, "feed", initial.NextMarker)
	if err != nil {
		t.Fatal(err)
	}
	contents, ok := final_page.Contents.([]model.Content)
	if !ok || len(contents) != 0 || final_page.NextMarker != "" || final_page.Scope != "posts" || queries != 1 {
		t.Fatalf("incorrect cursor page: %+v, queries=%d", final_page, queries)
	}
	if _, err := adapter.FetchHomeDetails(account, "stories", ""); err == nil {
		t.Fatal("unsupported scope should fail")
	}
	if _, err := adapter.FetchHomeDetails(nil, "posts", cursor); err == nil {
		t.Fatal("missing account should fail")
	}
}
