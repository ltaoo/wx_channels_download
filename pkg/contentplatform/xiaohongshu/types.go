package xiaohongshu

import (
	contentdownload "wx_channel/pkg/contentplatform/download"
	xhspkg "wx_channel/pkg/xiaohongshu"
)

const (
	PlatformID = xhspkg.PlatformID

	ContentTypeVideo      = "video"
	ContentTypeImage      = "image"
	ContentTypeImageAlbum = "image_album"

	OutputFormatJSON = "json"
)

type NoteContentEnvelope struct {
	Summary  contentdownload.ContentSummary `json:"summary,omitempty"`
	Data     xhspkg.Note                    `json:"data,omitempty"`
	Metadata NoteMetadata                   `json:"metadata,omitempty"`
	Output   NoteOutput                     `json:"output,omitempty"`
}

func NewNoteContentEnvelope(summary contentdownload.ContentSummary, data xhspkg.Note, metadata NoteMetadata, output NoteOutput) *NoteContentEnvelope {
	return &NoteContentEnvelope{
		Summary:  summary,
		Data:     data,
		Metadata: metadata,
		Output:   output,
	}
}

func (c *NoteContentEnvelope) ContentSummary() contentdownload.ContentSummary {
	if c == nil {
		return contentdownload.ContentSummary{}
	}
	return c.Summary
}

func (c *NoteContentEnvelope) ContentData() any {
	if c == nil {
		return nil
	}
	return c.Data
}

func (c *NoteContentEnvelope) ContentMetadata() map[string]any {
	if c == nil {
		return nil
	}
	return c.Metadata.Map()
}

func (c *NoteContentEnvelope) ContentOutput() map[string]any {
	if c == nil {
		return nil
	}
	return c.Output.Map()
}

type NoteMetadata struct {
	NoteID          string `json:"note_id"`
	XSecToken       string `json:"xsec_token,omitempty"`
	AuthorID        string `json:"author_id,omitempty"`
	AuthorXSecToken string `json:"author_xsec_token,omitempty"`
	SourceURL       string `json:"source_url,omitempty"`
	CanonicalURL    string `json:"canonical_url,omitempty"`
	PublishedAt     int64  `json:"published_at,omitempty"`
	LastUpdateTime  int64  `json:"last_update_time,omitempty"`
}

func (m NoteMetadata) Map() map[string]any {
	return map[string]any{
		"note_id":             m.NoteID,
		"xsec_token":          m.XSecToken,
		"author_id":           m.AuthorID,
		"author_xsec_token":   m.AuthorXSecToken,
		"source_url":          m.SourceURL,
		"canonical_url":       m.CanonicalURL,
		"published_at":        m.PublishedAt,
		"last_update_time":    m.LastUpdateTime,
		"account_external_id": m.AuthorID,
	}
}

type NoteOutput struct {
	Format       string   `json:"format"`
	ContentType  string   `json:"content_type"`
	NoteID       string   `json:"note_id"`
	Title        string   `json:"title"`
	SourceURL    string   `json:"source_url"`
	CanonicalURL string   `json:"canonical_url"`
	VideoURL     string   `json:"video_url,omitempty"`
	ImageURLs    []string `json:"image_urls,omitempty"`
	CoverURL     string   `json:"cover_url,omitempty"`
}

func (o NoteOutput) Map() map[string]any {
	out := map[string]any{
		"format":        o.Format,
		"content_type":  o.ContentType,
		"note_id":       o.NoteID,
		"title":         o.Title,
		"source_url":    o.SourceURL,
		"canonical_url": o.CanonicalURL,
	}
	if o.VideoURL != "" {
		out["video_url"] = o.VideoURL
	}
	if len(o.ImageURLs) > 0 {
		out["image_urls"] = o.ImageURLs
	}
	if o.CoverURL != "" {
		out["cover_url"] = o.CoverURL
	}
	return out
}
