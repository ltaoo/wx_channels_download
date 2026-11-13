package wxchannelsadapter

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"

	"wx_channel/internal/adapter"
	"wx_channel/pkg/hermes"
)

func TestStreamPostprocessPlaybackCleanup(t *testing.T) {
	for _, binary := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(binary); err != nil {
			t.Skipf("%s is required: %v", binary, err)
		}
	}
	fixture_path := filepath.Join(t.TempDir(), "source.mkv")
	cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=black:s=64x64:r=10",
		"-t", "0.2", "-c:v", "libx264", "-y", fixture_path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create recording fixture: %v: %s", err, output)
	}
	fixture, err := os.ReadFile(fixture_path)
	if err != nil {
		t.Fatal(err)
	}

	for _, test_case := range []struct {
		name             string
		input_name       string
		missing_playback bool
		corrupt_input    bool
		block_conversion bool
		block_commit     bool
	}{
		{name: "extensionless recording", input_name: "recording"},
		{name: "MKV recording", input_name: "recording.mkv"},
		{name: "MP4 input path", input_name: "recording.mp4"},
		{name: "no playback directory", input_name: "recording", missing_playback: true},
		{name: "probe failure preserves playback", input_name: "recording", corrupt_input: true},
		{name: "conversion failure preserves playback", input_name: "recording", block_conversion: true},
		{name: "commit failure preserves playback", input_name: "recording", block_commit: true},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			base_path := t.TempDir()
			input_file := filepath.Join(base_path, test_case.input_name)
			input := fixture
			if test_case.corrupt_input {
				input = []byte("invalid recording")
			}
			if err := os.WriteFile(input_file, input, 0644); err != nil {
				t.Fatal(err)
			}
			playback_dir := hermes.StreamPlaybackDir(input_file)
			segment_path := filepath.Join(playback_dir, "segment-000000000.ts")
			if !test_case.missing_playback {
				if err := os.MkdirAll(playback_dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(segment_path, []byte("playback segment"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if test_case.block_conversion || test_case.block_commit {
				blocked_path := input_file + ".mp4"
				if test_case.block_conversion {
					blocked_path += ".converting"
				}
				if err := os.MkdirAll(blocked_path, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(blocked_path, "keep"), []byte("keep"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			// Other recordings' playback directories must remain untouched.
			other_playback := hermes.StreamPlaybackDir(filepath.Join(base_path, "other"))
			if err := os.MkdirAll(other_playback, 0755); err != nil {
				t.Fatal(err)
			}
			task := &hermes.TaskJob{
				ID: 1,
				Resources: []hermes.ResourceJob{{
					ID: 1, Type: "STREAM", UniqueID: test_case.input_name,
					FilePath: input_file, Kind: "video/x-matroska",
				}},
			}
			err := NewChannelsAdapter().Postprocess(context.Background(), task, adapter.PostprocessDeps{
				BasePath: base_path, Logger: zerolog.Nop(),
			})
			if _, stat_err := os.Stat(other_playback); stat_err != nil {
				t.Fatalf("unrelated playback directory was removed: %v", stat_err)
			}
			if test_case.corrupt_input || test_case.block_conversion || test_case.block_commit {
				if err == nil {
					t.Fatal("expected conversion to fail")
				}
				if segment, read_err := os.ReadFile(segment_path); read_err != nil || string(segment) != "playback segment" {
					t.Fatalf("failed conversion did not preserve playback: %q, %v", segment, read_err)
				}
				if _, stat_err := os.Stat(input_file); stat_err != nil {
					t.Fatalf("failed conversion did not preserve recording: %v", stat_err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			resource := task.Resources[0]
			if resource.Kind != "video/mp4" || filepath.Ext(resource.FilePath) != ".mp4" || resource.Size <= 0 {
				t.Fatalf("unexpected converted resource: %+v", resource)
			}
			if codec, probe_err := probe_first_media_codec(context.Background(), resource.FilePath, "v:0"); probe_err != nil || codec != "h264" {
				t.Fatalf("converted MP4 is not playable: codec=%q, err=%v", codec, probe_err)
			}
			if _, stat_err := os.Stat(playback_dir); !os.IsNotExist(stat_err) {
				t.Fatalf("playback directory remains after conversion: %v", stat_err)
			}
		})
	}
}
