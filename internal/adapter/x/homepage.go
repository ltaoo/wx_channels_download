package xadapter

import (
	"fmt"
	"net/url"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/cookies"
	x_scraper "wx_channel/pkg/scraper/x"
	"wx_channel/pkg/util"
)

var x_home_tabs = []adapter.HomeContentTab{
	{Scope: "posts", Name: "帖子", ContentTypes: []string{model.ContentTypePost}},
	{Scope: "replies", Name: "回复", ContentTypes: []string{model.ContentTypePost}},
	{Scope: "reposts", Name: "转帖", ContentTypes: []string{model.ContentTypePost}},
	{Scope: "media", Name: "媒体", ContentTypes: []string{model.ContentTypePost}},
}

func (h *handler) HomeContentTabs(_ *model.Account) []adapter.HomeContentTab {
	return append([]adapter.HomeContentTab(nil), x_home_tabs...)
}

func (h *handler) BuildHomeContents(account *model.Account, scope string) (*adapter.HomeContents, error) {
	username := x_account_username(account)
	if username == "" {
		return nil, fmt.Errorf("X 账号缺少用户名或主页 URL")
	}
	user_id := ""
	if account != nil {
		user_id = strings.TrimSpace(account.ExternalId)
	}
	if user_id == "" {
		return nil, fmt.Errorf("X 账号 external_id 不能为空")
	}
	scope = strings.TrimSpace(scope)
	client, err := x_scraper.NewClient(h.runtime_cookie_reader())
	if err != nil {
		return nil, err
	}
	defer client.Close()
	h.runtime_mu.RLock()
	client.SetPersistentCache(h.file_cache)
	h.runtime_mu.RUnlock()
	items, err := client.FetchTimelineContext(nil, user_id, username, scope)
	if err != nil {
		return nil, fmt.Errorf("获取 X 主页 %s 失败: %w", scope, err)
	}
	contents := make([]model.Content, 0, len(items))
	for _, item := range items {
		contents = append(contents, x_home_content(item))
	}
	return &adapter.HomeContents{Scope: scope, Contents: contents}, nil
}

func (h *handler) runtime_cookie_reader() *cookies.Reader {
	h.runtime_mu.RLock()
	defer h.runtime_mu.RUnlock()
	return h.cookie_reader
}

func x_home_content(item x_scraper.TimelineItem) model.Content {
	title := strings.TrimSpace(item.Text)
	if len([]rune(title)) > 160 {
		title = string([]rune(title)[:160])
	}
	source_url := "https://x.com/" + url.PathEscape(item.Username) + "/status/" + item.ExternalID
	var publish_time *int64
	if item.PublishTime > 0 {
		publish_time = &item.PublishTime
	}
	now := util.NowMillis()
	return model.Content{
		Id: PlatformID + ":" + item.ExternalID, PlatformId: PlatformID,
		Type: model.ContentTypePost, Subtype: model.ContentSubtypeMicroblog,
		ExternalId: item.ExternalID, Title: title, Description: item.Text,
		URL: source_url, SourceURL: source_url, CoverURL: item.CoverURL, PublishTime: publish_time,
		ViewCount: item.ViewCount, LikeCount: item.LikeCount,
		CommentCount: item.CommentCount, ShareCount: item.ShareCount,
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
}

func x_account_username(account *model.Account) string {
	if account == nil {
		return ""
	}
	if alias := strings.TrimPrefix(strings.TrimSpace(account.Alias), "@"); alias != "" {
		return alias
	}
	parsed_url, err := url.Parse(strings.TrimSpace(account.ProfileURL))
	if err == nil && (strings.EqualFold(parsed_url.Hostname(), "x.com") || strings.HasSuffix(strings.ToLower(parsed_url.Hostname()), ".x.com")) {
		parts := strings.Split(strings.Trim(parsed_url.Path, "/"), "/")
		if len(parts) > 0 {
			return strings.TrimPrefix(parts[0], "@")
		}
	}
	return ""
}
