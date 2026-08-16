package model

import (
	"strconv"
	"strings"
)

const (
	ContentConversationSourceOfficialExport = "official_export"
	ContentConversationSourceBrowserCapture = "browser_capture"
	ContentConversationSourceAPI            = "api"
	ContentConversationSourceSharePage      = "share_page"
	ContentConversationSourceManualImport   = "manual_import"
)

const (
	ContentConversationMessageRoleUnknown   = "unknown"
	ContentConversationMessageRoleSystem    = "system"
	ContentConversationMessageRoleDeveloper = "developer"
	ContentConversationMessageRoleUser      = "user"
	ContentConversationMessageRoleAssistant = "assistant"
	ContentConversationMessageRoleTool      = "tool"
)

const (
	ContentConversationMessageStatusCompleted  = "completed"
	ContentConversationMessageStatusInProgress = "in_progress"
	ContentConversationMessageStatusIncomplete = "incomplete"
	ContentConversationMessageStatusError      = "error"
)

const (
	ContentConversationPartTypeText             = "text"
	ContentConversationPartTypeMarkdown         = "markdown"
	ContentConversationPartTypeImage            = "image"
	ContentConversationPartTypeAudio            = "audio"
	ContentConversationPartTypeVideo            = "video"
	ContentConversationPartTypeFile             = "file"
	ContentConversationPartTypeCode             = "code"
	ContentConversationPartTypeToolCall         = "tool_call"
	ContentConversationPartTypeToolResult       = "tool_result"
	ContentConversationPartTypeCitation         = "citation"
	ContentConversationPartTypeReasoningSummary = "reasoning_summary"
	ContentConversationPartTypeStructured       = "structured"
	ContentConversationPartTypeLinkPreview      = "link_preview"
	ContentConversationPartTypeCanvas           = "canvas"
	ContentConversationPartTypeArtifact         = "artifact"
	ContentConversationPartTypeRefusal          = "refusal"
)

// ContentConversation archives a conversation from an external platform such
// as ChatGPT or Doubao. The application's own chat_session tables have a
// separate lifecycle and are intentionally not reused here.
type ContentConversation struct {
	Id                   string                       `gorm:"primaryKey" json:"id"`
	SourceType           string                       `gorm:"index:idx_content_conversation_source" json:"source_type"`
	SourceFormat         string                       `json:"source_format"`
	FormatVersion        string                       `json:"format_version"`
	DefaultModelProvider string                       `json:"default_model_provider"`
	DefaultModelName     string                       `json:"default_model_name"`
	CurrentBranchKey     string                       `json:"current_branch_key"`
	MessageCount         int                          `json:"message_count"`
	BranchCount          int                          `json:"branch_count"`
	IsShared             int                          `json:"is_shared"`
	Metadata             string                       `json:"metadata"`
	Branches             []ContentConversationBranch  `gorm:"foreignKey:ConversationId;references:Id" json:"branches,omitempty"`
	Messages             []ContentConversationMessage `gorm:"foreignKey:ConversationId;references:Id" json:"messages,omitempty"`
}

func (ContentConversation) TableName() string { return "content_conversation" }

// ContentConversationBranch records a named or generated path through the
// message tree. Shared ancestors do not need to be duplicated; RootMessageKey
// and LeafMessageKey identify the path that can be reconstructed from parent
// links.
type ContentConversationBranch struct {
	Id             uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationId string `gorm:"not null;index:idx_content_conversation_branch_conversation;uniqueIndex:idx_content_conversation_branch_identity,priority:1" json:"conversation_id"`
	BranchKey      string `gorm:"not null;uniqueIndex:idx_content_conversation_branch_identity,priority:2" json:"branch_key"`
	Title          string `json:"title"`
	RootMessageKey string `json:"root_message_key"`
	LeafMessageKey string `json:"leaf_message_key"`
	IsCurrent      int    `json:"is_current"`
	SortOrder      int    `json:"sort_order"`
	Metadata       string `json:"metadata"`
	Timestamps
}

func (ContentConversationBranch) TableName() string { return "content_conversation_branch" }

