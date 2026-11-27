package instagramadapter

import (
	"fmt"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/instagram"
	"wx_channel/pkg/util"
)

func (h *handler) HomeContentTabs(_ *model.Account) []adapter.HomeContentTab {
	return []adapter.HomeContentTab{{Scope: "posts", Name: "帖子", ContentTypes: []string{model.ContentTypeAlbum, model.ContentTypeVideo}}}
}

func (h *handler) BuildHomeContents(account *model.Account, scope string) (*adapter.HomeContents, error) {
	return h.fetch_home_contents(account, scope, "")
}

func (h *handler) FetchHomeDetails(account *model.Account, scope string, page_marker string) (*adapter.HomeDetails, error) {
	result, err := h.fetch_home_contents(account, scope, page_marker)
	if err != nil {
		return nil, err
	}
	return &adapter.HomeDetails{Scopes: []adapter.HomeDetailsScope{{Label: "帖子", Value: "posts"}},
		Scope: result.Scope, Contents: result.Contents, NextMarker: result.NextMarker}, nil
}

func (h *handler) fetch_home_contents(account *model.Account, scope string, page_marker string) (*adapter.HomeContents, error) {
	if account == nil || account.PlatformId != PlatformID {
		return nil, fmt.Errorf("instagram: invalid homepage account")
	}
	scope = strings.TrimSpace(scope)
	if scope == "" || scope == "feed" {
		scope = "posts"
	}
	if scope != "posts" {
		return nil, fmt.Errorf("instagram: unsupported homepage scope %q", scope)
	}
	username, err := instagram.ExtractUsername(account.Alias)
	if err != nil {
		username, err = instagram.ExtractUsername(account.ProfileURL)
	}
	if err != nil {
		return nil, fmt.Errorf("instagram: homepage requires an account username or profile URL: %w", err)
	}
	client, err := h.new_scraper_client()
	if err != nil {
		return nil, err
	}
	defer client.Close()
	result, err := client.FetchHomeContentsPage(username, page_marker)
	if err != nil {
		return nil, err
	}
	return &adapter.HomeContents{Scope: scope, Contents: home_contents(result), NextMarker: result.NextMarker}, nil
}

func home_contents(result *instagram.HomeResult) []model.Content {
	contents := make([]model.Content, 0, len(result.Items))
	now := util.NowMillis()
	for _, item := range result.Items {
		content_type, subtype := model.ContentTypeAlbum, model.ContentSubtypePhotoAlbum
		if item.MediaType == 2 {
			content_type, subtype = model.ContentTypeVideo, model.ContentSubtypeShortVideo
		}
		contents = append(contents, model.Content{Id: PlatformID + ":" + item.ExternalID, PlatformId: PlatformID,
			Type: content_type, Subtype: subtype, ExternalId: item.ExternalID, ExternalId2: item.Shortcode,
			Title: content_title(item.BodyText, result.Account.Username, item.ExternalID), Description: item.BodyText,
			URL: item.SourceURL, SourceURL: item.SourceURL, CoverURL: item.CoverURL,
			Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now}})
	}
	return contents
}
