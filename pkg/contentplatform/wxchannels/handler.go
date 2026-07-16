package wxchannels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
	channelspkg "wx_channel/pkg/scraper/wxchannels"
	wxchelpers "wx_channel/internal/webcontent/wxchannels"
)

const PlatformID = "wx_channels"

type FeedProfileFetcher interface {
	FetchChannelsFeedProfile(oid, nid, reqURL, eid string) (*channelspkg.ChannelsFeedProfileResp, error)
}

type SphProfileFetcher interface {
	FetchChannelsSphProfile(reqURL string) (*SphProfile, error)
}

type Handler struct {
	Fetcher FeedProfileFetcher
}

func New(fetcher FeedProfileFetcher) *Handler {
	return &Handler{Fetcher: fetcher}
}

func (h *Handler) Platform() string {
	return PlatformID
}

func (h *Handler) Match(rawURL string) bool {
	if _, err := ParseFeedURL(rawURL); err == nil {
		return true
	}
	if _, err := ParseSphShareURL(rawURL); err == nil {
		return true
	}
	return false
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	if _, err := ParseSphShareURL(input.URL); err == nil {
		return h.probeSph(ctx, input)
	}

	parts, err := ParseFeedURL(input.URL)
	if err != nil {
		return nil, err
	}

	probe := &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    input.URL,
		CanonicalURL: parts.URL,
		ContentID:    parts.Oid,
		Content: NewFeedURLContentEnvelope(
			contentdownload.ContentSummary{
				Platform:  PlatformID,
				ID:        parts.Oid,
				SourceURL: input.URL,
			},
			FeedURLContent{URL: input.URL},
			FeedURLMetadata{OID: parts.Oid, NID: parts.Nid, EID: parts.Eid},
		),
		Defaults: contentdownload.Defaults{
			VariantID: "original",
			Suffix:    ".mp4",
		},
		Variants: []contentdownload.Variant{
			{ID: "original", Type: "video", Label: "默认/原始", Suffix: ".mp4"},
		},
		Internal: map[string]any{},
	}

	if h.Fetcher == nil {
		probe.Warnings = append(probe.Warnings, "channels fetcher is nil; returning url metadata only")
		return probe, nil
	}

	resp, err := h.Fetcher.FetchChannelsFeedProfile(parts.Oid, parts.Nid, input.URL, parts.Eid)
	if err != nil {
		return nil, fmt.Errorf("fetch channels feed profile: %w", err)
	}
	if resp.ErrCode != 0 {
		return nil, fmt.Errorf("fetch channels feed profile: %s", resp.ErrMsg)
	}

	obj := resp.Data.Object

	isPicture := obj.Type == "picture" || obj.ObjectDesc.MediaType == 2
	contentType := ContentTypeVideo
	if isPicture {
		contentType = ContentTypeImageAlbum
	}

	title := wxchelpers.ObjectTitle(&obj)
	url := wxchelpers.ObjectURL(&obj)
	coverURL := ""
	duration := 0
	nonceID := obj.ObjectNonceId
	decryptKey := ""
	authorNickname := obj.Contact.Nickname
	authorAvatarURL := obj.Contact.HeadUrl
	username := obj.Contact.Username

	if obj.LiveInfo != nil {
		if obj.AnchorContact != nil {
			coverURL = obj.AnchorContact.CoverImgUrl
			username = obj.AnchorContact.Username
			authorNickname = obj.AnchorContact.Nickname
			authorAvatarURL = obj.AnchorContact.HeadUrl
		}
		if coverURL == "" && len(obj.ObjectDesc.Media) > 0 {
			coverURL = obj.ObjectDesc.Media[0].CoverUrl
		}
	} else if len(obj.ObjectDesc.Media) > 0 {
		media := obj.ObjectDesc.Media[0]
		coverURL = media.CoverUrl
		duration = media.VideoPlayLen
		decryptKey = media.DecodeKey
	}

	authorHomepageURL := channelsAuthorProfileURL(username, obj.SourceURL)
	probe.ContentID = obj.ID
	probe.Content = NewFeedContentEnvelope(
		contentdownload.ContentSummary{
			Platform:        PlatformID,
			Type:            contentType,
			ID:              obj.ID,
			Title:           title,
			Author:          authorNickname,
			URL:             url,
			SourceURL:       obj.SourceURL,
			AuthorNickname:  authorNickname,
			AuthorAvatarURL: authorAvatarURL,
			CoverURL:        coverURL,
			Duration:        int64(duration),
		},
		obj,
		FeedMetadata{
			OID:               parts.Oid,
			NID:               parts.Nid,
			EID:               parts.Eid,
			NonceID:           nonceID,
			AuthorAvatarURL:   authorAvatarURL,
			AuthorHomepageURL: authorHomepageURL,
			SourceURL:         obj.SourceURL,
		},
	)
	probe.Internal["decode_key"] = decryptKey
	probe.Internal["pagejson"] = resp

	if isPicture {
		probe.Variants = []contentdownload.Variant{
			{ID: "pictures", Type: "archive", Label: "图集", Suffix: ".zip"},
		}
		probe.Defaults.VariantID = "pictures"
		probe.Defaults.Suffix = ".zip"
		return probe, nil
	}

	if len(obj.ObjectDesc.Media) == 0 {
		return probe, nil
	}

	media := obj.ObjectDesc.Media[0]
	probe.Variants = append(probe.Variants, contentdownload.Variant{
		ID:     "original",
		Type:   "video",
		Label:  "默认/原始",
		Suffix: ".mp4",
		Size:   int64(media.FileSize),
		Width:  int(media.Width),
		Height: int(media.Height),
	})
	for _, spec := range media.Spec {
		id := strings.TrimSpace(spec.FileFormat)
		if id == "" {
			continue
		}
		probe.Variants = append(probe.Variants, contentdownload.Variant{
			ID:     id,
			Type:   "video",
			Label:  id,
			Spec:   id,
			Suffix: ".mp4",
			Width:  int(spec.Width),
			Height: int(spec.Height),
		})
	}
	probe.Variants = append(probe.Variants,
		contentdownload.Variant{ID: "audio_mp3", Type: "audio", Label: "MP3", Suffix: ".mp3", Requires: []string{"ffmpeg"}},
		contentdownload.Variant{ID: "cover", Type: "image", Label: "封面", Suffix: ".jpg"},
	)
	return probe, nil
}

