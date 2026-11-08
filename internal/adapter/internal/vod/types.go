// Package vod implements shared downloader-model mapping for platform adapters.
package vod

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"wx_channel/pkg/scraper"
)

// Result exists only after a concrete adapter has decoded its scraper result.
type Result struct {
	Platform  string
	SourceURL string
	Profile   scraper.Profile
	Episodes  []scraper.Episode
}
type Specification struct {
	HTTPClient      *http.Client // Optional source transport, primarily for embedding and tests.
	VideoStreamType string
	PlatformID      string
	Name            string
	Homepage        string
	Resolve         func(context.Context, string, scraper.Episode, string, *http.Client) (scraper.Video, error)
	Fetch           func(context.Context, string, string) (any, error)
	Parse           func(any) (*Result, error)
	ParseURL        func(string) (string, string, string, error)
}

// Decode handles the in-memory native result and its JSON fetch-job representation.
func Decode[T any](data any) (*T, error) {
	switch value := data.(type) {
	case *T:
		if value == nil {
			return nil, fmt.Errorf("抓取结果为空")
		}
		return value, nil
	case T:
		return &value, nil
	}
	var raw []byte
	switch value := data.(type) {
	case json.RawMessage:
		raw = value
	case []byte:
		raw = value
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		raw = encoded
	}
	var result T
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
