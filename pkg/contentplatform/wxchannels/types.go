package wxchannels

import (
	contentdownload "wx_channel/pkg/contentplatform/download"
	channelspkg "wx_channel/pkg/scraper/wxchannels"
)

const (
	ContentTypeVideo      = "video"
	ContentTypeImageAlbum = "image_album"
)

// FeedURLContentEnvelope is the metadata-only content envelope returned for a
// Channels feed URL before a frontend fetcher has resolved the full object.
type FeedURLContentEnvelope struct {
	Summary  contentdownload.ContentSummary `json:"summary,omitempty"`
	Data     FeedURLContent                 `json:"data,omitempty"`
	Metadata FeedURLMetadata                `json:"metadata,omitempty"`
}

func NewFeedURLContentEnvelope(summary contentdownload.ContentSummary, data FeedURLContent, metadata FeedURLMetadata) *FeedURLContentEnvelope {
	return &FeedURLContentEnvelope{
		Summary:  summary,
		Data:     data,
		Metadata: metadata,
	}
}

func (c *FeedURLContentEnvelope) ContentSummary() contentdownload.ContentSummary {
	if c == nil {
		return contentdownload.ContentSummary{}
	}
	return c.Summary
}

func (c *FeedURLContentEnvelope) ContentData() any {
	if c == nil {
		return nil
	}
	return c.Data
}

func (c *FeedURLContentEnvelope) ContentMetadata() map[string]any {
	if c == nil {
		return nil
	}
	return c.Metadata.Map()
}

func (c *FeedURLContentEnvelope) ContentOutput() map[string]any {
	return nil
}

// FeedURLContent is the JSON content.data payload for an unresolved Channels
// feed URL.
type FeedURLContent struct {
	URL string `json:"url"`
}

// FeedURLMetadata is the JSON content.metadata payload for an unresolved
// Channels feed URL.
type FeedURLMetadata struct {
	OID string `json:"oid"`
	NID string `json:"nid"`
	EID string `json:"eid"`
}

func (m FeedURLMetadata) Map() map[string]any {
	return map[string]any{
		"oid": m.OID,
		"nid": m.NID,
		"eid": m.EID,
	}
}

// FeedContentEnvelope is the complete shared content envelope returned for a
// resolved Channels feed object. Data is the raw ChannelsObject used by Resolve
// to derive video, image-album, cover, and audio variants.
type FeedContentEnvelope struct {
	Summary  contentdownload.ContentSummary `json:"summary,omitempty"`
	Data     channelspkg.ChannelsObject     `json:"data,omitempty"`
	Metadata FeedMetadata                   `json:"metadata,omitempty"`
}

func NewFeedContentEnvelope(summary contentdownload.ContentSummary, data channelspkg.ChannelsObject, metadata FeedMetadata) *FeedContentEnvelope {
	return &FeedContentEnvelope{
		Summary:  summary,
		Data:     data,
		Metadata: metadata,
	}
}

func (c *FeedContentEnvelope) ContentSummary() contentdownload.ContentSummary {
	if c == nil {
		return contentdownload.ContentSummary{}
	}
	return c.Summary
}

func (c *FeedContentEnvelope) ContentData() any {
	if c == nil {
		return nil
	}
	return c.Data
}

func (c *FeedContentEnvelope) ContentMetadata() map[string]any {
	if c == nil {
		return nil
	}
	return c.Metadata.Map()
}

func (c *FeedContentEnvelope) ContentOutput() map[string]any {
	return nil
}

// FeedMetadata is the JSON content.metadata payload for a resolved Channels
// feed object.
type FeedMetadata struct {
	OID               string `json:"oid"`
	NID               string `json:"nid"`
	EID               string `json:"eid"`
	NonceID           string `json:"nonce_id"`
	AuthorAvatarURL   string `json:"author_avatar_url,omitempty"`
	AuthorHomepageURL string `json:"author_homepage_url,omitempty"`
	SourceURL         string `json:"source_url"`
}

