package xdownloader

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNoDownloader        = errors.New("xdownloader: no downloader for request")
	ErrInvalidRequest      = errors.New("xdownloader: invalid request")
	ErrContentTooShort     = errors.New("xdownloader: content too short")
	ErrRangeNotSatisfiable = errors.New("xdownloader: range not satisfiable")
)

type Protocol string

const (
	ProtocolHTTP     Protocol = "http"
	ProtocolHTTPS    Protocol = "https"
	ProtocolHLS      Protocol = "hls"
	ProtocolDASH     Protocol = "dash"
	ProtocolExternal Protocol = "external"
)

type Status string

const (
	StatusConnecting  Status = "connecting"
	StatusDownloading Status = "downloading"
	StatusAssembling  Status = "assembling"
	StatusFinished    Status = "finished"
	StatusSkipped     Status = "skipped"
)

type Hook func(Progress)

type Downloader interface {
	Name() string
	CanDownload(Request) bool
	Download(context.Context, Request) (*Result, error)
}

type Selector interface {
	Select(Request) (Downloader, error)
}

type Request struct {
	URL                  string
	Method               string
	Headers              map[string]string
	Body                 []byte
	DestPath             string
	Protocol             Protocol
	TempExt              string
	Resume               bool
	Connections          int
	RateLimitBytesPerSec int64
	MinSize              int64
	MaxSize              int64
	Fragment             *FragmentPlan
	Progress             Hook
	Metadata             map[string]any
}

type FragmentPlan struct {
	ManifestURL     string
	Fragments       []Fragment
	Live            bool
	Concurrent      int
	KeepFragments   bool
	SkipUnavailable bool
	Metadata        map[string]any
}

type Fragment struct {
	Index    int
	URL      string
	Headers  map[string]string
	Range    *ByteRange
	Key      *FragmentKey
	Metadata map[string]any
}

type ByteRange struct {
	Start int64
	End   int64
}

type FragmentKey struct {
	Method string
	URI    string
	Key    []byte
	IV     []byte
}

type Progress struct {
	Status            Status
	Filename          string
	DownloadedBytes   int64
	TotalBytes        int64
	Percent           float64
	SpeedBytesPerSec  int64
	FragmentIndex     int
	FragmentCount     int
	StartedAt         time.Time
	UpdatedAt         time.Time
	Resumed           bool
	TemporaryFilename string
}

type Result struct {
	Path     string
	Bytes    int64
	Resumed  bool
	Metadata map[string]any
}
