package douyin

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Mobile iteminfo omits bit_rate; its play endpoint still serves these renditions.
func (c *Client) enrich_playback_variants(raw_data json.RawMessage) json.RawMessage {
	var envelope map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw_data))
	decoder.UseNumber()
	if err := decoder.Decode(&envelope); err != nil {
		return raw_data
	}
	detail, ok := douyin_pc_object(envelope["aweme_detail"])
	if !ok {
		page, err := find_douyin_pc_video_page(envelope)
		if err != nil {
			return raw_data
		}
		video_info, _ := douyin_pc_object(page["videoInfoRes"])
		items, _ := douyin_pc_array(video_info["item_list"])
		if len(items) == 0 {
			return raw_data
		}
		detail, _ = douyin_pc_object(items[0])
	}
	if images, _ := douyin_pc_array(detail["images"]); len(images) > 0 {
		return raw_data
	}
	video, ok := douyin_pc_object(detail["video"])
	if !ok {
		return raw_data
	}
	if bit_rates, _ := douyin_pc_array(video["bit_rate"]); len(bit_rates) > 0 {
		return raw_data
	}
	address, _ := douyin_pc_object(video["play_addr"])
	urls, _ := douyin_pc_array(address["url_list"])
	var playback_url *url.URL
	for _, candidate := range urls {
		parsed, err := url.Parse(douyin_pc_scalar_string(candidate))
		if err == nil && parsed.Scheme == "https" && parsed.Host == "aweme.snssdk.com" && parsed.User == nil &&
			(parsed.Path == "/aweme/v1/play/" || parsed.Path == "/aweme/v1/playwm/") {
			playback_url = parsed
			break
		}
	}
	if playback_url == nil {
		return raw_data
	}
	playback_url.Path = "/aweme/v1/play/"
	ratios := []string{"540p", "720p", "1080p"}
	results := make([]*BitRate, len(ratios))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var pending sync.WaitGroup
	for ratio_index := range ratios {
		ratio_index := ratio_index
		pending.Add(1)
		go func() {
			defer pending.Done()
			candidate := *playback_url
			query := candidate.Query()
			query.Set("ratio", ratios[ratio_index])
			candidate.RawQuery = query.Encode()
			variant, err := c.probe_playback_variant(ctx, candidate.String(), ratios[ratio_index])
			if err != nil {
				c.logger.Debug().Err(err).Str("ratio", ratios[ratio_index]).Msg("douyin: playback rendition unavailable")
				return
			}
			results[ratio_index] = variant
		}()
	}
	pending.Wait()
	var variants []BitRate
	seen := make(map[string]bool)
	for _, variant := range results {
		if variant == nil {
			continue
		}
		identity := fmt.Sprintf("%dx%d:%d", variant.PlayAddr.Width, variant.PlayAddr.Height, variant.PlayAddr.DataSize)
		if seen[identity] {
			continue
		}
		seen[identity] = true
		variants = append(variants, *variant)
	}
	if len(variants) == 0 {
		return raw_data
	}
	video["bit_rate"] = variants
	enriched, err := json.Marshal(envelope)
	if err != nil {
		return raw_data
	}
	return enriched
}

func (c *Client) probe_playback_variant(ctx context.Context, playback_url, ratio string) (*BitRate, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, playback_url, nil)
	if err != nil {
		return nil, fmt.Errorf("playback probe: construct request")
	}
	const probe_bytes = 64 * 1024
	req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", probe_bytes-1))
	req.Header.Set("User-Agent", douyin_home_user_agent)
	req.Header.Set("Referer", "https://www.douyin.com/")
	resp, err := c.web.request_client().Do(req)
	if err != nil {
		// net/http errors can contain signed redirect URLs.
		return nil, fmt.Errorf("playback probe: request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("playback probe: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, probe_bytes))
	if err != nil {
		return nil, fmt.Errorf("playback probe: read response")
	}
	// shortcut: Probe fast-start MP4 headers; add tail probing if Douyin moves moov after media.
	width, height := playback_mp4_dimensions(data)
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("playback probe: video dimensions unavailable")
	}
	size := resp.ContentLength
	if resp.StatusCode == http.StatusPartialContent {
		var start, end int64
		if _, err := fmt.Sscanf(resp.Header.Get("Content-Range"), "bytes %d-%d/%d", &start, &end, &size); err != nil || start != 0 || end < start || size <= end {
			return nil, fmt.Errorf("playback probe: invalid Content-Range")
		}
	}
	if size < 0 {
		size = 0
	}
	return &BitRate{
		GearName: ratio,
		PlayAddr: PlayAddr{UrlList: []string{playback_url}, Width: width, Height: height, DataSize: size},
	}, nil
}

func playback_mp4_dimensions(data []byte) (int, int) {
	for len(data) >= 8 {
		size := uint64(binary.BigEndian.Uint32(data[:4]))
		header_size := uint64(8)
		if size == 1 {
			if len(data) < 16 {
				return 0, 0
			}
			size, header_size = binary.BigEndian.Uint64(data[8:16]), 16
		} else if size == 0 {
			size = uint64(len(data))
		}
		if size < header_size {
			return 0, 0
		}
		available := size
		if available > uint64(len(data)) {
			available = uint64(len(data))
		}
		payload := data[header_size:available]
		switch string(data[4:8]) {
		case "moov", "trak":
			if width, height := playback_mp4_dimensions(payload); width > 0 && height > 0 {
				return width, height
			}
		case "tkhd":
			dimension_offset := 76
			if len(payload) > 0 && payload[0] == 1 {
				dimension_offset = 88
			}
			if len(payload) >= dimension_offset+8 && payload[0] <= 1 {
				width := int(binary.BigEndian.Uint32(payload[dimension_offset:]) >> 16)
				height := int(binary.BigEndian.Uint32(payload[dimension_offset+4:]) >> 16)
				if width > 0 && height > 0 {
					return width, height
				}
			}
		}
		if size > uint64(len(data)) {
			return 0, 0
		}
		data = data[size:]
	}
	return 0, 0
}