func (h *Handler) probeSph(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	parts, err := ParseSphShareURL(input.URL)
	if err != nil {
		return nil, err
	}
	probe := &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    input.URL,
		CanonicalURL: parts.URL,
		ContentID:    parts.ID,
		Content: NewSphURLContentEnvelope(
			contentdownload.ContentSummary{
				Platform:  PlatformID,
				Type:      ContentTypeVideo,
				ID:        parts.ID,
				SourceURL: input.URL,
			},
			SphProfile{ShareURL: input.URL, SphID: parts.ID},
			SphURLMetadata{SphID: parts.ID, ShareURL: input.URL},
		),
		Defaults: contentdownload.Defaults{
			VariantID: "original",
			Suffix:    ".mp4",
		},
		Variants: []contentdownload.Variant{
			{ID: "original", Type: "video", Label: "默认/原始", Suffix: ".mp4"},
		},
		Internal: map[string]any{"sph": true},
	}

	fetcher, ok := h.Fetcher.(SphProfileFetcher)
	if h.Fetcher == nil || !ok {
		probe.Warnings = append(probe.Warnings, "channels sph fetcher is unavailable; returning url metadata only")
		return probe, nil
	}

	profile, err := fetcher.FetchChannelsSphProfile(input.URL)
	if err != nil {
		return nil, fmt.Errorf("fetch channels sph profile: %w", err)
	}
	if profile == nil {
		return nil, fmt.Errorf("fetch channels sph profile: empty response")
	}
	if profile.ErrCode != 0 {
		return nil, fmt.Errorf("fetch channels sph profile: %s", profile.ErrMsg)
	}

	profile.ShareURL = firstNonEmpty(profile.ShareURL, input.URL)
	profile.SphID = firstNonEmpty(profile.SphID, parts.ID)
	profile.OriginVideoURL = firstNonEmpty(profile.OriginVideoURL, cleanSphVideoURL(profile.VideoURL))
	if profile.OriginVideoURL == "" {
		return nil, fmt.Errorf("fetch channels sph profile: video url is empty")
	}

	contentID := firstNonEmpty(profile.ExportID, profile.SphID, parts.ID)
	title := firstNonEmpty(profile.Description, profile.AuthorNickname, contentID)
	authorHomepageURL := firstNonEmpty(profile.ShareURL, input.URL)
	probe.ContentID = contentID
	probe.Content = NewSphContentEnvelope(
		contentdownload.ContentSummary{
			Platform:        PlatformID,
			Type:            ContentTypeVideo,
			ID:              contentID,
			Title:           title,
			Description:     profile.Description,
			Author:          profile.AuthorNickname,
			URL:             profile.OriginVideoURL,
			SourceURL:       input.URL,
			AuthorNickname:  profile.AuthorNickname,
			AuthorAvatarURL: profile.AuthorAvatarURL,
			CoverURL:        profile.CoverURL,
		},
		*profile,
		SphMetadata{
			SphID:             profile.SphID,
			ExportID:          profile.ExportID,
			ShareURL:          profile.ShareURL,
			AuthorAvatarURL:   profile.AuthorAvatarURL,
			AuthorHomepageURL: authorHomepageURL,
			SourceURL:         input.URL,
		},
	)
	probe.Internal["pagejson"] = profile
	probe.Variants = []contentdownload.Variant{
		{ID: "original", Type: "video", Label: "默认/原始", Suffix: ".mp4"},
		{ID: "audio_mp3", Type: "audio", Label: "MP3", Suffix: ".mp3", Requires: []string{"ffmpeg"}},
	}
	if profile.CoverURL != "" {
		probe.Variants = append(probe.Variants, contentdownload.Variant{ID: "cover", Type: "image", Label: "封面", Suffix: ".jpg"})
	}
	return probe, nil
}

