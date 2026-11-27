package vod

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"strconv"

	"gorm.io/gorm"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/hermes"
)

// Postprocess merges/decrypts downloaded raw resources, validates the MP4,
// then persists media facts before Hermes finalizes filenames and task status.
func (h *Handler) Postprocess(ctx context.Context, info *hermes.TaskJob, deps adapter.PostprocessDeps) error {
	if info == nil || deps.DB == nil {
		return fmt.Errorf("%s media archive dependencies are nil", h.platform)
	}
	if err := PostprocessMediaResources(ctx, info); err != nil {
		return err
	}
	for resource_index := range info.Resources {
		resource := &info.Resources[resource_index]
		if resource.Kind != "video/mp4" || resource.Extra["vod_owner"] != "" {
			continue
		}
		output, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format=duration:stream=codec_type,codec_name,width,height", "-of", "json", resource.FilePath).Output()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("读取下载后的媒体信息失败: %w", err)
		}
		var media struct {
			Format struct {
				Duration string `json:"duration"`
			} `json:"format"`
			Streams []struct {
				Type   string `json:"codec_type"`
				Codec  string `json:"codec_name"`
				Width  int    `json:"width"`
				Height int    `json:"height"`
			} `json:"streams"`
		}
		if err := json.Unmarshal(output, &media); err != nil {
			return err
		}
		duration, err := strconv.ParseFloat(media.Format.Duration, 64)
		if err != nil || duration <= 0 || math.IsInf(duration, 0) || math.IsNaN(duration) {
			return fmt.Errorf("下载后的媒体缺少有效时长")
		}
		width, height, audio_tracks := 0, 0, 0
		codec := ""
		for _, stream := range media.Streams {
			if stream.Type == "audio" {
				audio_tracks++
			}
			if stream.Type == "video" && width == 0 {
				width, height, codec = stream.Width, stream.Height, stream.Codec
			}
		}
		if width <= 0 || height <= 0 {
			return fmt.Errorf("下载后的媒体缺少视频尺寸")
		}
		content_id := h.platform + ":" + resource.UniqueID
		duration_seconds := int64(math.Round(duration))
		err = deps.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&model.ContentVideo{}).Where("id = ?", content_id).Updates(map[string]any{"duration": duration_seconds, "width": width, "height": height, "codec": codec, "size": resource.Size, "audio_track_count": audio_tracks}).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.ContentEpisode{}).Where("id = ?", content_id).Updates(map[string]any{"duration": duration_seconds, "runtime": int(duration / 60)}).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.ContentVideoVariant{}).Where("video_id = ? AND variant_key = ?", content_id, "main").Updates(map[string]any{"width": width, "height": height, "codec": codec, "size": resource.Size, "has_audio": audio_tracks > 0}).Error; err != nil {
				return err
			}
			return tx.Model(&model.DownloadResource{}).Where("id = ?", resource.ID).Updates(map[string]any{"duration": duration_seconds, "extra": string_marshal(resource.Extra), "size": resource.Size, "downloaded": resource.Downloaded}).Error
		})
		if err != nil {
			return fmt.Errorf("保存下载后的媒体信息失败: %w", err)
		}
		resource.Duration = duration_seconds
	}
	return discard_media_inputs(info)
}
