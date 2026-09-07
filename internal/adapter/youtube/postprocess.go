package youtubeadapter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/pkg/hermes"
)

var youtube_ffmpeg_path = exec.LookPath

var run_youtube_ffmpeg = func(ctx context.Context, ffmpeg_path string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, ffmpeg_path, args...).CombinedOutput()
}

// Postprocess validates subtitles and merges separately downloaded YouTube video
// and audio tracks into H.264/AAC MP4 before Hermes finalizes filenames.
func (h *handler) Postprocess(ctx context.Context, info *hermes.TaskJob, deps adapter.PostprocessDeps) error {
	if info == nil {
		return fmt.Errorf("youtube postprocess: task is nil")
	}
	for _, resource := range info.Resources {
		if err := validate_youtube_subtitle(resource); err != nil {
			return fmt.Errorf("youtube postprocess: resource %s: %w", resource.Name, err)
		}
	}
	video_index, audio_index := youtube_media_resource_indexes(info.Resources)
	if video_index < 0 || audio_index < 0 {
		return nil
	}
	if err := context.Cause(ctx); err != nil {
		return err
	}

	video_resource := &info.Resources[video_index]
	audio_resource := &info.Resources[audio_index]
	video_path := strings.TrimSpace(video_resource.FilePath)
	audio_path := strings.TrimSpace(audio_resource.FilePath)
	if video_path == "" || audio_path == "" {
		return fmt.Errorf("youtube postprocess: task %d has missing media file path", info.ID)
	}
	if filepath.Clean(video_path) == filepath.Clean(audio_path) {
		return fmt.Errorf("youtube postprocess: video and audio resolve to the same file")
	}
	if err := require_youtube_regular_file(video_path, "video"); err != nil {
		return err
	}
	if err := require_youtube_regular_file(audio_path, "audio"); err != nil {
		return err
	}

	ffmpeg_path, err := youtube_ffmpeg_path("ffmpeg")
	if err != nil {
		return fmt.Errorf("youtube postprocess: ffmpeg is required: %w", err)
	}
	merged_path, err := youtube_merge_media(ctx, ffmpeg_path, video_path, audio_path)
	if err != nil {
		return err
	}
	defer os.Remove(merged_path)

	video_info, err := os.Stat(video_path)
	if err != nil {
		return fmt.Errorf("youtube postprocess: stat video before replacement: %w", err)
	}
	if err := replace_youtube_video(video_path, merged_path, video_info.Mode().Perm()); err != nil {
		return err
	}
	if err := os.Remove(audio_path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("youtube postprocess: remove merged audio file: %w", err)
	}
	merged_info, err := os.Stat(video_path)
	if err != nil {
		return fmt.Errorf("youtube postprocess: stat merged video: %w", err)
	}

	video_resource.Kind = "video/mp4"
	video_resource.Size = merged_info.Size()
	video_resource.Downloaded = merged_info.Size()
	if video_resource.Extra == nil {
		video_resource.Extra = make(map[string]string)
	}
	video_resource.Extra["postprocessed"] = "true"
	video_resource.Extra["audio_merged"] = "true"

	kept_resources := make([]hermes.ResourceJob, 0, len(info.Resources)-1)
	for resource_index := range info.Resources {
		if resource_index != audio_index {
			kept_resources = append(kept_resources, info.Resources[resource_index])
		}
	}
	info.Resources = kept_resources

	deps.Logger.Info().
		Int("task_id", info.ID).
		Int("video_resource_id", video_resource.ID).
		Int("audio_resource_id", audio_resource.ID).
		Int64("merged_size", merged_info.Size()).
		Msg("Postprocessor.youtube: video and audio merged into MP4")
	return nil
}

func validate_youtube_subtitle(resource hermes.ResourceJob) error {
	kind := youtube_resource_media_type(resource.Kind)
	if kind != "text/vtt" && kind != "application/ttml+xml" {
		return nil
	}
	file, err := os.Open(resource.FilePath)
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 512))
	if err != nil {
		return err
	}
	data = bytes.TrimSpace(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}))
	if len(data) == 0 || youtube_resource_media_type(http.DetectContentType(data)) == "text/html" {
		return errors.New("字幕源返回空内容或 HTML，无法下载有效字幕；YouTube 字幕请检查 web.subs PO Token 是否有效并重新解析视频")
	}
	if kind == "text/vtt" && !(bytes.Equal(data, []byte("WEBVTT")) ||
		bytes.HasPrefix(data, []byte("WEBVTT\n")) || bytes.HasPrefix(data, []byte("WEBVTT\r")) ||
		bytes.HasPrefix(data, []byte("WEBVTT ")) || bytes.HasPrefix(data, []byte("WEBVTT\t"))) {
		return errors.New("字幕源返回的内容不是有效的 WebVTT 字幕")
	}
	return nil
}

func youtube_media_resource_indexes(resources []hermes.ResourceJob) (int, int) {
	video_index := -1
	audio_index := -1
	for resource_index := range resources {
		resource := &resources[resource_index]
		if video_index < 0 && youtube_resource_is_mergeable_video(resource) {
			video_index = resource_index
			continue
		}
		if audio_index < 0 && youtube_resource_is_mergeable_audio(resource) {
			audio_index = resource_index
		}
	}
	return video_index, audio_index
}

