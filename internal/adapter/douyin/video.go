package douyinadapter

import (
	"fmt"
	"strings"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/douyin"
	"wx_channel/pkg/util"
)

func douyin_video_variants(video_id, default_url string, width, height int, bit_rates []douyin.BitRate) []model.ContentVideoVariant {
	default_url = douyin_no_watermark_url(default_url)
	var variants []model.ContentVideoVariant
	seen_urls := make(map[string]bool)
	now := util.NowMillis()
	for stream_index := range bit_rates {
		stream := &bit_rates[stream_index]
		stream_url := douyin_no_watermark_url(first_douyin_url(stream.PlayAddr.UrlList))
		if stream_url == "" || seen_urls[stream_url] {
			continue
		}
		seen_urls[stream_url] = true
		key := fmt.Sprintf("bitrate:%s:%d", strings.TrimSpace(stream.GearName), stream_index)
		variant := model.ContentVideoVariant{
			VideoId:    BuildContentID(video_id),
			VariantKey: key,
			Spec:       strings.TrimSpace(stream.GearName),
			Size:       stream.PlayAddr.DataSize,
			Format:     "mp4",
			StreamType: model.ContentVideoVariantStreamTypeProgressive,
			HasVideo:   1,
			HasAudio:   1,
			URL:        stream_url,
			Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
		}
		if variant.Spec == "" {
			variant.Spec = key
		}
		if stream.PlayAddr.Width > 0 && stream.PlayAddr.Height > 0 {
			variant.Width = &stream.PlayAddr.Width
			variant.Height = &stream.PlayAddr.Height
			resolution := stream.PlayAddr.Height
			if stream.PlayAddr.Width < resolution {
				resolution = stream.PlayAddr.Width
			}
			variant.Quality = fmt.Sprintf("%dp", resolution)
		}
		if stream.BitRate > 0 {
			bitrate := stream.BitRate / 1000
			variant.Bitrate = &bitrate
		}
		if stream.IsH265 == 1 {
			variant.Codec = "h265"
		} else {
			variant.Codec = "h264"
		}
		if stream_url == default_url {
			variant.IsDefault = 1
		}
		variants = append(variants, variant)
	}
	if default_url != "" && !seen_urls[default_url] {
		variant := model.ContentVideoVariant{
			VideoId: BuildContentID(video_id), VariantKey: "default", Spec: "default", Quality: "默认",
			Format: "mp4", StreamType: model.ContentVideoVariantStreamTypeProgressive,
			HasVideo: 1, HasAudio: 1, IsDefault: 1, URL: default_url,
			Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
		}
		if width > 0 {
			variant.Width = &width
		}
		if height > 0 {
			variant.Height = &height
		}
		variants = append([]model.ContentVideoVariant{variant}, variants...)
	} else if default_url == "" && len(variants) > 0 {
		variants[0].IsDefault = 1
	}
	return variants
}

func douyin_selected_video_variant(variants []model.ContentVideoVariant, config map[string]any) (*model.ContentVideoVariant, error) {
	variant_key, _ := config["video_variant_key"].(string)
	spec, _ := config["video_variant_spec"].(string)
	if strings.TrimSpace(spec) == "" {
		spec, _ = config["spec"].(string)
	}
	variant_key, spec = strings.TrimSpace(variant_key), strings.TrimSpace(spec)
	for variant_index := range variants {
		variant := &variants[variant_index]
		if variant_key != "" {
			if variant.VariantKey == variant_key {
				return variant, nil
			}
		} else if spec != "" {
			if variant.Spec == spec {
				return variant, nil
			}
		} else if variant.IsDefault == 1 {
			return variant, nil
		}
	}
	if variant_key != "" || spec != "" {
		return nil, fmt.Errorf("抖音视频不包含所选视频规格 %s %s", variant_key, spec)
	}
	return nil, nil
}
