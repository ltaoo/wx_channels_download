package frontend

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestWxChannelsStreamWorkerAssets(t *testing.T) {
	working_dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(".."); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(working_dir)
	}()
	for _, mode := range []string{"dev", "prod"} {
		handler := NewServer(mode)
		for _, testcase := range []struct {
			path string
			want string
		}{
			{
				path: "/wxchannels.stream.js",
				want: "src/pages/wxchannels.crypto.js",
			},
			{
				path: "/src/pages/wxchannels.crypto.js",
				want: "create_wxchannels_decryptor",
			},
		} {
			request := httptest.NewRequest(http.MethodGet, testcase.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("%s %s status = %d, want %d", mode, testcase.path, response.Code, http.StatusOK)
			}
			if !strings.Contains(response.Header().Get("Content-Type"), "javascript") {
				t.Fatalf("%s %s content type = %q, want JavaScript", mode, testcase.path, response.Header().Get("Content-Type"))
			}
			if !strings.Contains(response.Body.String(), testcase.want) {
				t.Fatalf("%s %s does not contain %q", mode, testcase.path, testcase.want)
			}
		}
	}
}
