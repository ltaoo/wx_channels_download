// Package kuaishou retrieves public Kuaishou short-video metadata and media
// URLs from share links and desktop detail pages.
package kuaishou

const (
	// PlatformID is the stable platform identifier used by the adapter layer.
	PlatformID = "kuaishou"
	// DefaultUserAgent matches the desktop browser request recorded in the
	// reference Kuaishou request chain.
	DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36"
)

// FetchResult contains the resolved Kuaishou page and its matching feed.
type FetchResult struct {
	SourceURL string `json:"source_url"`
	PageURL   string `json:"page_url"`
	PhotoID   string `json:"photo_id"`
	Feed      Feed   `json:"feed"`
}

// Feed is the public work and publisher data returned by
// visionShortVideoReco.
type Feed struct {
	Type            any             `json:"type"`
	Author          Author          `json:"author"`
	Photo           Photo           `json:"photo"`
	Tags            []Tag           `json:"tags"`
	AuthorStatement AuthorStatement `json:"authorStatement"`
}

// Author identifies the publisher of a Kuaishou work.
type Author struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	HeaderURL string `json:"headerUrl"`
}

// AuthorStatement carries the public statement attached to a work.
type AuthorStatement struct {
	Content       string `json:"content"`
	Type          any    `json:"type"`
	RiskStyleType any    `json:"riskStyleType"`
}

// Tag is one Kuaishou work tag.
type Tag struct {
	Type any    `json:"type"`
	Name string `json:"name"`
}

// Photo contains the useful subset of a Kuaishou video response.
type Photo struct {
	ID               string         `json:"id"`
	Duration         flexible_int64 `json:"duration"`
	Caption          string         `json:"caption"`
	OriginCaption    string         `json:"originCaption"`
	LikeCount        flexible_int64 `json:"likeCount"`
	ViewCount        flexible_int64 `json:"viewCount"`
	CommentCount     flexible_int64 `json:"commentCount"`
	RealLikeCount    flexible_int64 `json:"realLikeCount"`
	CoverURL         string         `json:"coverUrl"`
	PhotoURL         string         `json:"photoUrl"`
	PhotoH265URL     string         `json:"photoH265Url"`
	Manifest         Manifest       `json:"manifest"`
	ManifestH265     Manifest       `json:"manifestH265"`
	Timestamp        flexible_int64 `json:"timestamp"`
	ExpTag           string         `json:"expTag"`
	AnimatedCoverURL string         `json:"animatedCoverUrl"`
	VideoRatio       any            `json:"videoRatio"`
	StereoType       any            `json:"stereoType"`
	MusicBlocked     any            `json:"musicBlocked"`
	RiskTagContent   string         `json:"riskTagContent"`
	RiskTagURL       string         `json:"riskTagUrl"`
	VideoResource    any            `json:"videoResource"`
}

// DurationMillis returns the work duration reported by Kuaishou.
func (p Photo) DurationMillis() int64 { return int64(p.Duration) }

// LikeCountValue returns the normalized like count.
func (p Photo) LikeCountValue() int64 { return int64(p.LikeCount) }

// ViewCountValue returns the normalized view count.
func (p Photo) ViewCountValue() int64 { return int64(p.ViewCount) }

// CommentCountValue returns the normalized comment count.
func (p Photo) CommentCountValue() int64 { return int64(p.CommentCount) }

// TimestampValue returns the work publication timestamp.
func (p Photo) TimestampValue() int64 { return int64(p.Timestamp) }

// Manifest describes the progressive representations advertised by Kuaishou.
// The web API has returned this value both as an object and as a JSON-encoded
// string, so its custom decoder accepts both forms.
type Manifest struct {
	MediaType     any             `json:"mediaType"`
	BusinessType  any             `json:"businessType"`
	Version       any             `json:"version"`
	AdaptationSet []AdaptationSet `json:"adaptationSet"`
}

// AdaptationSet groups compatible media representations.
type AdaptationSet struct {
	ID             any              `json:"id"`
	Duration       flexible_int64   `json:"duration"`
	Representation []Representation `json:"representation"`
}

// Representation is one downloadable Kuaishou video variant.
type Representation struct {
	ID              any              `json:"id"`
	DefaultSelect   flexible_bool    `json:"defaultSelect"`
	BackupURL       flexible_strings `json:"backupUrl"`
	Codecs          string           `json:"codecs"`
	URL             string           `json:"url"`
	Height          flexible_int     `json:"height"`
	Width           flexible_int     `json:"width"`
	AverageBitrate  flexible_int     `json:"avgBitrate"`
	MaximumBitrate  flexible_int     `json:"maxBitrate"`
	QualityType     string           `json:"qualityType"`
	QualityLabel    string           `json:"qualityLabel"`
	FrameRate       flexible_int     `json:"frameRate"`
	Hidden          flexible_bool    `json:"hidden"`
	DisableAdaptive flexible_bool    `json:"disableAdaptive"`
}

// DurationMillis returns the adaptation-set duration reported by Kuaishou.
func (a AdaptationSet) DurationMillis() int64 { return int64(a.Duration) }

// IsDefault reports whether Kuaishou marked this representation as default.
func (r Representation) IsDefault() bool { return bool(r.DefaultSelect) }

// BackupURLs returns a copy of the representation's mirror URLs.
func (r Representation) BackupURLs() []string {
	return append([]string(nil), r.BackupURL...)
}

// WidthValue returns the normalized representation width.
func (r Representation) WidthValue() int { return int(r.Width) }

// HeightValue returns the normalized representation height.
func (r Representation) HeightValue() int { return int(r.Height) }

// AverageBitrateValue returns the normalized average bitrate.
func (r Representation) AverageBitrateValue() int { return int(r.AverageBitrate) }

// FrameRateValue returns the normalized frame rate.
func (r Representation) FrameRateValue() int { return int(r.FrameRate) }
