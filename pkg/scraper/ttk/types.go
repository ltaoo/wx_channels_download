package ttk

import (
	"context"
	"errors"
)

const ttk_base_url = "https://ttks.tw"

var (
	// ErrUnsupportedURL is returned when Fetch receives a non-TTK URL.
	ErrUnsupportedURL = errors.New("unsupported ttk url")
	// ErrFetchInterrupted is returned after the fetch context is cancelled.
	ErrFetchInterrupted = errors.New("ttk fetch interrupted")
)

// FetchParams contains the input accepted by Fetch.
type FetchParams struct {
	URL          string          `json:"url"`
	RequestID    string          `json:"request_id,omitempty"`
	ForceRefresh bool            `json:"force_refresh,omitempty"`
	Context      context.Context `json:"-"`
}

const (
	FetchStageStart       = "start"
	FetchStageProfile     = "profile"
	FetchStageDirectory   = "directory"
	FetchStageChapter     = "chapter"
	FetchStageComplete    = "complete"
	FetchStageFailed      = "failed"
	FetchStageInterrupted = "interrupted"

	FetchStatusRunning     = "running"
	FetchStatusCompleted   = "completed"
	FetchStatusFailed      = "failed"
	FetchStatusInterrupted = "interrupted"
)

// FetchProgress describes a progress snapshot emitted while Fetch loads a
// novel profile, its directory, and each chapter.
type FetchProgress struct {
	RequestID    string             `json:"request_id"`
	Platform     string             `json:"platform"`
	URL          string             `json:"url"`
	BookID       string             `json:"book_id,omitempty"`
	BookTitle    string             `json:"book_title,omitempty"`
	Stage        string             `json:"stage"`
	Status       string             `json:"status"`
	Current      int                `json:"current"`
	Total        int                `json:"total"`
	Percent      float64            `json:"percent"`
	ChapterID    string             `json:"chapter_id,omitempty"`
	ChapterTitle string             `json:"chapter_title,omitempty"`
	Message      string             `json:"message"`
	Error        string             `json:"error,omitempty"`
	Cached       bool               `json:"cached,omitempty"`
	CacheHits    int                `json:"cache_hits,omitempty"`
	Profile      *TtkNovel          `json:"-"`
	Chapter      *TtkFetchedChapter `json:"-"`
}

// FetchProgressHandler receives progress snapshots from Fetch.
type FetchProgressHandler func(FetchProgress)

// TtkResp wraps parsed data with cache metadata.
type TtkResp[T any] struct {
	Data   T    `json:"data"`
	Cached bool `json:"cached,omitempty"`
}

// TtkNovel describes a TTK novel and its chapter directory.
type TtkNovel struct {
	Title    string       `json:"title"`
	URL      string       `json:"url"`
	Author   string       `json:"author,omitempty"`
	CoverURL string       `json:"cover_url,omitempty"`
	Chapters []TtkChapter `json:"chapters,omitempty"`
}

// TtkChapter describes one chapter entry in a TTK directory.
type TtkChapter struct {
	Index int    `json:"index"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// TtkChapterContent contains one fetched chapter body.
type TtkChapterContent struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// TtkFetchedChapter is a fetched chapter plus its position in the novel.
type TtkFetchedChapter struct {
	Index   int    `json:"index"`
	URL     string `json:"url"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// TtkFetchResult contains the directory and all fetched chapter contents.
type TtkFetchResult struct {
	Profile  *TtkNovel           `json:"profile"`
	Chapters []TtkFetchedChapter `json:"chapters"`
}
