package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResourcesAdvertisedAndReadableOverHTTP(t *testing.T) {
	server := NewServer(ServerOptions{})
	server.AddResource(Resource{URI: "dm://skills/test", Name: "test", MimeType: "text/markdown", Text: "skill instructions"})
	handler := NewHTTPHandler(server)
	call := func(method string, params string) map[string]json.RawMessage {
		t.Helper()
		request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"`+method+`","params":`+params+`}`))
		request.Header.Set("Content-Type", "application/json")
		writer := httptest.NewRecorder()
		handler.ServeHTTP(writer, request)
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(writer.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		return envelope
	}
	initialized := call("initialize", `{"protocolVersion":"2025-11-25"}`)
	if !strings.Contains(string(initialized["result"]), `"resources":{}`) {
		t.Fatalf("capability missing: %s", initialized["result"])
	}
	catalog := call("resources/list", `{}`)
	if !strings.Contains(string(catalog["result"]), "dm://skills/test") || strings.Contains(string(catalog["result"]), "skill instructions") {
		t.Fatalf("resource catalog: %s", catalog["result"])
	}
	read := call("resources/read", `{"uri":"dm://skills/test"}`)
	if !strings.Contains(string(read["result"]), "skill instructions") {
		t.Fatalf("resource body missing: %s", read["result"])
	}
	missing := call("resources/read", `{"uri":"file:///unregistered"}`)
	if missing["error"] == nil {
		t.Fatal("unregistered resources must not be read")
	}
}
