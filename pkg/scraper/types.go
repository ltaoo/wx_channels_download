// Package scraper defines shared extraction results and playback sources.
package scraper

import (
	"encoding/json"
	"net/http"
)

type Profile struct {
	ID                    string          `json:"id"`
	Kind                  string          `json:"kind"` // series, movie, or video when the source does not identify its type
	Title                 string          `json:"title"`
	Description           string          `json:"description"`
	CoverURL              string          `json:"cover_url"`
	ReleaseDate           string          `json:"release_date"`
	EpisodeCount          int             `json:"episode_count"`
	Metadata              json.RawMessage `json:"metadata,omitempty"`
	Homepage              string          `json:"homepage,omitempty"`
	BackdropURL           string          `json:"backdrop_url,omitempty"`
	ReleaseYear           int             `json:"release_year,omitempty"`
	AvailableEpisodeCount int             `json:"available_episode_count,omitempty"`
	Genres                []string        `json:"genres,omitempty"`
	Countries             []string        `json:"countries,omitempty"`
	Status                string          `json:"status,omitempty"`
	Tips                  string          `json:"tips,omitempty"`
	Tagline               string          `json:"tagline,omitempty"`
	Credits               []Credit        `json:"credits,omitempty"`
}

// Credit is a source person's participation in a work; database identity and
// relationship persistence belong to the adapter.
type Credit struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Character string `json:"character,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	SortOrder int    `json:"sort_order"`
}

type Episode struct {
	ID          string  `json:"id"`
	SourceURL   string  `json:"source_url"`
	Title       string  `json:"title"`
	Order       int     `json:"order"`
	Duration    float64 `json:"duration"` // seconds
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	Codec       string  `json:"codec"`
	URL         string  `json:"url,omitempty"`
	Description string  `json:"description,omitempty"`
	CoverURL    string  `json:"cover_url,omitempty"`
	ReleaseDate string  `json:"release_date,omitempty"`
	LongTitle   string  `json:"long_title,omitempty"`
	Badge       string  `json:"badge,omitempty"`
}

type PlayInfo struct {
	SeparateAudio bool
	Address       string
	Key           []byte
	Codec         string
	Width         int
	Height        int
	Duration      float64
}

type Video struct {
	Transport         http.RoundTripper  `json:"-"` // Optional source-specific streaming transport.
	CheckMediaAddress func(string) error `json:"-"` // Source-owned URL policy used for resources and redirects.
	UserAgent         string
	Referer           string
	Profile           *Profile
	Title             string
	Playlist          string
	AudioPlaylist     string
	Info              PlayInfo
}
