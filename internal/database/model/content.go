package model

import (
	"gorm.io/gorm"
)

type Content struct {
	Id            string         `gorm:"primaryKey" json:"id"`
	PlatformId    string         `gorm:"not null;index:idx_content_platform_type,priority:1;index:idx_content_external_id,priority:1" json:"platform_id"`
	Type          string         `gorm:"not null;index:idx_content_platform_type,priority:2;index:idx_content_type" json:"type"`
	Subtype       string         `gorm:"index:idx_content_subtype" json:"subtype"`
	ExternalId    string         `gorm:"not null;index:idx_content_external_id,priority:2" json:"external_id"`
	ExternalId2   string         `json:"external_id2"`
	ExternalId3   string         `json:"external_id3"`
	Title         string         `json:"title"`
	Description   string         `json:"description"`
	URL           string         `json:"url"`
	SourceURL     string         `json:"source_url"`
	CoverURL      string         `json:"cover_url"`
	CoverWidth    string         `json:"cover_width"`
	CoverHeight   string         `json:"cover_height"`
	PublishTime   *int64         `json:"publish_time"`
	UpdateTime    *int64         `json:"update_time"`
	IsPrivate     int            `json:"is_private"`
	ViewCount     int64          `json:"view_count"`
	LikeCount     int64          `json:"like_count"`
	CommentCount  int64          `json:"comment_count"`
	ShareCount    int64          `json:"share_count"`
	CollectCount  int64          `json:"collect_count"`
	Unread        int            `json:"unread"`
	SourceDeleted int            `json:"source_deleted"`
	Validated     int            `json:"validated"`
	Tags          string         `json:"tags"`
	Category      string         `json:"category"`
	Metadata      string         `json:"metadata"`
	Assets        []ContentAsset `gorm:"foreignKey:ContentId;references:Id" json:"assets,omitempty"`
	Timestamps
}

func (Content) TableName() string { return "content" }

func (c *Content) BeforeCreate(tx *gorm.DB) error {
	if c.Id == "" {
		c.Id = c.PlatformId + ":" + c.ExternalId
	}
	return nil
}

type ContentVideo struct {
	Id              string                      `gorm:"primaryKey" json:"id"`
	Duration        int64                       `json:"duration"`
	Width           int                         `json:"width"`
	Height          int                         `json:"height"`
	FPS             int                         `json:"fps"`
	Bitrate         int                         `json:"bitrate"`
	Size            int64                       `json:"size"`
	Codec           string                      `json:"codec"`
	Format          string                      `json:"format"`
	HasSubtitle     int                         `json:"has_subtitle"`
	SubtitleURL     string                      `json:"subtitle_url"`
	AudioTrackCount int                         `json:"audio_track_count"`
	URL             string                      `json:"url"`
	PlayTimes       int64                       `json:"play_times"`
	Variants        []ContentVideoVariant       `gorm:"foreignKey:VideoId;references:Id" json:"variants"`
	SubtitleTracks  []ContentVideoSubtitleTrack `gorm:"foreignKey:VideoId;references:Id" json:"subtitle_tracks"`
	DeletedAt       *int64                      `gorm:"column:deleted_at;index" json:"deleted_at"`
}

func (ContentVideo) TableName() string { return "content_video" }

const (
	ContentVideoVariantStreamTypeProgressive = "progressive"
	ContentVideoVariantStreamTypeVideoOnly   = "video_only"
	ContentVideoVariantStreamTypeManifest    = "manifest"
)

// ContentVideoVariant describes one selectable/downloadable video
// representation. AssetId is also the primary key of ContentAsset.
type ContentVideoVariant struct {
	AssetId      uint         `gorm:"primaryKey;autoIncrement:false" json:"asset_id"`
	VideoId      string       `gorm:"not null;index:idx_content_video_variant_video;uniqueIndex:idx_content_video_variant_identity,priority:1" json:"video_id"`
	VariantKey   string       `gorm:"not null;uniqueIndex:idx_content_video_variant_identity,priority:2" json:"variant_key"`
	Spec         string       `json:"spec"`
	Quality      string       `json:"quality"`
	Width        *int         `json:"width"`
	Height       *int         `json:"height"`
	FPS          *int         `json:"fps"`
	Bitrate      *int         `json:"bitrate"`
	Size         int64        `json:"size"`
	Codec        string       `json:"codec"`
	Format       string       `json:"format"`
	StreamType   string       `json:"stream_type"`
	HasVideo     int          `json:"has_video"`
	HasAudio     int          `json:"has_audio"`
	IsDefault    int          `json:"is_default"`
	URL          string       `json:"url"`
	URLExpiresAt *int64       `gorm:"column:url_expires_at" json:"url_expires_at"`
	Metadata     string       `json:"metadata"`
	Asset        ContentAsset `gorm:"foreignKey:AssetId;references:Id" json:"asset"`
	Timestamps
}

