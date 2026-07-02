package api

import (
	"testing"

	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"

	"wx_channel/pkg/util"
)

func TestProcessTaskFilenameAllowsSameBaseWithDifferentSuffix(t *testing.T) {
	c := &APIClient{
		formatter: util.NewFilenameProcessor("", make(map[string]int)),
	}

	name, dir, err := c.processTaskFilename("clip", ".mp4")
	if err != nil {
		t.Fatalf("process mp4 filename: %v", err)
	}
	if name != "clip.mp4" {
		t.Fatalf("first mp4 name = %q, want %q", name, "clip.mp4")
	}
	if dir != "" {
		t.Fatalf("first mp4 dir = %q, want empty", dir)
	}

	name, dir, err = c.processTaskFilename("clip", ".jpg")
	if err != nil {
		t.Fatalf("process jpg filename: %v", err)
	}
	if name != "clip.jpg" {
		t.Fatalf("jpg name = %q, want %q", name, "clip.jpg")
	}
	if dir != "" {
		t.Fatalf("jpg dir = %q, want empty", dir)
	}

	name, _, err = c.processTaskFilename("clip", ".mp4")
	if err != nil {
		t.Fatalf("process duplicate mp4 filename: %v", err)
	}
	if name != "clip(1).mp4" {
		t.Fatalf("duplicate mp4 name = %q, want %q", name, "clip(1).mp4")
	}
}

func TestCountDownloadTaskStatusesNormalizesFailureAliases(t *testing.T) {
	counts := countDownloadTaskStatuses([]*downloadpkg.Task{
		{Status: base.DownloadStatusError},
		{Status: base.Status("failed")},
		{Status: base.Status("fail")},
		{Status: base.Status("failure")},
		{Status: base.Status("errored")},
		{Status: base.Status("paused")},
		{Status: base.Status("pending")},
		{Status: base.Status("completed")},
	})

	if counts["total"] != 8 {
		t.Fatalf("total = %d, want 8", counts["total"])
	}
	if counts["error"] != 5 {
		t.Fatalf("error = %d, want 5", counts["error"])
	}
	if counts["pause"] != 1 {
		t.Fatalf("pause = %d, want 1", counts["pause"])
	}
	if counts["wait"] != 1 {
		t.Fatalf("wait = %d, want 1", counts["wait"])
	}
	if counts["done"] != 1 {
		t.Fatalf("done = %d, want 1", counts["done"])
	}
}
