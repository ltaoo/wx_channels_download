package model

import (
	"strconv"
	"strings"
)

// Content.Type is intentionally a broad archive family. Platform-specific or
// presentation-specific distinctions belong in Content.Subtype.
const (
	ContentTypeVideo        = "video"
	ContentTypeAudio        = "audio"
	ContentTypeImage        = "image"
	ContentTypeAlbum        = "album"
	ContentTypeArticle      = "article"
	ContentTypePost         = "post"
	ContentTypeNovel        = "novel"
	ContentTypeComic        = "comic"
	ContentTypeDocument     = "document"
	ContentTypeLive         = "live"
	ContentTypePodcast      = "podcast"
	ContentTypeCourse       = "course"
	ContentTypeCollection   = "collection"
	ContentTypeWebpage      = "webpage"
	ContentTypeConversation = "conversation"
	ContentTypeOther        = "other"
)

// Content.Subtype refines a broad archive family without changing the tables
// used to store it. Values are intentionally shared across platforms.
const (
	ContentSubtypeShortVideo     = "short_video"
	ContentSubtypeLongVideo      = "long_video"
	ContentSubtypeMovie          = "movie"
	ContentSubtypeEpisode        = "episode"
	ContentSubtypeClip           = "clip"
	ContentSubtypeLiveReplay     = "live_replay"
	ContentSubtypeMusic          = "music"
	ContentSubtypeAudiobook      = "audiobook"
	ContentSubtypeVoice          = "voice"
	ContentSubtypeRadio          = "radio"
	ContentSubtypePodcastEpisode = "podcast_episode"
	ContentSubtypePhotoAlbum     = "photo_album"
	ContentSubtypeIllustration   = "illustration"
	ContentSubtypeBlog           = "blog"
	ContentSubtypeNews           = "news"
	ContentSubtypeNewsletter     = "newsletter"
	ContentSubtypeQuestion       = "question"
	ContentSubtypeAnswer         = "answer"
	ContentSubtypeWiki           = "wiki"
	ContentSubtypeMicroblog      = "microblog"
	ContentSubtypeThread         = "thread"
	ContentSubtypeComment        = "comment"
	ContentSubtypeEbook          = "ebook"
	ContentSubtypePDF            = "pdf"
	ContentSubtypeSlides         = "slides"
	ContentSubtypeSpreadsheet    = "spreadsheet"
	ContentSubtypeLivestream     = "livestream"
	ContentSubtypeAudioRoom      = "audio_room"
	ContentSubtypePlaylist       = "playlist"
	ContentSubtypeSeries         = "series"
	ContentSubtypeFeed           = "feed"
	ContentSubtypeChat           = "chat"
	ContentSubtypeAIChat         = "ai_chat"
	ContentSubtypeHumanChat      = "human_chat"
	ContentSubtypeEmailThread    = "email_thread"
)

const (
	ContentRelationContains      = "contains"
	ContentRelationPartOf        = "part_of"
	ContentRelationEpisodeOf     = "episode_of"
	ContentRelationAnswerOf      = "answer_of"
	ContentRelationReplyTo       = "reply_to"
	ContentRelationQuoteOf       = "quote_of"
	ContentRelationRepostOf      = "repost_of"
	ContentRelationTranslationOf = "translation_of"
	ContentRelationDerivedFrom   = "derived_from"
	ContentRelationRelated       = "related"
)

// ContentAsset.Kind describes the physical representation of an archive
// asset. It deliberately does not describe why the asset belongs to content.
const (
	ContentAssetKindVideo    = "video"
	ContentAssetKindAudio    = "audio"
	ContentAssetKindImage    = "image"
	ContentAssetKindText     = "text"
	ContentAssetKindDocument = "document"
	ContentAssetKindArchive  = "archive"
	ContentAssetKindManifest = "manifest"
	ContentAssetKindData     = "data"
	ContentAssetKindBinary   = "binary"
)

const (
	ContentAssetSubjectNovelVolume             = "novel_volume"
	ContentAssetSubjectNovelChapter            = "novel_chapter"
	ContentAssetSubjectAlbumImage              = "album_image"
	ContentAssetSubjectLivePhoto               = "live_photo"
	ContentAssetSubjectComicChapter            = "comic_chapter"
	ContentAssetSubjectComicPage               = "comic_page"
	ContentAssetSubjectCourseLesson            = "course_lesson"
	ContentAssetSubjectPodcastEpisode          = "podcast_episode"
	ContentAssetSubjectConversationMessage     = "conversation_message"
	ContentAssetSubjectConversationMessagePart = "conversation_message_part"
)

const (
	ContentAssetSubjectRelationRepresentation = "representation"
	ContentAssetSubjectRelationContains       = "contains"
)

// ContentAsset.Role describes the semantic use of an archive asset.
const (
	ContentAssetRolePrimary            = "primary"
	ContentAssetRoleVideoVariant       = "video_variant"
	ContentAssetRoleAudioVariant       = "audio_variant"
	ContentAssetRoleSubtitle           = "subtitle"
	ContentAssetRoleTranscript         = "transcript"
	ContentAssetRoleLyrics             = "lyrics"
	ContentAssetRoleCover              = "cover"
	ContentAssetRoleThumbnail          = "thumbnail"
	ContentAssetRoleLivePhoto          = "live_photo"
	ContentAssetRoleArticleBody        = "article_body"
	ContentAssetRoleNovelChapter       = "novel_chapter"
	ContentAssetRoleNovelBook          = "novel_book"
	ContentAssetRoleComicPage          = "comic_page"
	ContentAssetRoleMessageAttachment  = "message_attachment"
	ContentAssetRoleGeneratedImage     = "generated_image"
	ContentAssetRoleGeneratedFile      = "generated_file"
	ContentAssetRoleCanvas             = "canvas"
	ContentAssetRoleArtifact           = "artifact"
	ContentAssetRoleCodeOutput         = "code_output"
	ContentAssetRoleConversationExport = "conversation_export"
	ContentAssetRoleAttachment         = "attachment"
	ContentAssetRoleSourceSnapshot     = "source_snapshot"
)

