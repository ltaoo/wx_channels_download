package quark

import (
	"context"
	"testing"
	"time"
)

func TestClientFetchFileList(t *testing.T) {
	share_url := "https://pan.quark.cn/s/1080a7210964#/list/share"

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client := NewClient()
	share, err := client.FetchContext(ctx, share_url)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	t.Logf("Title: %s", share.Title)
	t.Logf("Author: %s", share.Author)
	t.Logf("PwdID: %s", share.PwdID)
	t.Logf("Stoken: %s", share.Stoken)
	t.Logf("FileCount: %d", share.FileCount)
	t.Logf("TotalSize: %d", share.TotalSize)

	if share.FileCount == 0 {
		t.Fatal("expected non-zero file count")
	}

	flat := share.FlattenFiles()
	t.Logf("FlattenFiles count: %d", len(flat))

	dirs := 0
	files := 0
	for _, f := range flat {
		if f.IsDir {
			dirs++
		} else {
			files++
		}
	}
	t.Logf("Directories: %d, Files: %d", dirs, files)

	// Show first 10 entries
	for i, f := range flat {
		if i >= 10 {
			t.Logf("... and %d more entries", len(flat)-10)
			break
		}
		if f.IsDir {
			t.Logf("  [%d] 📁 %s", i, f.Path)
		} else {
			t.Logf("  [%d] 📄 %s (%d bytes)", i, f.Path, f.Size)
		}
	}
}

func TestSetCookieHeader(t *testing.T) {
	client := NewClient()
	client.SetCookieHeader("__pus=1; __puus=2=value")

	if got := client.transient_cookie_header(); got != "__pus=1; __puus=2=value" {
		t.Fatalf("unexpected cookie header: %s", got)
	}
}

func TestBuildTreePropagatesDirectoryStoken(t *testing.T) {
	client := &Client{}
	share := &Share{PwdID: "share"}
	entry_count := 0
	files, err := client.build_tree(
		context.Background(),
		"share",
		"0",
		"",
		"directory-token",
		[]api_file{{Fid: "file", FileName: "a.mp4", File: true}},
		0,
		make(map[string]bool),
		&entry_count,
		share,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Stoken != "directory-token" {
		t.Fatalf("unexpected files: %#v", files)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
