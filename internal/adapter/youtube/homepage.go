package youtubeadapter

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/youtube"
	"wx_channel/pkg/util"
)

var youtube_home_tabs = []adapter.HomeContentTab{
	{Scope: "videos", Name: "视频", ContentTypes: []string{model.ContentTypeVideo}},
	{Scope: "shorts", Name: "Shorts", ContentTypes: []string{model.ContentTypeVideo}},
	{Scope: "streams", Name: "直播", ContentTypes: []string{model.ContentTypeLive, model.ContentTypeVideo}},
	{Scope: "podcasts", Name: "播客", ContentTypes: []string{model.ContentTypePodcast, model.ContentTypeCollection}},
	{Scope: "playlists", Name: "播放列表", ContentTypes: []string{model.ContentTypeCollection}},
	{Scope: "community", Name: "帖子", ContentTypes: []string{model.ContentTypePost}},
}

func (h *handler) HomeContentTabs(_ *model.Account) []adapter.HomeContentTab {
	return append([]adapter.HomeContentTab(nil), youtube_home_tabs...)
}

func (h *handler) BuildHomeContents(account *model.Account, scope string) (*adapter.HomeContents, error) {
	scope = youtube_home_scope(scope)
	if !youtube_home_scope_supported(scope) {
		return nil, fmt.Errorf("YouTube 不支持主页 scope: %s", scope)
	}
	page, err := h.fetch_youtube_home_contents_page(account, scope, "")
	if err != nil {
		return nil, err
	}
	if page == nil {
		return nil, fmt.Errorf("YouTube 主页%s列表为空", youtube_home_scope_name(scope))
	}
	return &adapter.HomeContents{Scope: scope, Contents: page.contents}, nil
}

type youtube_home_content_page struct {
	contents    []model.Content
	next_marker string
}

type youtube_home_content_page_reader interface {
	fetch_youtube_home_contents_page(account *model.Account, scope string, page_marker string) (*youtube_home_content_page, error)
}

// FetchHomeDetails fetches one YouTube channel tab. Videos are the default
// channel content scope exposed by the generic account details endpoint.
func (h *handler) FetchHomeDetails(account *model.Account, scope string, page_marker string) (*adapter.HomeDetails, error) {
	return fetch_youtube_home_details(h, account, scope, page_marker)
}

func fetch_youtube_home_details(reader youtube_home_content_page_reader, account *model.Account, scope string, page_marker string) (*adapter.HomeDetails, error) {
	if reader == nil {
		return nil, fmt.Errorf("YouTube 主页内容读取器不能为空")
	}
	scope = youtube_home_scope(scope)
	if !youtube_home_scope_supported(scope) {
		return nil, fmt.Errorf("YouTube 不支持主页详情 scope: %s", scope)
	}
	page, err := reader.fetch_youtube_home_contents_page(account, scope, strings.TrimSpace(page_marker))
	if err != nil {
		return nil, err
	}
	if page == nil {
		return nil, fmt.Errorf("YouTube 主页%s列表为空", youtube_home_scope_name(scope))
	}
	contents := page.contents
	if contents == nil {
		contents = make([]model.Content, 0)
	}
	scopes := make([]adapter.HomeDetailsScope, 0, len(youtube_home_tabs))
	for _, tab := range youtube_home_tabs {
		scopes = append(scopes, adapter.HomeDetailsScope{Label: tab.Name, Value: tab.Scope})
	}
	return &adapter.HomeDetails{Scopes: scopes, Scope: scope, Contents: contents, NextMarker: page.next_marker}, nil
}

func (h *handler) fetch_youtube_home_contents_page(account *model.Account, scope string, page_marker string) (*youtube_home_content_page, error) {
	base_url, err := youtube_channel_url(account)
	if err != nil {
		return nil, err
	}
	parsed_page, err := h.scraper_client(false).FetchChannelContentsPage(context.Background(), base_url, scope, page_marker)
	if err != nil {
		return nil, fmt.Errorf("获取 YouTube 频道 %s 失败: %w", scope, err)
	}
	return &youtube_home_content_page{
		contents: youtube_channel_contents(parsed_page.Items), next_marker: parsed_page.NextMarker,
	}, nil
}

func youtube_home_scope(scope string) string {
	scope = strings.ToLower(strings.TrimSpace(scope))
	switch scope {
	case "", "feed", "video":
		return "videos"
	default:
		return scope
	}
}

func youtube_home_scope_supported(scope string) bool {
	for _, tab := range youtube_home_tabs {
		if tab.Scope == scope {
			return true
		}
	}
	return false
}

func youtube_home_scope_name(scope string) string {
	for _, tab := range youtube_home_tabs {
		if tab.Scope == scope {
			return tab.Name
		}
	}
	return scope
}

func youtube_channel_contents(items []youtube.ChannelContent) []model.Content {
	now := util.NowMillis()
	contents := make([]model.Content, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		title := strings.TrimSpace(item.Title)
		if title == "" {
			title = item.ID
		}
		contents = append(contents, model.Content{
			Id: PlatformID + ":" + item.ID, PlatformId: PlatformID,
			Type: item.Type, ExternalId: item.ID, Title: title, Description: title,
			URL: item.URL, SourceURL: item.URL, CoverURL: item.CoverURL,
			Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
		})
	}
	return contents
}

func youtube_channel_url(account *model.Account) (string, error) {
	if account == nil {
		return "", fmt.Errorf("YouTube 账号不能为空")
	}
	if raw_url := strings.TrimSpace(account.ProfileURL); raw_url != "" {
		parsed_url, err := url.Parse(raw_url)
		if err == nil && youtube_home_hostname(parsed_url.Hostname()) {
			parts := strings.Split(strings.Trim(parsed_url.Path, "/"), "/")
			if len(parts) >= 2 && parts[0] == "channel" {
				return "https://www.youtube.com/channel/" + parts[1], nil
			}
			if len(parts) >= 1 && strings.HasPrefix(parts[0], "@") {
				return "https://www.youtube.com/" + parts[0], nil
			}
		}
	}
	external_id := strings.TrimSpace(account.ExternalId)
	if external_id == "" {
		return "", fmt.Errorf("YouTube 账号 external_id 不能为空")
	}
	return "https://www.youtube.com/channel/" + url.PathEscape(external_id), nil
}

func youtube_home_hostname(hostname string) bool {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	return hostname == "youtube.com" || strings.HasSuffix(hostname, ".youtube.com")
}