func (m FeedMetadata) Map() map[string]any {
	return map[string]any{
		"oid":                 m.OID,
		"nid":                 m.NID,
		"eid":                 m.EID,
		"nonce_id":            m.NonceID,
		"author_avatar_url":   m.AuthorAvatarURL,
		"author_homepage_url": m.AuthorHomepageURL,
		"source_url":          m.SourceURL,
	}
}

// SphURLContentEnvelope is the metadata-only content envelope returned for a
// Channels sph share URL before the sph profile is resolved.
type SphURLContentEnvelope struct {
	Summary  contentdownload.ContentSummary `json:"summary,omitempty"`
	Data     SphProfile                     `json:"data,omitempty"`
	Metadata SphURLMetadata                 `json:"metadata,omitempty"`
}

func NewSphURLContentEnvelope(summary contentdownload.ContentSummary, data SphProfile, metadata SphURLMetadata) *SphURLContentEnvelope {
	return &SphURLContentEnvelope{
		Summary:  summary,
		Data:     data,
		Metadata: metadata,
	}
}

func (c *SphURLContentEnvelope) ContentSummary() contentdownload.ContentSummary {
	if c == nil {
		return contentdownload.ContentSummary{}
	}
	return c.Summary
}

func (c *SphURLContentEnvelope) ContentData() any {
	if c == nil {
		return nil
	}
	return c.Data
}

func (c *SphURLContentEnvelope) ContentMetadata() map[string]any {
	if c == nil {
		return nil
	}
	return c.Metadata.Map()
}

func (c *SphURLContentEnvelope) ContentOutput() map[string]any {
	return nil
}

// SphURLMetadata is the JSON content.metadata payload for an unresolved
// Channels sph share URL.
type SphURLMetadata struct {
	SphID    string `json:"sph_id"`
	ShareURL string `json:"share_url"`
}

func (m SphURLMetadata) Map() map[string]any {
	return map[string]any{
		"sph_id":    m.SphID,
		"share_url": m.ShareURL,
	}
}

// SphContentEnvelope is the complete shared content envelope returned for a
// resolved Channels sph video profile.
type SphContentEnvelope struct {
	Summary  contentdownload.ContentSummary `json:"summary,omitempty"`
	Data     SphProfile                     `json:"data,omitempty"`
	Metadata SphMetadata                    `json:"metadata,omitempty"`
}

func NewSphContentEnvelope(summary contentdownload.ContentSummary, data SphProfile, metadata SphMetadata) *SphContentEnvelope {
	return &SphContentEnvelope{
		Summary:  summary,
		Data:     data,
		Metadata: metadata,
	}
}

func (c *SphContentEnvelope) ContentSummary() contentdownload.ContentSummary {
	if c == nil {
		return contentdownload.ContentSummary{}
	}
	return c.Summary
}

func (c *SphContentEnvelope) ContentData() any {
	if c == nil {
		return nil
	}
	return c.Data
}

func (c *SphContentEnvelope) ContentMetadata() map[string]any {
	if c == nil {
		return nil
	}
	return c.Metadata.Map()
}

func (c *SphContentEnvelope) ContentOutput() map[string]any {
	return nil
}

// SphProfile is the JSON content.data payload for Channels sph videos.
type SphProfile = channelspkg.SphProfile

// SphMetadata is the JSON content.metadata payload for a resolved Channels sph
// profile.
type SphMetadata struct {
	SphID             string `json:"sph_id"`
	ExportID          string `json:"export_id"`
	ShareURL          string `json:"share_url"`
	AuthorAvatarURL   string `json:"author_avatar_url,omitempty"`
	AuthorHomepageURL string `json:"author_homepage_url,omitempty"`
	SourceURL         string `json:"source_url"`
}

func (m SphMetadata) Map() map[string]any {
	return map[string]any{
		"sph_id":              m.SphID,
		"export_id":           m.ExportID,
		"share_url":           m.ShareURL,
		"author_avatar_url":   m.AuthorAvatarURL,
		"author_homepage_url": m.AuthorHomepageURL,
		"source_url":          m.SourceURL,
	}
}

// ProbeOutput is reserved for WeChat Channels-specific probe output. Channels
// currently stores probe metadata in the shared content summary and typed
// content metadata.
type ProbeOutput struct{}

func (ProbeOutput) Map() map[string]any {
	return nil
}
