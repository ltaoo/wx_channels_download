package mgtvadapter

import (
	"wx_channel/internal/adapter/internal/vod"
	"wx_channel/pkg/scraper/mgtv"
)

// parse_result decodes this platform's extraction payload before mapping it to
// Content, related episode/series records, assets and download task endpoints.
func parse_result(data any) (*vod.Result, error) {
	result, err := vod.Decode[mgtv.FetchResult](data)
	if err != nil {
		return nil, err
	}
	return &vod.Result{Platform: result.Platform, SourceURL: result.SourceURL, Profile: result.Profile, Episodes: result.Episodes}, nil
}