func youtube_resource_is_mergeable_video(resource *hermes.ResourceJob) bool {
	if resource == nil {
		return false
	}
	media_type := youtube_resource_media_type(resource.Kind)
	if media_type == "video/mp4" || media_type == "video/webm" {
		return true
	}
	if media_type != "" && media_type != "application/octet-stream" && media_type != "binary/octet-stream" {
		return false
	}
	return strings.EqualFold(filepath.Ext(resource.Name), ".mp4") ||
		strings.EqualFold(filepath.Ext(resource.FilePath), ".mp4") ||
		strings.EqualFold(filepath.Ext(resource.Name), ".webm") ||
		strings.EqualFold(filepath.Ext(resource.FilePath), ".webm")
}

func youtube_resource_is_mergeable_audio(resource *hermes.ResourceJob) bool {
	if resource == nil {
		return false
	}
	media_type := youtube_resource_media_type(resource.Kind)
	if media_type == "audio/mp4" || media_type == "audio/x-m4a" || media_type == "audio/webm" {
		return true
	}
	if media_type != "" && media_type != "application/octet-stream" && media_type != "binary/octet-stream" {
		return false
	}
	return strings.EqualFold(filepath.Ext(resource.Name), ".m4a") ||
		strings.EqualFold(filepath.Ext(resource.FilePath), ".m4a") ||
		strings.EqualFold(filepath.Ext(resource.Name), ".weba") ||
		strings.EqualFold(filepath.Ext(resource.FilePath), ".weba")
}

func youtube_resource_media_type(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if media_type, _, err := mime.ParseMediaType(kind); err == nil {
		return media_type
	}
	return kind
}

func require_youtube_regular_file(file_path string, media_name string) error {
	file_info, err := os.Stat(file_path)
	if err != nil {
		return fmt.Errorf("youtube postprocess: stat %s file: %w", media_name, err)
	}
	if !file_info.Mode().IsRegular() {
		return fmt.Errorf("youtube postprocess: %s path is not a regular file", media_name)
	}
	return nil
}

func youtube_merge_media(ctx context.Context, ffmpeg_path string, video_path string, audio_path string) (string, error) {
	temporary_file, err := os.CreateTemp(filepath.Dir(video_path), ".youtube-merge-*.mp4")
	if err != nil {
		return "", fmt.Errorf("youtube postprocess: create merge file: %w", err)
	}
	merged_path := temporary_file.Name()
	if err := temporary_file.Close(); err != nil {
		os.Remove(merged_path)
		return "", fmt.Errorf("youtube postprocess: close merge file: %w", err)
	}
	if err := os.Remove(merged_path); err != nil {
		return "", fmt.Errorf("youtube postprocess: prepare merge file: %w", err)
	}

	output, err := run_youtube_ffmpeg(ctx, ffmpeg_path,
		"-hide_banner", "-loglevel", "error", "-nostdin", "-y",
		"-i", video_path,
		"-i", audio_path,
		"-map", "0:v:0",
		"-map", "1:a:0",
		// Normalize codecs as well as the container for browser playback.
		"-c:v", "libx264", "-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-movflags", "+faststart",
		"-f", "mp4",
		merged_path,
	)
	if err != nil {
		os.Remove(merged_path)
		message := strings.TrimSpace(string(output))
		if message == "" {
			return "", fmt.Errorf("youtube postprocess: ffmpeg merge failed: %w", err)
		}
		return "", fmt.Errorf("youtube postprocess: ffmpeg merge failed: %w: %s", err, message)
	}
	merged_info, err := os.Stat(merged_path)
	if err != nil {
		return "", fmt.Errorf("youtube postprocess: stat merged file: %w", err)
	}
	if !merged_info.Mode().IsRegular() || merged_info.Size() <= 0 {
		os.Remove(merged_path)
		return "", fmt.Errorf("youtube postprocess: ffmpeg produced an empty output")
	}
	return merged_path, nil
}

func replace_youtube_video(video_path string, merged_path string, file_mode os.FileMode) error {
	backup_file, err := os.CreateTemp(filepath.Dir(video_path), ".youtube-video-backup-*")
	if err != nil {
		return fmt.Errorf("youtube postprocess: create video backup path: %w", err)
	}
	backup_path := backup_file.Name()
	if err := backup_file.Close(); err != nil {
		os.Remove(backup_path)
		return fmt.Errorf("youtube postprocess: close video backup path: %w", err)
	}
	if err := os.Remove(backup_path); err != nil {
		return fmt.Errorf("youtube postprocess: prepare video backup path: %w", err)
	}
	if err := os.Rename(video_path, backup_path); err != nil {
		return fmt.Errorf("youtube postprocess: back up original video: %w", err)
	}
	restore_original := true
	defer func() {
		if restore_original {
			_ = os.Rename(backup_path, video_path)
		}
	}()
	if err := os.Rename(merged_path, video_path); err != nil {
		return fmt.Errorf("youtube postprocess: replace original video: %w", err)
	}
	if err := os.Chmod(video_path, file_mode); err != nil {
		_ = os.Remove(video_path)
		return fmt.Errorf("youtube postprocess: restore video permissions: %w", err)
	}
	restore_original = false
	if err := os.Remove(backup_path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("youtube postprocess: remove original video backup: %w", err)
	}
	return nil
}