func (ContentVideoVariant) TableName() string { return "content_video_variant" }

const (
	ContentVideoSubtitleKindSubtitle = "subtitle"
	ContentVideoSubtitleKindCaption  = "caption"
	ContentVideoSubtitleKindForced   = "forced"
)

// ContentVideoSubtitleTrack is one logical language/role subtitle track.
type ContentVideoSubtitleTrack struct {
	Id                uint                         `gorm:"primaryKey;autoIncrement" json:"id"`
	VideoId           string                       `gorm:"not null;index:idx_content_video_subtitle_track_video;uniqueIndex:idx_content_video_subtitle_track_identity,priority:1" json:"video_id"`
	TrackKey          string                       `gorm:"not null;uniqueIndex:idx_content_video_subtitle_track_identity,priority:2" json:"track_key"`
	LanguageCode      string                       `gorm:"not null;default:und;index:idx_content_video_subtitle_track_language" json:"language_code"`
	LanguageName      string                       `json:"language_name"`
	Label             string                       `json:"label"`
	Kind              string                       `json:"kind"`
	IsDefault         int                          `json:"is_default"`
	IsForced          int                          `json:"is_forced"`
	IsAutoGenerated   int                          `json:"is_auto_generated"`
	IsHearingImpaired int                          `json:"is_hearing_impaired"`
	Sources           []ContentVideoSubtitleSource `gorm:"foreignKey:TrackId;references:Id" json:"sources"`
	Timestamps
}

func (ContentVideoSubtitleTrack) TableName() string {
	return "content_video_subtitle_track"
}

// ContentVideoSubtitleSource is one concrete downloadable subtitle file.
// AssetId is also the primary key of ContentAsset.
type ContentVideoSubtitleSource struct {
	AssetId      uint         `gorm:"primaryKey;autoIncrement:false" json:"asset_id"`
	TrackId      uint         `gorm:"not null;index:idx_content_video_subtitle_source_track;uniqueIndex:idx_content_video_subtitle_source_identity,priority:1" json:"track_id"`
	SourceKey    string       `gorm:"not null;uniqueIndex:idx_content_video_subtitle_source_identity,priority:2" json:"source_key"`
	Format       string       `json:"format"`
	URL          string       `json:"url"`
	URLExpiresAt *int64       `gorm:"column:url_expires_at" json:"url_expires_at"`
	Encoding     string       `json:"encoding"`
	Metadata     string       `json:"metadata"`
	Asset        ContentAsset `gorm:"foreignKey:AssetId;references:Id" json:"asset"`
	Timestamps
}

func (ContentVideoSubtitleSource) TableName() string {
	return "content_video_subtitle_source"
}

const (
	ContentImageTypeStill     = "still"
	ContentImageTypeLivePhoto = "live_photo"
)

