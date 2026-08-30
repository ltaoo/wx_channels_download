package minib

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"wx_channel/pkg/clawreq"
)

func TestHARBodyBudgetPreservesSizesAndMarksTruncation(t *testing.T) {
	recorder := new_har_recorder(time.Now(), false, 5)
	response := &clawreq.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
		Body:       []byte("abcdefghij"),
	}
	recorder.record_network(context.Background(), time.Now(), time.Millisecond, http.MethodPost, "https://example.test/api", http.Header{"Content-Type": []string{"text/plain"}}, []byte("12"), response, nil)
	har_data, err := recorder.marshal("budget")
	if err != nil {
		t.Fatal(err)
	}
	var archive har_archive
	if err := json.Unmarshal(har_data, &archive); err != nil {
		t.Fatal(err)
	}
	if len(archive.Log.Entries) != 1 {
		t.Fatalf("HAR entries = %d, want 1", len(archive.Log.Entries))
	}
	entry := archive.Log.Entries[0]
	if entry.Request.BodySize != 2 || entry.Request.PostData == nil || entry.Request.PostData.Text != "12" || entry.Request.PostData.Truncated {
		t.Fatalf("request body metadata = %+v", entry.Request)
	}
	if entry.Response.BodySize != 10 || entry.Response.Content.Size != 10 || entry.Response.Content.Text != "abc" || !entry.Response.Content.Truncated {
		t.Fatalf("response body metadata = %+v", entry.Response)
	}
}

func TestHARCanCaptureMetadataWithoutBodies(t *testing.T) {
	recorder := new_har_recorder(time.Now(), true, 0)
	response := &clawreq.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte(`{"ok":true}`),
	}
	recorder.record_network(context.Background(), time.Now(), time.Millisecond, http.MethodPost, "https://example.test/api", http.Header{"Content-Type": []string{"application/json"}}, []byte(`{"request":true}`), response, nil)
	har_data, err := recorder.marshal("metadata")
	if err != nil {
		t.Fatal(err)
	}
	var archive har_archive
	if err := json.Unmarshal(har_data, &archive); err != nil {
		t.Fatal(err)
	}
	entry := archive.Log.Entries[0]
	if entry.Request.PostData == nil || entry.Request.PostData.Text != "" || !entry.Request.PostData.Truncated || entry.Request.BodySize == 0 {
		t.Fatalf("request body was retained or lost metadata: %+v", entry.Request)
	}
	if entry.Response.Content.Text != "" || !entry.Response.Content.Truncated || entry.Response.Content.Size == 0 {
		t.Fatalf("response body was retained or lost metadata: %+v", entry.Response.Content)
	}
}
