package api

import (
	"testing"

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
