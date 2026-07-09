package qidian

import (
	"encoding/json"
	"time"
)

const (
	PlatformID = "qidian"
	BaseURL    = "https://www.qidian.com"
)

type URLParts struct {
	BookID    string
	Canonical string
}

type BookVolume struct {
	Idx      int       `json:"idx"`
	Title    string    `json:"title"`
	Chapters []Chapter `json:"chapters"`
}

type Chapter struct {
	Idx         int       `json:"idx"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Locked      bool      `json:"locked,omitempty"`
	WordCount   int64     `json:"word_count,omitempty"`
	PublishedAt time.Time `json:"published_at,omitempty"`
}

type Author struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name"`
	URL    string `json:"url"`
	Avatar string `json:"avatar,omitempty"`
	Desc   string `json:"desc,omitempty"`
}

type BookProfile struct {
	URL              string          `json:"url"`
	Title            string          `json:"title"`
	Description      string          `json:"description"`
	Slogan           string          `json:"slogan"`
	CoverURL         string          `json:"cover_url"`
	LatestUpdateAt   time.Time       `json:"latest_update_at,omitempty"`
	Tags             []string        `json:"tags,omitempty"`
	LatestChapter    Chapter         `json:"latest_chapter"`
	ChapterCount     int             `json:"chapter_count"`
	WordCount        int64           `json:"word_count,omitempty"`
	DisplayWordCount string          `json:"display_word_count,omitempty"`
	Category         string          `json:"category,omitempty"`
	SubCategory      string          `json:"sub_category,omitempty"`
	Status           string          `json:"status,omitempty"`
	Author           Author          `json:"author"`
	Volumes          []BookVolume    `json:"volumes,omitempty"`
	PageContextJSON  json.RawMessage `json:"-"`
	PageHTML         string          `json:"-"`
}
