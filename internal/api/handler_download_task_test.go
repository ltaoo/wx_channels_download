package api

import (
	"path/filepath"
	"testing"

	"wx_channel/internal/database/model"
)

func TestDownloadTaskFullPath(t *testing.T) {
	downloadDir := filepath.Join(string(filepath.Separator), "tmp", "downloads")
	absPath := filepath.Join(string(filepath.Separator), "tmp", "downloads", "video.mp4")

	tests := []struct {
		name        string
		downloadDir string
		filePath    string
		want        string
	}{
		{
			name:        "relative file path",
			downloadDir: downloadDir,
			filePath:    "video.mp4",
			want:        filepath.Join(downloadDir, "video.mp4"),
		},
		{
			name:        "relative nested path",
			downloadDir: downloadDir,
			filePath:    filepath.Join("channels", "video.mp4"),
			want:        filepath.Join(downloadDir, "channels", "video.mp4"),
		},
		{
			name:        "absolute path",
			downloadDir: downloadDir,
			filePath:    absPath,
			want:        filepath.Clean(absPath),
		},
		{
			name:        "empty path",
			downloadDir: downloadDir,
			filePath:    " ",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := downloadTaskFullPath(tt.downloadDir, tt.filePath); got != tt.want {
				t.Fatalf("downloadTaskFullPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompatDownloadTaskUsesPlatformRetry(t *testing.T) {
	tests := []struct {
		name string
		rec  model.DownloadTask
		want bool
	}{
		{
			name: "platform reason",
			rec:  model.DownloadTask{Reason: "platform", Protocol: "http", URL: "https://example.com/video.mp4"},
			want: true,
		},
		{
			name: "legacy platform metadata source",
			rec:  model.DownloadTask{Protocol: "http", URL: "https://example.com/video.mp4", Metadata2: `{"source":{"platform":"youtube"}}`},
			want: true,
		},
		{
			name: "legacy workflow metadata",
			rec:  model.DownloadTask{Protocol: "http", URL: "https://example.com/video.mp4", Metadata2: `{"metadata":{"workflow_run_id":"run_123"}}`},
			want: true,
		},
		{
			name: "plain https direct task",
			rec:  model.DownloadTask{Protocol: "https", URL: "https://example.com/video.mp4"},
			want: false,
		},
		{
			name: "missing protocol with https url",
			rec:  model.DownloadTask{URL: "https://example.com/video.mp4"},
			want: false,
		},
		{
			name: "custom platform protocol",
			rec:  model.DownloadTask{Protocol: "inline_json", URL: "inline-json://content/1"},
			want: true,
		},
		{
			name: "unknown scheme without slashes",
			rec:  model.DownloadTask{URL: "blob:https://example.com/asset-id"},
			want: true,
		},
		{
			name: "unsupported data url",
			rec:  model.DownloadTask{URL: "data:application/json;base64,e30="},
			want: true,
		},
		{
			name: "gopeed official account protocol remains direct",
			rec:  model.DownloadTask{Protocol: "officialaccount", URL: "officialaccount://https://mp.weixin.qq.com/s/example"},
			want: false,
		},
		{
			name: "gopeed stream protocol remains direct",
			rec:  model.DownloadTask{URL: "rtmp://example.com/live"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compatDownloadTaskUsesPlatformRetry(tt.rec); got != tt.want {
				t.Fatalf("compatDownloadTaskUsesPlatformRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompatPlatformRetryBodyUsesLegacyURLFields(t *testing.T) {
	tests := []struct {
		name      string
		metadata2 string
		wantURL   string
	}{
		{
			name:      "top level url",
			metadata2: `{"url":"https://channels.weixin.qq.com/web/pages/feed?oid=abc&nid=def"}`,
			wantURL:   "https://channels.weixin.qq.com/web/pages/feed?oid=abc&nid=def",
		},
		{
			name:      "top level content url",
			metadata2: `{"content_url":"https://channels.weixin.qq.com/web/pages/feed?oid=abc&nid=def"}`,
			wantURL:   "https://channels.weixin.qq.com/web/pages/feed?oid=abc&nid=def",
		},
		{
			name:      "nested metadata url",
			metadata2: `{"metadata":{"url":"https://channels.weixin.qq.com/web/pages/feed?oid=abc&nid=def"}}`,
			wantURL:   "https://channels.weixin.qq.com/web/pages/feed?oid=abc&nid=def",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := compatPlatformRetryBody(model.DownloadTask{Metadata2: tt.metadata2})
			if err != nil {
				t.Fatalf("compatPlatformRetryBody() error = %v", err)
			}
			if body.URL != tt.wantURL {
				t.Fatalf("compatPlatformRetryBody().URL = %q, want %q", body.URL, tt.wantURL)
			}
		})
	}
}

func TestCompatDownloadTaskShouldRestartPlatformOnRetryChildren(t *testing.T) {
	tests := []struct {
		name string
		rec  model.DownloadTask
		want bool
	}{
		{
			name: "pending cdp child restarts parent platform task",
			rec:  model.DownloadTask{ParentId: 1, NodeType: "file", Engine: "cdp", Status: 1},
			want: true,
		},
		{
			name: "completed cdp child is skipped",
			rec:  model.DownloadTask{ParentId: 1, NodeType: "file", Engine: "cdp", Status: 4},
			want: false,
		},
		{
			name: "failed platform container restarts",
			rec:  model.DownloadTask{Id: 1, NodeType: "container", Status: 5},
			want: true,
		},
		{
			name: "plain running http task is skipped",
			rec:  model.DownloadTask{Protocol: "http", URL: "https://example.com/file.mp4", Status: 1},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compatDownloadTaskShouldRestartPlatformOnRetryChildren(tt.rec); got != tt.want {
				t.Fatalf("compatDownloadTaskShouldRestartPlatformOnRetryChildren() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompatDownloadTaskPlatformRetryPathsByParentSkipsCompletedChildren(t *testing.T) {
	root := model.DownloadTask{Id: 10, NodeType: "container", Status: 5}
	records := []model.DownloadTask{
		root,
		{
			Id:        11,
			ParentId:  10,
			NodeType:  "file",
			Engine:    "cdp",
			Status:    4,
			Metadata2: `{"tree_path":"chapters/done.html"}`,
		},
		{
			Id:        12,
			ParentId:  10,
			NodeType:  "file",
			Engine:    "cdp",
			Status:    5,
			Metadata2: `{"tree_path":"chapters/failed.html"}`,
		},
		{
			Id:        13,
			ParentId:  10,
			NodeType:  "file",
			Engine:    "cdp",
			Status:    1,
			Metadata2: `{"tree_path":"chapters/pending.html"}`,
		},
	}

	got := compatDownloadTaskPlatformRetryPathsByParent(root, records)
	paths := got[10]
	want := []string{"chapters/failed.html", "chapters/pending.html"}
	if len(paths) != len(want) {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("paths = %#v, want %#v", paths, want)
		}
	}
}