func (h *Handler) Resolve(ctx context.Context, input contentdownload.ResolveInput) (*contentdownload.ResolvedRequest, error) {
	probe := input.Probe
	if probe == nil {
		var err error
		probe, err = h.Probe(ctx, contentdownload.ProbeInput{URL: input.URL, Extra: input.Extra})
		if err != nil {
			return nil, err
		}
	}
	if h.Fetcher == nil && contentdownload.ContentDataOf(probe.Content) == nil {
		return nil, contentdownload.ErrResolveUnavailable
	}

	variant, err := contentdownload.SelectVariant(probe, input.Options)
	if err != nil {
		return nil, err
	}

	if sph, ok := contentdownload.ContentDataOf(probe.Content).(SphProfile); ok {
		return h.resolveSph(ctx, input, probe, variant, sph)
	}

	obj, _ := contentdownload.ContentDataOf(probe.Content).(channelspkg.ChannelsObject)
	if obj.ID == "" {
		return nil, contentdownload.ErrResolveUnavailable
	}

	title := wxchelpers.ObjectTitle(&obj)
	objectURL := wxchelpers.ObjectURL(&obj)
	coverURL := ""
	username := obj.Contact.Username
	if obj.LiveInfo != nil && obj.AnchorContact != nil {
		username = obj.AnchorContact.Username
		coverURL = obj.AnchorContact.CoverImgUrl
	}
	if coverURL == "" && len(obj.ObjectDesc.Media) > 0 {
		coverURL = obj.ObjectDesc.Media[0].CoverUrl
	}
	authorHomepageURL := channelsAuthorProfileURL(username, obj.SourceURL)

	content := probe.Content
	filename := firstNonEmpty(input.Options.Filename, contentdownload.ContentTitle(content), probe.ContentID)
	suffix := firstNonEmpty(input.Options.Suffix, variant.Suffix, ".mp4")
	spec := firstNonEmpty(input.Options.Spec, variant.Spec)
	downloadURL := objectURL
	protocol := "http"

	isPicture := obj.Type == "picture" || obj.ObjectDesc.MediaType == 2
	if isPicture {
		suffix = ".zip"
		files := obj.Files
		if len(files) == 0 {
			files = obj.ObjectDesc.Media
		}
		var items []map[string]string
		for i, f := range files {
			items = append(items, map[string]string{
				"url":      f.URL + f.URLToken,
				"filename": fmt.Sprintf("%d.jpg", i+1),
			})
		}
		data, _ := json.Marshal(items)
		downloadURL = "zip://weixin.qq.com?files=" + url.QueryEscape(string(data))
		protocol = "zip"
	} else if variant.ID == "cover" {
		suffix = ".jpg"
		downloadURL = coverURL
	} else {
		if len(obj.ObjectDesc.Media) == 0 {
			return nil, fmt.Errorf("channels media is empty")
		}
		media := obj.ObjectDesc.Media[0]
		downloadURL = media.URL + media.URLToken
		if spec != "" {
			downloadURL += "&X-snsvideoflag=" + spec
		} else if u, err := url.Parse(downloadURL); err == nil {
			filekey := u.Query().Get("encfilekey")
			token := u.Query().Get("token")
			if filekey != "" && token != "" {
				downloadURL = u.Scheme + "://" + u.Host + u.Path + "?encfilekey=" + filekey + "&token=" + token
			}
		}
	}

	key := "0"
	if len(obj.ObjectDesc.Media) > 0 && obj.ObjectDesc.Media[0].DecodeKey != "" {
		key = obj.ObjectDesc.Media[0].DecodeKey
	}

	resolved := &contentdownload.ResolvedRequest{
		Platform:     PlatformID,
		SourceURL:    firstNonEmpty(probe.SourceURL, input.URL),
		CanonicalURL: probe.CanonicalURL,
		ContentID:    obj.ID,
		Title:        firstNonEmpty(title, contentdownload.ContentTitle(content)),
		Filename:     filename,
		Suffix:       suffix,
		Download: contentdownload.DownloadSpec{
			URL:         downloadURL,
			Method:      "GET",
			Protocol:    protocol,
			Connections: 4,
		},
		Labels: map[string]string{
			"platform":   PlatformID,
			"id":         obj.ID,
			"nonce_id":   obj.ObjectNonceId,
			"title":      title,
			"key":        key,
			"spec":       spec,
			"suffix":     suffix,
			"source_url": obj.SourceURL,
		},
		Metadata: map[string]any{
			"variant_id":          variant.ID,
			"decode_key":          key,
			"author_avatar_url":   contentdownload.ContentAuthorAvatarURL(content),
			"author_homepage_url": authorHomepageURL,
		},
		Content: channelsContentWithSummary(content, contentdownload.ContentSummary{
			Platform:        PlatformID,
			Type:            contentdownload.ContentType(content),
			ID:              obj.ID,
			Title:           title,
			URL:             objectURL,
			SourceURL:       obj.SourceURL,
			Author:          contentdownload.ContentAuthor(content),
			AuthorNickname:  contentdownload.ContentAuthorNickname(content),
			AuthorAvatarURL: contentdownload.ContentAuthorAvatarURL(content),
			CoverURL:        coverURL,
			Duration:        contentdownload.ContentDuration(content),
		}),
	}
	plan, err := h.Plan(ctx, resolved)
	if err != nil {
		return nil, err
	}
	resolved.Pipeline = plan
	return resolved, nil
}

