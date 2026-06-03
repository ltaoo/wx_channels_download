package api

import (
	"path/filepath"
	"testing"
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
