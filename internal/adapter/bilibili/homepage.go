package bilibiliadapter

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/bilibili"
	"wx_channel/pkg/util"
)

var bilibili_home_tabs = []adapter.HomeContentTab{
	{Scope: "dynamic", Name: "动态", ContentTypes: []string{model.ContentTypePost, model.ContentTypeVideo, model.ContentTypeArticle}},
	{Scope: "video", Name: "投稿", ContentTypes: []string{model.ContentTypeVideo}},
	{Scope: "articles", Name: "专栏", ContentTypes: []string{model.ContentTypeArticle}},
	{Scope: "lists", Name: "合集和系列", ContentTypes: []string{model.ContentTypeCollection}},
	{Scope: "bangumi", Name: "追番追剧", ContentTypes: []string{model.ContentTypeCollection}},
}

func (h *handler) HomeContentTabs(_ *model.Account) []adapter.HomeContentTab {
	return append([]adapter.HomeContentTab(nil), bilibili_home_tabs...)
}

func (h *handler) BuildHomeContents(account *model.Account, scope string) (*adapter.HomeContents, error) {
	mid, err := bilibili_account_mid(account)
	if err != nil {
		return nil, err
	}
	scope = strings.TrimSpace(scope)
	if scope == "videos" {
		scope = "video"
	}
	if !bilibili_home_scope_supported(scope) {
		return nil, fmt.Errorf("B站不支持主页 scope: %s", scope)
	}
	client := bilibili.NewClientWithLoggerAndCookieProvider(
		h.config_string("bilibili.cookie"), h.get_cookie_provider(), h.get_logger(),
	)
	client.SetPersistentCache(h.get_file_cache())
	items, err := client.FetchHomeContext(nil, mid, scope)
	if err != nil {
		return nil, fmt.Errorf("获取B站主页%s失败: %w", scope, err)
	}
	contents := make([]model.Content, 0, len(items))
	for _, item := range items {
		contents = append(contents, bilibili_home_item_content(item))
	}
	return &adapter.HomeContents{Scope: scope, Contents: contents}, nil
}

type bilibili_user_video_reader interface {
	FetchVideoListOfUser(raw_url string, page int) (*bilibili.UserVideoList, error)
}

func (h *handler) FetchHomeDetails(account *model.Account, scope string, page_marker string) (*adapter.HomeDetails, error) {
	client := bilibili.NewClientWithLoggerAndCookieProvider(
		h.config_string("bilibili.cookie"), h.get_cookie_provider(), h.get_logger(),
	)
	client.SetPersistentCache(h.get_file_cache())
	return fetch_bilibili_home_details(client, account, scope, page_marker)
}

func fetch_bilibili_home_details(reader bilibili_user_video_reader, account *model.Account, scope string, page_marker string) (*adapter.HomeDetails, error) {
	mid, err := bilibili_account_mid(account)
	if err != nil {
		return nil, err
	}
	if reader == nil {
		return nil, fmt.Errorf("B站主页内容读取器不能为空")
	}
	scope = strings.TrimSpace(scope)
	if scope == "" || scope == "feed" || scope == "videos" {
		scope = "video"
	}
	if scope != "video" {
		return nil, fmt.Errorf("B站不支持主页详情 scope: %s", scope)
	}
	page := 1
	if page_marker = strings.TrimSpace(page_marker); page_marker != "" {
		parsed_page, parse_err := strconv.Atoi(page_marker)
		if parse_err != nil || parsed_page < 1 {
			return nil, fmt.Errorf("B站主页 page 必须是正整数: %s", page_marker)
		}
		page = parsed_page
	}
	page_url := "https://space.bilibili.com/" + url.PathEscape(mid) + "/upload/video"
	video_list, err := reader.FetchVideoListOfUser(page_url, page)
	if err != nil {
		return nil, fmt.Errorf("获取B站账号 %s 的投稿失败: %w", mid, err)
	}
	if video_list == nil {
		return nil, fmt.Errorf("B站账号 %s 的投稿列表为空", mid)
	}
	contents := make([]model.Content, 0, len(video_list.Items))
	for _, item := range video_list.Items {
		if strings.TrimSpace(item.BVID) == "" {
			continue
		}
		contents = append(contents, bilibili_home_content(
			item.BVID, item.Title, item.Description, item.CoverURL,
			model.ContentTypeVideo, "https://www.bilibili.com/video/"+item.BVID,
			item.CreatedAt, item.PlayCount, item.CommentCount,
		))
	}
	next_marker := ""
	if video_list.HasNext && video_list.NextPage > 0 {
		next_marker = strconv.Itoa(video_list.NextPage)
	}
	return &adapter.HomeDetails{
		Scopes: []adapter.HomeDetailsScope{{Label: "投稿", Value: "video"}},
		Scope:  "video", Contents: contents, NextMarker: next_marker,
	}, nil
}

func bilibili_home_scope_supported(scope string) bool {
	for _, tab := range bilibili_home_tabs {
		if tab.Scope == scope {
			return true
		}
	}
	return false
}

func bilibili_home_item_content(item bilibili.HomeItem) model.Content {
	content_type := model.ContentTypePost
	switch item.Kind {
	case bilibili.HomeKindVideo:
		content_type = model.ContentTypeVideo
	case bilibili.HomeKindArticle:
		content_type = model.ContentTypeArticle
	case bilibili.HomeKindCollection:
		content_type = model.ContentTypeCollection
	}
	content := bilibili_home_content(
		item.ID, item.Title, item.Description, item.CoverURL, content_type,
		item.SourceURL, item.PublishTime, item.ViewCount, item.CommentCount,
	)
	content.LikeCount = item.LikeCount
	content.CollectCount = item.CollectCount
	return content
}

func bilibili_home_content(id string, title string, description string, cover_url string, content_type string, source_url string, publish_seconds int64, view_count int64, comment_count int64) model.Content {
	now := util.NowMillis()
	title = strings.TrimSpace(title)
	if title == "" {
		title = id
	}
	var publish_time *int64
	if publish_seconds > 0 {
		value := publish_seconds * 1000
		publish_time = &value
	}
	return model.Content{
		Id: PlatformID + ":" + id, PlatformId: PlatformID, Type: content_type,
		ExternalId: id, Title: title, Description: strings.TrimSpace(description),
		URL: source_url, SourceURL: source_url, CoverURL: cover_url, PublishTime: publish_time,
		ViewCount: view_count, CommentCount: comment_count,
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
}

func bilibili_account_mid(account *model.Account) (string, error) {
	if account == nil {
		return "", fmt.Errorf("B站账号不能为空")
	}
	if external_id := strings.TrimSpace(account.ExternalId); external_id != "" {
		return external_id, nil
	}
	parsed_url, err := url.Parse(strings.TrimSpace(account.ProfileURL))
	if err == nil && strings.EqualFold(parsed_url.Hostname(), "space.bilibili.com") {
		parts := strings.Split(strings.Trim(parsed_url.Path, "/"), "/")
		if len(parts) > 0 && parts[0] != "" {
			return parts[0], nil
		}
	}
	return "", fmt.Errorf("B站账号 external_id 不能为空")
}