func (h *Handler) resolveSph(ctx context.Context, input contentdownload.ResolveInput, probe *contentdownload.Probe, variant *contentdownload.Variant, sph SphProfile) (*contentdownload.ResolvedRequest, error) {
	content := probe.Content
	contentID := firstNonEmpty(probe.ContentID, sph.ExportID, sph.SphID)
	filename := firstNonEmpty(input.Options.Filename, contentdownload.ContentTitle(content), contentID)
	suffix := firstNonEmpty(input.Options.Suffix, variant.Suffix, ".mp4")
	downloadURL := firstNonEmpty(sph.OriginVideoURL, cleanSphVideoURL(sph.VideoURL), sph.VideoURL)

	if variant.ID == "cover" {
		suffix = ".jpg"
		downloadURL = sph.CoverURL
	}
	if downloadURL == "" {
		return nil, fmt.Errorf("channels sph media url is empty")
	}

	resolved := &contentdownload.ResolvedRequest{
		Platform:     PlatformID,
		SourceURL:    firstNonEmpty(probe.SourceURL, input.URL),
		CanonicalURL: probe.CanonicalURL,
		ContentID:    contentID,
		Title:        firstNonEmpty(contentdownload.ContentTitle(content), sph.Description, contentID),
		Filename:     filename,
		Suffix:       suffix,
		Download: contentdownload.DownloadSpec{
			URL:         downloadURL,
			Method:      "GET",
			Protocol:    "http",
			Connections: 4,
		},
		Labels: map[string]string{
			"platform":   PlatformID,
			"id":         contentID,
			"sph_id":     sph.SphID,
			"export_id":  sph.ExportID,
			"title":      firstNonEmpty(sph.Description, contentdownload.ContentTitle(content)),
			"key":        "0",
			"suffix":     suffix,
			"source_url": firstNonEmpty(sph.ShareURL, probe.SourceURL, input.URL),
		},
		Metadata: map[string]any{
			"variant_id":          variant.ID,
			"decode_key":          "0",
			"sph":                 true,
			"author_avatar_url":   firstNonEmpty(sph.AuthorAvatarURL, contentdownload.ContentAuthorAvatarURL(content)),
			"author_homepage_url": firstNonEmpty(sph.ShareURL, probe.SourceURL, input.URL),
		},
		Content: channelsContentWithSummary(content, contentdownload.ContentSummary{
			Platform:        PlatformID,
			Type:            contentdownload.ContentType(content),
			ID:              contentID,
			Title:           firstNonEmpty(sph.Description, contentdownload.ContentTitle(content)),
			Description:     sph.Description,
			Author:          firstNonEmpty(sph.AuthorNickname, contentdownload.ContentAuthor(content)),
			URL:             firstNonEmpty(sph.OriginVideoURL, cleanSphVideoURL(sph.VideoURL), contentdownload.ContentSummaryOf(content).URL),
			SourceURL:       firstNonEmpty(sph.ShareURL, probe.SourceURL, input.URL),
			AuthorNickname:  firstNonEmpty(sph.AuthorNickname, contentdownload.ContentAuthorNickname(content)),
			AuthorAvatarURL: firstNonEmpty(sph.AuthorAvatarURL, contentdownload.ContentAuthorAvatarURL(content)),
			CoverURL:        firstNonEmpty(sph.CoverURL, contentdownload.ContentCoverURL(content)),
		}),
	}
	plan, err := h.Plan(ctx, resolved)
	if err != nil {
		return nil, err
	}
	resolved.Pipeline = plan
	return resolved, nil
}