// ContentConversationMessage is one stable node in the conversation tree.
// ParentMessageKey preserves edits, regenerated answers, and alternative
// branches without flattening them into one lossy transcript.
type ContentConversationMessage struct {
	Id               uint                             `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationId   string                           `gorm:"not null;index:idx_content_conversation_message_conversation;uniqueIndex:idx_content_conversation_message_identity,priority:1" json:"conversation_id"`
	MessageKey       string                           `gorm:"not null;uniqueIndex:idx_content_conversation_message_identity,priority:2" json:"message_key"`
	ParentMessageKey string                           `gorm:"index:idx_content_conversation_message_parent" json:"parent_message_key"`
	Role             string                           `gorm:"not null;index:idx_content_conversation_message_role" json:"role"`
	AuthorName       string                           `json:"author_name"`
	ModelProvider    string                           `json:"model_provider"`
	ModelName        string                           `json:"model_name"`
	Status           string                           `json:"status"`
	ContentText      string                           `gorm:"type:longtext" json:"content_text"`
	ContentHash      string                           `json:"content_hash"`
	Sequence         int                              `gorm:"index:idx_content_conversation_message_sequence" json:"sequence"`
	SentAt           *int64                           `json:"sent_at"`
	EditedAt         *int64                           `json:"edited_at"`
	Metadata         string                           `json:"metadata"`
	Parts            []ContentConversationMessagePart `gorm:"foreignKey:MessageId;references:Id" json:"parts,omitempty"`
	Assets           []ContentAssetLink               `gorm:"-" json:"assets,omitempty"`
	Timestamps
}

func (ContentConversationMessage) TableName() string { return "content_conversation_message" }

// ContentConversationMessagePart keeps mixed text, media, tool calls,
// citations, Canvas documents, and generated artifacts in their original
// order. SubjectKey is the stable key used by ContentAssetLink.
type ContentConversationMessagePart struct {
	Id             uint               `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationId string             `gorm:"not null;index:idx_content_conversation_message_part_conversation;uniqueIndex:idx_content_conversation_message_part_identity,priority:1;uniqueIndex:idx_content_conversation_message_part_subject_identity,priority:1" json:"conversation_id"`
	MessageId      uint               `gorm:"not null;index:idx_content_conversation_message_part_message" json:"message_id"`
	MessageKey     string             `gorm:"not null;uniqueIndex:idx_content_conversation_message_part_identity,priority:2" json:"message_key"`
	PartKey        string             `gorm:"not null;uniqueIndex:idx_content_conversation_message_part_identity,priority:3" json:"part_key"`
	SubjectKey     string             `gorm:"not null;index:idx_content_conversation_message_part_subject;uniqueIndex:idx_content_conversation_message_part_subject_identity,priority:2" json:"subject_key"`
	SortOrder      int                `json:"sort_order"`
	Type           string             `gorm:"not null;index:idx_content_conversation_message_part_type" json:"type"`
	Text           string             `gorm:"type:longtext" json:"text"`
	URL            string             `json:"url"`
	MIMEType       string             `gorm:"column:mime_type" json:"mime_type"`
	LanguageCode   string             `json:"language_code"`
	ToolCallId     string             `gorm:"column:tool_call_id" json:"tool_call_id"`
	ToolName       string             `json:"tool_name"`
	Metadata       string             `json:"metadata"`
	Assets         []ContentAssetLink `gorm:"-" json:"assets,omitempty"`
	Timestamps
}

func (ContentConversationMessagePart) TableName() string {
	return "content_conversation_message_part"
}

func BuildContentConversationBranchKey(external_id string, leaf_message_key string, sort_order int) string {
	if external_id = strings.TrimSpace(external_id); external_id != "" {
		return "external:" + external_id
	}
	if leaf_message_key = strings.TrimSpace(leaf_message_key); leaf_message_key != "" {
		return "leaf:" + leaf_message_key
	}
	return "idx:" + strconv.Itoa(sort_order)
}

func BuildContentConversationMessageKey(external_id string, sequence int) string {
	if external_id = strings.TrimSpace(external_id); external_id != "" {
		return "external:" + external_id
	}
	return "idx:" + strconv.Itoa(sequence)
}

func BuildContentConversationMessagePartKey(external_id string, sort_order int) string {
	if external_id = strings.TrimSpace(external_id); external_id != "" {
		return "external:" + external_id
	}
	return "idx:" + strconv.Itoa(sort_order)
}

func BuildContentConversationMessagePartSubjectKey(message_key string, part_key string) string {
	message_key = strings.TrimSpace(message_key)
	part_key = strings.TrimSpace(part_key)
	return strconv.Itoa(len(message_key)) + ":" + message_key + ":" + part_key
}

func BuildContentConversationAssetKey(subject_key string, representation_key string) string {
	return strings.TrimSpace(subject_key) + ":" + strings.TrimSpace(representation_key)
}
