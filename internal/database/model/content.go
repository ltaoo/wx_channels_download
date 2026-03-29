package model

type Content struct {
	Id             int    `gorm:"primaryKey;autoIncrement" json:"id"`
	PlatformId     string `gorm:"not null;index:idx_content_platform_type,priority:1;index:idx_content_external_id,priority:1" json:"platform_id"`
	ContentType    string `gorm:"not null;index:idx_content_platform_type,priority:2;index:idx_content_type" json:"content_type"`
	ExternalId     string `gorm:"not null;index:idx_content_external_id,priority:2" json:"external_id"`
	ExternalId2    string `json:"external_id2"`
	ExternalId3    string `json:"external_id3"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	ContentURL     string `json:"content_url"`
	URL            string `json:"url"`
	SourceURL      string `json:"source_url"`
	CoverURL       string `json:"cover_url"`
	CoverWidth     string `json:"cover_width"`
	CoverHeight    string `json:"cover_height"`
	Metadata       string `json:"metadata"`
	PublishTime    *int64 `json:"publish_time"`
	UpdateTime     *int64 `json:"update_time"`
	IsOriginal     int    `json:"is_original"`
	IsPrivate      int    `json:"is_private"`
	ViewCount      int64  `json:"view_count"`
	PlayTimes      int64  `json:"play_times"`
	LikeCount      int64  `json:"like_count"`
	CommentCount   int64  `json:"comment_count"`
	ShareCount     int64  `json:"share_count"`
	CollectCount   int64  `json:"collect_count"`
	DownloadTaskId *int   `json:"download_task_id"`
	DownloadStatus int    `json:"download_status"`
	DownloadPath   string `json:"download_path"`
	FileSize       int64  `json:"file_size"`
	Size           int64  `json:"size"`
	Duration       int64  `json:"duration"`
	DownloadTime   *int64 `json:"download_time"`
	ErrorMsg       string `json:"error_msg"`
	Unread         int    `json:"unread"`
	SourceDeleted  int    `json:"source_deleted"`
	Validated      int    `json:"validated"`
	Tags           string `json:"tags"`
	Category       string `json:"category"`
	ExtraData      string `json:"extra_data"`
	Timestamps
}

func (Content) TableName() string { return "content" }

type ContentVideo struct {
	ContentId       int    `gorm:"primaryKey" json:"content_id"`
	Duration        int64  `json:"duration"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	FPS             int    `json:"fps"`
	Bitrate         int    `json:"bitrate"`
	Codec           string `json:"codec"`
	Format          string `json:"format"`
	HasSubtitle     int    `json:"has_subtitle"`
	SubtitleURL     string `json:"subtitle_url"`
	AudioTrackCount int    `json:"audio_track_count"`
	NonceId         string `json:"nonce_id"`
	DecodeKey       string `json:"decode_key"`
	DeletedAt       *int64 `gorm:"column:deleted_at;index" json:"deleted_at"`
}

func (ContentVideo) TableName() string { return "content_video" }

type ContentImage struct {
	ContentId  int    `gorm:"primaryKey" json:"content_id"`
	ImageCount int    `json:"image_count"`
	Images     string `json:"images"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Format     string `json:"format"`
	IsGIF      int    `json:"is_gif"`
	DeletedAt  *int64 `gorm:"column:deleted_at;index" json:"deleted_at"`
}

func (ContentImage) TableName() string { return "content_image" }

type ContentAudio struct {
	ContentId     int    `gorm:"primaryKey" json:"content_id"`
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
	ContentId       int    `gorm:"primaryKey" json:"content_id"`
	WordCount       int    `json:"word_count"`
	ReadingTime     int    `json:"reading_time"`
	ContentText     string `json:"content_text"`
	ContentHTML     string `json:"content_html"`
	ContentMarkdown string `json:"content_markdown"`
	ChapterNumber   int    `json:"chapter_number"`
	VolumeNumber    int    `json:"volume_number"`
	SeriesName      string `json:"series_name"`
	IsFinished      int    `json:"is_finished"`
	AuthorName      string `json:"author_name"`
	PublishPlatform string `json:"publish_platform"`
}

func (ContentArticle) TableName() string { return "content_article" }

type ContentAccount struct {
	ContentId int    `gorm:"primaryKey;index:idx_content_account_account" json:"content_id"`
	AccountId int    `gorm:"primaryKey;index:idx_content_account_account" json:"account_id"`
	Role      string `json:"role"`
	CreatedAt int64  `json:"created_at"`
}

func (ContentAccount) TableName() string { return "content_account" }

type ContentInfluencer struct {
	ContentId    int    `gorm:"primaryKey;index:idx_content_influencer_influencer" json:"content_id"`
	InfluencerId int    `gorm:"primaryKey;index:idx_content_influencer_influencer" json:"influencer_id"`
	Role         string `json:"role"`
	CreatedAt    int64  `json:"created_at"`
}

func (ContentInfluencer) TableName() string { return "content_influencer" }