// ContentAsset is the stable identity of one downloadable or generated archive
// artifact. DownloadResource is an execution-time instance and links to this
// record through DownloadResourceAsset instead of through UniqueID.
type ContentAsset struct {
	Id                uint               `gorm:"primaryKey;autoIncrement" json:"id"`
	ContentId         string             `gorm:"not null;index:idx_content_asset_content;uniqueIndex:idx_content_asset_identity,priority:1" json:"content_id"`
	Kind              string             `gorm:"not null;index:idx_content_asset_kind" json:"kind"`
	Role              string             `gorm:"not null;index:idx_content_asset_role;uniqueIndex:idx_content_asset_identity,priority:2" json:"role"`
	AssetKey          string             `gorm:"not null;uniqueIndex:idx_content_asset_identity,priority:3" json:"asset_key"`
	Label             string             `json:"label"`
	LanguageCode      string             `gorm:"column:language_code;index:idx_content_asset_language" json:"language_code"`
	MIMEType          string             `gorm:"column:mime_type" json:"mime_type"`
	Size              int64              `json:"size"`
	SortOrder         int                `gorm:"column:sort_order" json:"sort_order"`
	Metadata          string             `json:"metadata"`
	DownloadResources []DownloadResource `gorm:"many2many:download_resource_asset;foreignKey:Id;joinForeignKey:AssetId;references:Id;joinReferences:ResourceId" json:"download_resources,omitempty"`
	Timestamps
}

func (ContentAsset) TableName() string { return "content_asset" }

// ContentAssetLink binds a root content asset to a stable nested subject. It is
// intentionally polymorphic so every nested archive type does not need another
// join table. Subject keys remain owned and validated by their domain model.
type ContentAssetLink struct {
	ContentId   string       `gorm:"primaryKey;index:idx_content_asset_link_subject,priority:1" json:"content_id"`
	SubjectType string       `gorm:"primaryKey;index:idx_content_asset_link_subject,priority:2" json:"subject_type"`
	SubjectKey  string       `gorm:"primaryKey;index:idx_content_asset_link_subject,priority:3" json:"subject_key"`
	AssetId     uint         `gorm:"primaryKey;autoIncrement:false;index:idx_content_asset_link_asset" json:"asset_id"`
	Relation    string       `gorm:"primaryKey;not null" json:"relation"`
	Asset       ContentAsset `gorm:"foreignKey:AssetId;references:Id" json:"asset"`
	CreatedAt   int64        `json:"created_at"`
}

func (ContentAssetLink) TableName() string { return "content_asset_link" }

// ContentRelation links top-level archive items without forcing series,
// playlists, threads, replies, and translations into platform-specific JSON.
// The direction is SourceContentId --Type--> TargetContentId.
type ContentRelation struct {
	SourceContentId string `gorm:"primaryKey;index:idx_content_relation_source" json:"source_content_id"`
	TargetContentId string `gorm:"primaryKey;index:idx_content_relation_target" json:"target_content_id"`
	Type            string `gorm:"primaryKey;index:idx_content_relation_type" json:"type"`
	SortOrder       int    `json:"sort_order"`
	Metadata        string `json:"metadata"`
	CreatedAt       int64  `json:"created_at"`
}

func (ContentRelation) TableName() string { return "content_relation" }

// BuildContentVideoSubtitleAssetKey returns the catalog key shared by a
// subtitle source and its DownloadResource content asset reference.
func BuildContentVideoSubtitleAssetKey(track_key string, source_key string) string {
	return strings.TrimSpace(track_key) + ":" + strings.TrimSpace(source_key)
}

// BuildContentNovelChapterKey returns a stable chapter key. Platform chapter
// IDs should be supplied as external_id. Source URL is the next-best identity;
// sequence index is only a compatibility fallback because it can change.
func BuildContentNovelChapterKey(external_id string, source_url string, idx int) string {
	if external_id = strings.TrimSpace(external_id); external_id != "" {
		return "external:" + external_id
	}
	if source_url = strings.TrimSpace(source_url); source_url != "" {
		return "url:" + source_url
	}
	return "idx:" + strconv.Itoa(idx)
}

func BuildContentNovelVolumeKey(external_id string, idx int) string {
	if external_id = strings.TrimSpace(external_id); external_id != "" {
		return "external:" + external_id
	}
	return "idx:" + strconv.Itoa(idx)
}

func BuildContentNovelChapterAssetKey(chapter_key string, representation_key string) string {
	return strings.TrimSpace(chapter_key) + ":" + strings.TrimSpace(representation_key)
}

func BuildContentAlbumImageKey(external_id string, source_url string, sort_order int) string {
	if external_id = strings.TrimSpace(external_id); external_id != "" {
		return "external:" + external_id
	}
	if source_url = strings.TrimSpace(source_url); source_url != "" {
		return "url:" + source_url
	}
	return "idx:" + strconv.Itoa(sort_order)
}

func BuildContentAlbumImageAssetKey(image_key string, representation_key string) string {
	return strings.TrimSpace(image_key) + ":" + strings.TrimSpace(representation_key)
}
