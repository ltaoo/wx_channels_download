package xiaohongshu

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// PlatformID is the stable Xiaohongshu platform identifier.
const PlatformID = "xiaohongshu"

// FetchResult is the structured result of scraping one Xiaohongshu note.
type FetchResult struct {
	SourceURL string `json:"source_url"`
	PageURL   string `json:"page_url,omitempty"`
	HTML      string `json:"html"`
	Note      *Note  `json:"note"`
}

// Note contains the note metadata embedded in Xiaohongshu's initial state.
type Note struct {
	XSecToken      string      `json:"xsecToken"`
	Title          string      `json:"title"`
	Description    string      `json:"desc"`
	Type           string      `json:"type"`
	NoteID         string      `json:"noteId"`
	Time           int64       `json:"time"`
	LastUpdateTime int64       `json:"lastUpdateTime"`
	User           NoteUser    `json:"user"`
	InteractInfo   Interaction `json:"interactInfo"`
	ImageList      []Image     `json:"imageList"`
	TagList        []Tag       `json:"tagList"`
	Video          Video       `json:"video"`
}

type NoteUser struct {
	UserID    string `json:"userId"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar"`
	XSecToken string `json:"xsecToken"`
}

type Interaction struct {
	LikedCount     FlexibleInt64 `json:"likedCount"`
	CommentCount   FlexibleInt64 `json:"commentCount"`
	CollectedCount FlexibleInt64 `json:"collectedCount"`
	ShareCount     FlexibleInt64 `json:"shareCount"`
}

// FlexibleInt64 accepts the string and numeric count encodings used by the
// Xiaohongshu initial state.
type FlexibleInt64 int64

func (v *FlexibleInt64) UnmarshalJSON(data []byte) error {
	value := strings.TrimSpace(string(data))
	if value == "" || value == "null" || value == `""` {
		*v = 0
		return nil
	}
	if strings.HasPrefix(value, `"`) {
		if err := json.Unmarshal(data, &value); err != nil {
			return fmt.Errorf("invalid Xiaohongshu count %q", value)
		}
	}
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, "+")
	value = strings.TrimSpace(value)
	value = strings.NewReplacer(",", "", "，", "").Replace(value)

	parsed_value, err := strconv.ParseInt(value, 10, 64)
	if err == nil {
		*v = FlexibleInt64(parsed_value)
		return nil
	}

	multiplier := float64(1)
	for suffix, suffix_multiplier := range map[string]float64{
		"万": 10_000,
		"亿": 100_000_000,
		"w": 10_000,
		"W": 10_000,
		"k": 1_000,
		"K": 1_000,
	} {
		if strings.HasSuffix(value, suffix) {
			value = strings.TrimSpace(strings.TrimSuffix(value, suffix))
			multiplier = suffix_multiplier
			break
		}
	}

	parsed_float, float_err := strconv.ParseFloat(value, 64)
	if float_err != nil || math.IsNaN(parsed_float) || math.IsInf(parsed_float, 0) {
		return fmt.Errorf("invalid Xiaohongshu count %q", value)
	}
	scaled_value := parsed_float * multiplier
	max_int64_boundary := float64(uint64(1) << 63)
	if math.IsInf(scaled_value, 0) || scaled_value >= max_int64_boundary || scaled_value < -max_int64_boundary {
		return fmt.Errorf("Xiaohongshu count out of range %q", value)
	}
	if multiplier == 1 {
		parsed_value = int64(scaled_value)
	} else {
		parsed_value = int64(math.Round(scaled_value))
	}
	*v = FlexibleInt64(parsed_value)
	return nil
}

type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type Image struct {
	FileID     string      `json:"fileId"`
	URL        string      `json:"url"`
	URLPreview string      `json:"urlPre"`
	URLDefault string      `json:"urlDefault"`
	Width      int         `json:"width"`
	Height     int         `json:"height"`
	InfoList   []ImageInfo `json:"infoList"`
}

type ImageInfo struct {
	ImageScene string `json:"imageScene"`
	URL        string `json:"url"`
}

type Video struct {
	Media VideoMedia `json:"media"`
	Capa  VideoCapa  `json:"capa"`
}

type VideoCapa struct {
	Duration int64 `json:"duration"`
}

type VideoMedia struct {
	Video  VideoMetadata `json:"video"`
	Stream VideoStreams  `json:"stream"`
}

type VideoMetadata struct {
	Duration int64  `json:"duration"`
	MD5      string `json:"md5"`
	BizID    string `json:"bizId"`
}

// VideoStreams groups variants by Xiaohongshu's codec family key. The keys
// are not stable: pages currently use values such as h264, EF4, and EF5.
type VideoStreams map[string][]VideoStream

type VideoStream struct {
	StreamType     int      `json:"streamType"`
	StreamDesc     string   `json:"streamDesc"`
	DefaultStream  int      `json:"defaultStream"`
	Format         string   `json:"format"`
	Width          int      `json:"width"`
	Height         int      `json:"height"`
	Duration       int64    `json:"duration"`
	Size           int64    `json:"size"`
	AverageBitrate int      `json:"avgBitrate"`
	FPS            int      `json:"fps"`
	VideoCodec     string   `json:"videoCodec"`
	AudioCodec     string   `json:"audioCodec"`
	AudioChannels  int      `json:"audioChannels"`
	MasterURL      string   `json:"masterUrl"`
	BackupURLs     []string `json:"backupUrls"`
	QualityType    string   `json:"qualityType"`
}

// HomeContentList is a structured Xiaohongshu profile tab.
type HomeContentList struct {
	Source     string         `json:"source"`
	Scope      string         `json:"scope"`
	Items      []HomeNoteItem `json:"items"`
	NextMarker string         `json:"next_marker"`
	Redacted   bool           `json:"redacted,omitempty"`
	HTML       string         `json:"-"`
}

type HomeNoteItem struct {
	ID        string       `json:"id"`
	Index     int          `json:"index"`
	URL       string       `json:"url,omitempty"`
	XSecToken string       `json:"xsecToken"`
	NoteCard  HomeNoteCard `json:"noteCard"`
}

type HomeNoteCard struct {
	NoteID       string      `json:"noteId"`
	XSecToken    string      `json:"xsecToken"`
	DisplayTitle string      `json:"displayTitle"`
	Type         string      `json:"type"`
	Time         int64       `json:"time"`
	User         NoteUser    `json:"user"`
	InteractInfo Interaction `json:"interactInfo"`
	Cover        Image       `json:"cover"`
}