func channelsContentWithSummary(content any, summary contentdownload.ContentSummary) any {
	switch c := content.(type) {
	case *FeedURLContentEnvelope:
		next := *c
		next.Summary = summary
		return &next
	case *FeedContentEnvelope:
		next := *c
		next.Summary = summary
		return &next
	case *SphURLContentEnvelope:
		next := *c
		next.Summary = summary
		return &next
	case *SphContentEnvelope:
		next := *c
		next.Summary = summary
		return &next
	default:
		return contentdownload.NewContent(summary, contentdownload.ContentDataOf(content), contentdownload.ContentMetadataOf(content), contentdownload.ContentOutputOf(content))
	}
}

func (h *Handler) Plan(ctx context.Context, resolved *contentdownload.ResolvedRequest) (*contentdownload.PipelinePlan, error) {
	nodes := []contentdownload.PipelineNode{
		{ID: "download", Type: "download_asset", Stage: "download"},
	}
	if resolved != nil && resolved.Labels["key"] != "" && resolved.Labels["key"] != "0" && resolved.Suffix != ".jpg" && resolved.Suffix != ".zip" {
		nodes = append(nodes, contentdownload.PipelineNode{
			ID:        "decrypt",
			Type:      "wechat_channels_decrypt",
			Stage:     "post",
			DependsOn: []string{"download"},
			Args: map[string]any{
				"key":     resolved.Labels["key"],
				"bytes":   131072,
				"inplace": true,
			},
		})
	}
	if resolved != nil && resolved.Suffix == ".mp3" {
		dep := "download"
		if len(nodes) > 1 {
			dep = "decrypt"
		}
		nodes = append(nodes, contentdownload.PipelineNode{
			ID:        "transcode_mp3",
			Type:      "ffmpeg_extract_mp3",
			Stage:     "post",
			DependsOn: []string{dep},
			Args: map[string]any{
				"bitrate": "192k",
			},
		})
	}
	nodes = append(nodes, contentdownload.PipelineNode{ID: "persist", Type: "persist_artifacts", Stage: "persist"})
	return &contentdownload.PipelinePlan{Platform: PlatformID, Nodes: nodes}, nil
}

type FeedURLParts = channelspkg.FeedURLParts
type SphURLParts = channelspkg.SphURLParts

func ParseFeedURL(rawURL string) (*FeedURLParts, error) {
	parts, err := channelspkg.ParseFeedURL(rawURL)
	if errors.Is(err, channelspkg.ErrUnsupportedURL) {
		return nil, contentdownload.ErrUnsupportedURL
	}
	return parts, err
}

func ParseSphShareURL(rawURL string) (*SphURLParts, error) {
	parts, err := channelspkg.ParseSphShareURL(rawURL)
	if errors.Is(err, channelspkg.ErrUnsupportedURL) {
		return nil, contentdownload.ErrUnsupportedURL
	}
	return parts, err
}

func cleanSphVideoURL(videoURL string) string {
	u, err := url.Parse(videoURL)
	if err != nil {
		return ""
	}
	filekey := u.Query().Get("encfilekey")
	token := u.Query().Get("token")
	if filekey == "" || token == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host + u.Path + "?encfilekey=" + filekey + "&token=" + token
}

func channelsAuthorProfileURL(username string, fallback string) string {
	username = strings.TrimSpace(username)
	if username == "" {
		return strings.TrimSpace(fallback)
	}
	u := url.URL{
		Scheme: "https",
		Host:   "channels.weixin.qq.com",
		Path:   "/web/pages/profile",
	}
	q := u.Query()
	q.Set("username", username)
	u.RawQuery = q.Encode()
	return u.String()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
