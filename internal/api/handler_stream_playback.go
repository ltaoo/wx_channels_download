package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/hermes"
)

const (
	stream_playback_workspace_dir = "playback"
	stream_playback_manifest_name = "index.m3u8"
)

var stream_playback_segment_name_pattern = regexp.MustCompile(`^segment-[0-9]{9}\.ts$`)

type stream_playback_location struct {
	output_path string
	safe_roots  []string
}

func download_task_stream_playback_url(task_id, resource_id int) string {
	return fmt.Sprintf(
		"/api/v1/download_task/live/%d/%d/%s",
		task_id,
		resource_id,
		stream_playback_manifest_name,
	)
}

func (c *APIClient) handle_stream_playback_asset(ctx *gin.Context) {
	task_id, task_err := strconv.Atoi(ctx.Param("task_id"))
	resource_id, resource_err := strconv.Atoi(ctx.Param("resource_id"))
	asset_name := strings.TrimSpace(ctx.Param("asset_name"))
	if task_err != nil || resource_err != nil || task_id <= 0 || resource_id <= 0 || !valid_stream_playback_asset_name(asset_name) {
		ctx.Status(http.StatusBadRequest)
		return
	}

	asset_path, err := c.resolve_stream_playback_asset(task_id, resource_id, asset_name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.Status(http.StatusNotFound)
			return
		}
		c.logger.Error().
			Int("task_id", task_id).
			Int("resource_id", resource_id).
			Str("asset_name", asset_name).
			Err(err).
			Msg("Failed to resolve live playback asset")
		ctx.Status(http.StatusInternalServerError)
		return
	}

	file, err := os.Open(asset_path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			ctx.Status(http.StatusNotFound)
			return
		}
		ctx.Status(http.StatusInternalServerError)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		ctx.Status(http.StatusNotFound)
		return
	}

	ctx.Header("X-Content-Type-Options", "nosniff")
	if asset_name == stream_playback_manifest_name {
		ctx.Header("Content-Type", "application/vnd.apple.mpegurl")
		ctx.Header("Cache-Control", "no-store, max-age=0")
	} else {
		ctx.Header("Content-Type", "video/mp2t")
		ctx.Header("Cache-Control", "private, max-age=31536000, immutable")
	}
	http.ServeContent(ctx.Writer, ctx.Request, asset_name, info.ModTime(), file)
}

func valid_stream_playback_asset_name(asset_name string) bool {
	return asset_name == stream_playback_manifest_name || stream_playback_segment_name_pattern.MatchString(asset_name)
}

func (c *APIClient) resolve_stream_playback_asset(task_id, resource_id int, asset_name string) (string, error) {
	if !valid_stream_playback_asset_name(asset_name) {
		return "", os.ErrNotExist
	}
	location, err := c.resolve_stream_playback_location(task_id, resource_id)
	if err != nil {
		return "", err
	}
	for _, playback_dir := range stream_playback_directories(location.output_path) {
		asset_path := filepath.Join(playback_dir, asset_name)
		info, stat_err := os.Lstat(asset_path)
		if errors.Is(stat_err, os.ErrNotExist) {
			continue
		}
		if stat_err != nil {
			return "", stat_err
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return "", os.ErrNotExist
		}
		resolved_path, resolve_err := filepath.EvalSymlinks(asset_path)
		if resolve_err != nil {
			return "", resolve_err
		}
		resolved_path, resolve_err = filepath.Abs(resolved_path)
		if resolve_err != nil {
			return "", resolve_err
		}
		if !path_within_any_download_root(location.safe_roots, resolved_path) {
			return "", fmt.Errorf("stream playback asset resolved outside its download root")
		}
		return filepath.Clean(resolved_path), nil
	}
	return "", os.ErrNotExist
}

func (c *APIClient) resolve_stream_playback_location(task_id, resource_id int) (stream_playback_location, error) {
	if c == nil || c.db == nil || c.cfg == nil {
		return stream_playback_location{}, errors.New("download service is unavailable")
	}
	var task model.DownloadTask
	if err := c.db.Where("id = ? AND deleted_at IS NULL", task_id).First(&task).Error; err != nil {
		return stream_playback_location{}, err
	}
	var resource model.DownloadResource
	if err := c.db.
		Where("id = ? AND task_id = ? AND deleted_at IS NULL AND UPPER(type) = ?", resource_id, task_id, model.ResourceTypeStream).
		First(&resource).Error; err != nil {
		return stream_playback_location{}, err
	}
	return c.stream_playback_location_for_resource(task, resource)
}

func (c *APIClient) stream_playback_location_for_resource(task model.DownloadTask, resource model.DownloadResource) (stream_playback_location, error) {
	output_path := hermes.ResolveOutputPath(c.cfg.DownloadDir, resource.DownloadDir, resource.UniqueID)
	output_path, err := filepath.Abs(output_path)
	if err != nil {
		return stream_playback_location{}, err
	}
	output_path = filepath.Clean(output_path)
	roots_by_path := c.download_task_local_file_roots(task, &resource)
	safe_roots := make([]string, 0, len(roots_by_path))
	for root := range roots_by_path {
		safe_roots = append(safe_roots, root)
		if resolved_root, resolve_err := filepath.EvalSymlinks(root); resolve_err == nil {
			if resolved_root, absolute_err := filepath.Abs(resolved_root); absolute_err == nil {
				resolved_root = filepath.Clean(resolved_root)
				if resolved_root != root {
					safe_roots = append(safe_roots, resolved_root)
				}
			}
		}
	}
	if !path_within_any_download_root(safe_roots, output_path) {
		return stream_playback_location{}, errors.New("stream output path is outside its download root")
	}
	return stream_playback_location{output_path: output_path, safe_roots: safe_roots}, nil
}

func stream_playback_directories(output_path string) []string {
	return []string{
		filepath.Join(hermes.StreamRecordingDir(output_path), stream_playback_workspace_dir),
		hermes.StreamPlaybackDir(output_path),
	}
}

func path_within_any_download_root(roots []string, target string) bool {
	for _, root := range roots {
		if path_within_download_root(root, target) {
			return true
		}
	}
	return false
}

func (c *APIClient) stream_playback_availability(task_id int) map[int]bool {
	availability := make(map[int]bool)
	if c == nil || c.db == nil || c.cfg == nil {
		return availability
	}
	var task model.DownloadTask
	if err := c.db.Where("id = ? AND deleted_at IS NULL", task_id).First(&task).Error; err != nil {
		return availability
	}
	var resources []model.DownloadResource
	if err := c.db.
		Where("task_id = ? AND deleted_at IS NULL AND UPPER(type) = ?", task_id, model.ResourceTypeStream).
		Find(&resources).Error; err != nil {
		return availability
	}
	for _, resource := range resources {
		location, err := c.stream_playback_location_for_resource(task, resource)
		if err != nil {
			continue
		}
		for _, playback_dir := range stream_playback_directories(location.output_path) {
			manifest_path := filepath.Join(playback_dir, stream_playback_manifest_name)
			if info, stat_err := os.Lstat(manifest_path); stat_err == nil && info.Mode().IsRegular() && info.Size() > 0 {
				availability[resource.Id] = true
				break
			}
		}
	}
	return availability
}
