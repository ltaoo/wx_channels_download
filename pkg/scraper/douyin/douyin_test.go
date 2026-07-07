package douyin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	got := SanitizeFilename(` a/b:c*d?"e<f>g|h#i
.. `)
	want := "a_b_c_d__e_f_g_h_i_"
	if got != want {
		t.Fatalf("SanitizeFilename() = %q, want %q", got, want)
	}
}

func TestParseVideoID(t *testing.T) {
	tests := map[string]string{
		"https://www.iesdouyin.com/share/video/7123456789012345678/?region=CN": "7123456789012345678",
		"https://www.douyin.com/video/7123456789012345678":                     "7123456789012345678",
		"https://v.douyin.com/abcdef/":                                         "abcdef",
	}
	for rawURL, want := range tests {
		if got := parseVideoID(rawURL); got != want {
			t.Fatalf("parseVideoID(%q) = %q, want %q", rawURL, got, want)
		}
	}
}

func TestExtractShareURL(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "simple share text",
			text: "1.23 复制打开抖音 https://v.douyin.com/abcdef/ 看视频",
			want: "https://v.douyin.com/abcdef/",
		},
		{
			name: "douyin full share copy",
			text: "8.28 复制打开抖音，看看【小埋姐姐的作品】最难解的题其实是你# 温柔风穿搭 # 大理  https://v.douyin.com/s4lgym1_6Gg/ 12/25 xsR:/ S@L.WZ :3pm",
			want: "https://v.douyin.com/s4lgym1_6Gg/",
		},
		{
			name: "trailing chinese punctuation",
			text: "复制打开抖音 https://v.douyin.com/abcdef/.",
			want: "https://v.douyin.com/abcdef/",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractShareURL(tt.text); got != tt.want {
				t.Fatalf("ExtractShareURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractRouterDataAndItem(t *testing.T) {
	page := `<html><script>window._ROUTER_DATA = {"loaderData":{"video_(id)/page":{"videoInfoRes":{"item_list":[{"desc":"hello/world","author":{"uid":"uid1","unique_id":"author1","nickname":"作者","avatar_thumb":{"url_list":["https://example.com/avatar.jpg"]}},"video":{"play_addr":{"url_list":["https://example.com/playwm.mp4"]},"cover":{"url_list":["https://example.com/cover.jpg"]}}}]}}}}</script></html>`

	data, err := extractRouterData(page)
	if err != nil {
		t.Fatalf("extractRouterData() error = %v", err)
	}
	item, err := extractItem(data)
	if err != nil {
		t.Fatalf("extractItem() error = %v", err)
	}
	videoURL, err := extractString(item, "video", "play_addr", "url_list", "0")
	if err != nil {
		t.Fatalf("extractString(video URL) error = %v", err)
	}
	if videoURL != "https://example.com/playwm.mp4" {
		t.Fatalf("videoURL = %q", videoURL)
	}
	authorNickname, err := extractString(item, "author", "nickname")
	if err != nil {
		t.Fatalf("extractString(author nickname) error = %v", err)
	}
	if authorNickname != "作者" {
		t.Fatalf("authorNickname = %q", authorNickname)
	}
}

func TestDownload(t *testing.T) {
	content := strings.Repeat("abc123", 1024)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Fatal("missing User-Agent")
		}
		w.Header().Set("Content-Length", "6144")
		_, _ = w.Write([]byte(content))
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "video.mp4")
	var progressCalls int
	err := NewClient().Download(context.Background(), &VideoInfo{
		URL:       server.URL,
		UserAgent: "test-agent",
	}, dest, func(progress Progress) {
		progressCalls++
		if progress.DownloadedBytes < 0 {
			t.Fatalf("invalid progress: %+v", progress)
		}
	})
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != content {
		t.Fatal("downloaded content mismatch")
	}
	if progressCalls == 0 {
		t.Fatal("expected progress callback")
	}
}
