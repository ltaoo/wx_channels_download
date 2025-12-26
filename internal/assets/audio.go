package assets

import (
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

//go:embed done.mp3
var doneMp3 []byte

var (
	lastPlayed time.Time
	mu         sync.Mutex
)

// PlayDoneAudio plays the done.mp3 sound effect.
// It throttles playback to at most once per second.
func PlayDoneAudio() error {
	mu.Lock()
	if time.Since(lastPlayed) < time.Second {
		mu.Unlock()
		return nil
	}
	lastPlayed = time.Now()
	mu.Unlock()

	// Only support macOS for now
	if runtime.GOOS != "darwin" {
		return nil
	}

	tmpFile := filepath.Join(os.TempDir(), "wx_channels_download_done.mp3")
	// Always write to ensure the file exists and is up to date
	if err := os.WriteFile(tmpFile, doneMp3, 0644); err != nil {
		return err
	}

	// Play in background to avoid blocking
	go func() {
		cmd := exec.Command("afplay", tmpFile)
		_ = cmd.Run()
	}()

	return nil
}
