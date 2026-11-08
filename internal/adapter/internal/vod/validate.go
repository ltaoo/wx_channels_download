// Adapted from dramadl; see ../../../../pkg/scraper/NOTICE and LICENSE.
package vod

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"wx_channel/pkg/scraper"
)

func validate_media(ctx context.Context, filename string, info scraper.PlayInfo) error {
	output, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format=duration:stream=codec_name,codec_type,width,height,duration", "-of", "json", filename).Output()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("下载文件无法通过 FFprobe 检查")
	}
	var metadata struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			CodecName string `json:"codec_name"`
			Duration  string `json:"duration"`
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}
	if json.Unmarshal(output, &metadata) != nil {
		return errors.New("FFprobe 返回无效媒体信息")
	}
	duration, err := strconv.ParseFloat(metadata.Format.Duration, 64)
	if err != nil || math.IsNaN(duration) || math.IsInf(duration, 0) || duration <= 0 || math.Abs(duration-info.Duration) > 1 {
		return fmt.Errorf("文件时长与完整分集不匹配（文件 %.2f 秒，片源 %.2f 秒），拒绝将预览或截断文件标为完成", duration, info.Duration)
	}
	has_video, has_audio := false, false
	for _, stream := range metadata.Streams {
		if info.SeparateAudio && (stream.CodecType == "video" || stream.CodecType == "audio") {
			track_duration, err := strconv.ParseFloat(stream.Duration, 64)
			if err != nil || math.IsNaN(track_duration) || math.IsInf(track_duration, 0) || math.Abs(track_duration-info.Duration) > 1 {
				return errors.New("音视频轨道时长与完整分集不匹配")
			}
		}
		if stream.CodecType == "audio" {
			has_audio = true
		}
		if stream.CodecType == "video" && (stream.CodecName == "h264" || stream.CodecName == "hevc") && short_edge(stream.Width, stream.Height) >= short_edge(info.Width, info.Height) {
			has_video = true
		}
	}
	if has_video && (!info.SeparateAudio || has_audio) {
		// MP4 headers can still report valid duration and dimensions when a wrong
		// content key leaves encrypted samples unreadable. Decode a bounded sample
		// before publishing; metadata inspection alone cannot detect that failure.
		if err := exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-v", "error", "-xerror", "-i", filename, "-t", "1", "-map", "0:v:0", "-an", "-f", "null", "-").Run(); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return errors.New("下载文件视频样本无法解码，可能解密失败或媒体损坏")
		}
		return nil
	}

	return errors.New("文件未包含符合片源尺寸的 H.264/HEVC 视频轨道")
}

func short_edge(width, height int) int {
	if width < height {
		return width
	}
	return height
}