type ContentImageLivePhotoFormat struct {
	FormatId   int    `json:"format_id"`
	URL        string `json:"url"`
	Size       int64  `json:"size"`
	DurationMs int64  `json:"duration_ms"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

// ContentImageLivePhoto describes the motion component paired with an album
// image. URL and the scalar media fields identify the selected download
// variant; Formats preserves every variant returned by the platform.
type ContentImageLivePhoto struct {
	Vid        string                        `json:"vid"`
	Type       int                           `json:"type"`
	URL        string                        `json:"url"`
	FormatId   int                           `json:"format_id"`
	Width      int                           `json:"width"`
	Height     int                           `json:"height"`
	Size       int64                         `json:"size"`
	DurationMs int64                         `json:"duration_ms"`
	Formats    []ContentImageLivePhotoFormat `gorm:"serializer:json;type:text" json:"formats,omitempty"`
}

type ContentImage struct {
	Id        uint                   `gorm:"primaryKey;autoIncrement" json:"id"`
	AlbumId   string                 `gorm:"not null;index:idx_content_image_album;uniqueIndex:idx_content_image_identity,priority:1" json:"album_id"`
	ImageKey  string                 `gorm:"not null;uniqueIndex:idx_content_image_identity,priority:2" json:"image_key"`
	SortOrder int                    `json:"sort_order"`
	URL       string                 `json:"url"`
	Width     int                    `json:"width"`
	Height    int                    `json:"height"`
	Size      int64                  `json:"size"`
	Ext       string                 `json:"ext"`
	ImageType string                 `gorm:"not null;default:still" json:"image_type"`
	LivePhoto *ContentImageLivePhoto `gorm:"embedded;embeddedPrefix:live_photo_" json:"live_photo,omitempty"`
	Assets    []ContentAssetLink     `gorm:"-" json:"assets,omitempty"`
	DeletedAt *int64                 `gorm:"column:deleted_at;index" json:"deleted_at"`
}

func (ContentImage) TableName() string { return "content_image" }

type ContentAudio struct {
	Id            string `gorm:"primaryKey" json:"id"`
	Duration      int64  `json:"duration"`
	Bitrate       int    `json:"bitrate"`
	Format        string `json:"format"`
	SampleRate    int    `json:"sample_rate"`
	Artist        string `json:"artist"`
	Album         string `json:"album"`
	Genre         string `json:"genre"`
	LyricsURL     string `json:"lyrics_url"`
	EpisodeNumber int    `json:"episode_number"`
	SeasonNumber  int    `json:"season_number"`
	SeriesName    string `json:"series_name"`
}

func (ContentAudio) TableName() string { return "content_audio" }

type ContentArticle struct {
	Id              string `gorm:"primaryKey" json:"id"`
	Type            string `json:"type"`
	WordCount       int    `json:"word_count"`
	ReadingTime     int    `json:"reading_time"`
	Text            string `json:"text"`
	HTML            string `gorm:"type:longtext" json:"html"`
	Markdown        string `json:"markdown"`
	ChapterNumber   int    `json:"chapter_number"`
	VolumeNumber    int    `json:"volume_number"`
	SeriesName      string `json:"series_name"`
	IsFinished      int    `json:"is_finished"`
	PublishPlatform string `json:"publish_platform"`
	IsOriginal      int    `json:"is_original"`
}

func (ContentArticle) TableName() string { return "content_article" }

const (
	ContentArticleTypeHTML = "html"
	ContentArticleTypeText = "text"
)

type ContentAccount struct {
	ContentId string `gorm:"primaryKey;index:idx_content_account_account" json:"content_id"`
	AccountId string `gorm:"primaryKey;index:idx_content_account_account" json:"account_id"`
	Role      string `json:"role"`
	CreatedAt int64  `json:"created_at"`
}

func (ContentAccount) TableName() string { return "content_account" }

type ContentInfluencer struct {
	ContentId    string `gorm:"primaryKey;index:idx_content_influencer_influencer" json:"content_id"`
	InfluencerId int    `gorm:"primaryKey;index:idx_content_influencer_influencer" json:"influencer_id"`
	Role         string `json:"role"`
	CreatedAt    int64  `json:"created_at"`
}

func (ContentInfluencer) TableName() string { return "content_influencer" }

// --- New extension tables ---

type ContentLive struct {
	Id          string `gorm:"primaryKey" json:"id"`
	Duration    int64  `json:"duration"`
	StreamURL   string `json:"stream_url"`
	CoverWidth  int    `json:"cover_width"`
	CoverHeight int    `json:"cover_height"`
	ViewerCount int64  `json:"viewer_count"`
	Format      string `json:"format"`
	Bitrate     int    `json:"bitrate"`
	Size        int64  `json:"size"`
	Metadata    string `json:"metadata"`
}

func (ContentLive) TableName() string { return "content_live" }

type ContentNovel struct {
	Id           string                `gorm:"primaryKey" json:"id"`
	AuthorName   string                `json:"author_name"`
	WordCount    int                   `json:"word_count"`
	ChapterCount int                   `json:"chapter_count"`
	VolumeCount  int                   `json:"volume_count"`
	SeriesName   string                `json:"series_name"`
	IsFinished   int                   `json:"is_finished"`
	Text         string                `gorm:"type:longtext" json:"text"`
	HTML         string                `gorm:"type:longtext" json:"html"`
	Volumes      []ContentNovelVolume  `gorm:"foreignKey:NovelId;references:Id" json:"volumes,omitempty"`
	Chapters     []ContentNovelChapter `gorm:"foreignKey:NovelId;references:Id" json:"chapters,omitempty"`
}

func (ContentNovel) TableName() string { return "content_novel" }

type ContentNovelVolume struct {
	Id        uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	NovelId   string `gorm:"not null;index:idx_novel_volume_novel;uniqueIndex:idx_novel_volume_identity,priority:1" json:"novel_id"`
	VolumeKey string `gorm:"not null;uniqueIndex:idx_novel_volume_identity,priority:2" json:"volume_key"`
	Idx       int    `json:"idx"`
	Title     string `json:"title"`
}

func (ContentNovelVolume) TableName() string { return "content_novel_volume" }

type ContentNovelChapter struct {
	Id         uint               `gorm:"primaryKey;autoIncrement" json:"id"`
	NovelId    string             `gorm:"not null;index:idx_novel_chapter_novel;index:idx_novel_chapter_novel_extra,priority:1;uniqueIndex:idx_novel_chapter_identity,priority:1" json:"novel_id"`
	ChapterKey string             `gorm:"not null;uniqueIndex:idx_novel_chapter_identity,priority:2" json:"chapter_key"`
	VolumeId   *uint              `json:"volume_id"`
	VolumeKey  string             `json:"volume_key"`
	Idx        int                `json:"idx"`
	Title      string             `json:"title"`
	URL        string             `json:"url"`
	Locked     bool               `json:"locked"`
	IsExtra    bool               `gorm:"not null;default:false;index:idx_novel_chapter_novel_extra,priority:2" json:"is_extra"`
	WordCount  int                `json:"word_count"`
	Assets     []ContentAssetLink `gorm:"-" json:"assets,omitempty"`
}

func (ContentNovelChapter) TableName() string { return "content_novel_chapter" }

type ContentAlbum struct {
	Id          string         `gorm:"primaryKey" json:"id"`
	ImageCount  int            `json:"image_count"`
	CoverWidth  int            `json:"cover_width"`
	CoverHeight int            `json:"cover_height"`
	Format      string         `json:"format"`
	Description string         `json:"description"`
	Images      []ContentImage `gorm:"foreignKey:AlbumId;references:Id" json:"images,omitempty"`
}

func (ContentAlbum) TableName() string { return "content_album" }

type ContentPodcast struct {
	Id            string `gorm:"primaryKey" json:"id"`
	Duration      int64  `json:"duration"`
	Bitrate       int    `json:"bitrate"`
	Format        string `json:"format"`
	SampleRate    int    `json:"sample_rate"`
	Artist        string `json:"artist"`
	SeriesName    string `json:"series_name"`
	EpisodeNumber int    `json:"episode_number"`
	SeasonNumber  int    `json:"season_number"`
	Size          int64  `json:"size"`
}

func (ContentPodcast) TableName() string { return "content_podcast" }

type ContentDocument struct {
	Id         string `gorm:"primaryKey" json:"id"`
	PageCount  int    `json:"page_count"`
	FileFormat string `json:"file_format"`
	FileSize   int64  `json:"file_size"`
	AuthorName string `json:"author_name"`
	WordCount  int    `json:"word_count"`
	ISBN       string `json:"isbn"`
	Publisher  string `json:"publisher"`
	Language   string `json:"language"`
}

func (ContentDocument) TableName() string { return "content_document" }

type ContentCourse struct {
	Id            string `gorm:"primaryKey" json:"id"`
	Duration      int64  `json:"duration"`
	Instructor    string `json:"instructor"`
	SeriesName    string `json:"series_name"`
	EpisodeNumber int    `json:"episode_number"`
	TotalEpisodes int    `json:"total_episodes"`
	PlatformName  string `json:"platform_name"`
	ChapterCount  int    `json:"chapter_count"`
}

func (ContentCourse) TableName() string { return "content_course" }

type ContentComic struct {
	Id               string `gorm:"primaryKey" json:"id"`
	ChapterNumber    int    `json:"chapter_number"`
	VolumeNumber     int    `json:"volume_number"`
	SeriesName       string `json:"series_name"`
	PageCount        int    `json:"page_count"`
	AuthorName       string `json:"author_name"`
	ArtistName       string `json:"artist_name"`
	Format           string `json:"format"`
	IsFinished       int    `json:"is_finished"`
	ReadingDirection string `json:"reading_direction"`
}

func (ContentComic) TableName() string { return "content_comic" }

type ContentPost struct {
	Id           string `gorm:"primaryKey" json:"id"`
	Text         string `gorm:"type:longtext" json:"text"`
	ReplyCount   int64  `json:"reply_count"`
	RepostCount  int64  `json:"repost_count"`
	QuoteCount   int64  `json:"quote_count"`
	Language     string `json:"language"`
	MentionsJSON string `json:"mentions_json"`
	HashtagsJSON string `json:"hashtags_json"`
}

func (ContentPost) TableName() string { return "content_post" }
