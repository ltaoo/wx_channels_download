package source

import (
	"errors"
	"math"

	"wx_channel/pkg/hls"
)

// ParseHLS applies the platform's media address policy to every resource
// returned by the platform-independent HLS parser, including keys and init data.
func (p Policy) ParseHLS(text string) (hls.Playlist, error) {
	playlist, err := hls.ParseMedia(text)
	if err != nil {
		return hls.Playlist{}, err
	}
	for _, resource := range playlist.Resources {
		if err := p.CheckMediaAddress(resource.Address); err != nil {
			return hls.Playlist{}, err
		}
	}
	return playlist, nil
}

func (p Policy) CheckPlaylistDuration(text string, duration float64) error {
	playlist, err := p.ParseHLS(text)
	if err != nil {
		return err
	}
	if duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) || math.Abs(playlist.Duration-duration) > math.Max(2, duration*0.01) {
		return errors.New("播放清单时长与完整视频不符，拒绝保存试看或截断片段")
	}
	return nil
}
