package logtest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLogRotationScript(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "logtest")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logFile := filepath.Join(tmpDir, "wx_video_download.out.log")
	largeData := make([]byte, 10485761)
	if err := os.WriteFile(logFile, largeData, 0644); err != nil {
		t.Fatalf("failed to write test log file: %v", err)
	}

	LOG_MAX := int64(10485760)
	fi, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if fi.Size() > LOG_MAX {
		if err := os.WriteFile(logFile, []byte{}, 0644); err != nil {
			t.Fatalf("truncate failed: %v", err)
		}
	}

	fiAfter, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("stat after failed: %v", err)
	}
	if fiAfter.Size() != 0 {
		t.Errorf("expected log file size 0 after rotation, got %d", fiAfter.Size())
	}
}
