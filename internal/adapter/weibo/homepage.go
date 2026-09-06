package weiboadapter

import (
	"fmt"
	"net/url"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/weibo"
	"wx_channel/pkg/util"
)

var weibo_home_tabs = []adapter.HomeContentTab{
	{Scope: "posts", Name: "微博", ContentTypes: []string{model.ContentTypePost, model.ContentTypeVideo, model.ContentTypeAlbum}},
	{Scope: "videos", Name: "视频", ContentTypes: []string{model.ContentTypeVideo}},
	{Scope: "photos", Name: "相册", ContentTypes: []string{model.ContentTypeAlbum}},
}

func (h *handler) HomeContentTabs(_ *model.Account) []adapter.HomeContentTab {
	return append([]adapter.HomeContentTab(nil), weibo_home_tabs...)
}

func (h *handler) BuildHomeContents(account *model.Account, scope string) (*adapter.HomeContents, error) {
	uid, err := weibo_account_uid(account)
	if err != nil {
		return nil, err
	}
	scope = strings.TrimSpace(scope)
	if scope != "posts" && scope != "videos" && scope != "photos" {
		return nil, fmt.Errorf("微博不支持主页 scope: %s", scope)
	}
	h.runtime_mu.RLock()
	client := weibo.NewClient(h.cookie_provider)
	client.SetPersistentCache(h.file_cache)
	h.runtime_mu.RUnlock()
	items, err := client.FetchHomeContext(nil, uid, scope)
	if err != nil {
		return nil, fmt.Errorf("获取微博主页%s失败: %w", scope, err)
	}
	contents := make([]model.Content, 0, len(items))
	for _, item := range items {
		contents = append(contents, weibo_home_content(item, uid))
	}
	return &adapter.HomeContents{Scope: scope, Contents: contents}, nil
}

func weibo_home_content(item weibo.HomeItem, uid string) model.Content {
	content_type := model.ContentTypePost
	if item.HasVideo {
		content_type = model.ContentTypeVideo
	} else if item.HasPhotos {
		content_type = model.ContentTypeAlbum
	}
	short_id := strings.TrimSpace(item.MblogID)
	if short_id == "" {
		short_id = item.ExternalID
	}
	source_url := "https://weibo.com/" + url.PathEscape(uid) + "/" + url.PathEscape(short_id)
	var publish_time *int64
	if item.PublishTime > 0 {
		publish_time = &item.PublishTime
	}
	now := util.NowMillis()
	return model.Content{
		Id: PlatformID + ":" + item.ExternalID, PlatformId: PlatformID, Type: content_type,
		ExternalId: item.ExternalID, ExternalId2: uid, Title: item.Text, Description: item.Text,
		URL: source_url, SourceURL: source_url, CoverURL: item.CoverURL, PublishTime: publish_time,
		LikeCount: item.LikeCount, CommentCount: item.CommentCount, ShareCount: item.RepostCount,
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
}

func weibo_account_uid(account *model.Account) (string, error) {
	if account == nil {
		return "", fmt.Errorf("微博账号不能为空")
	}
	if external_id := strings.TrimSpace(account.ExternalId); external_id != "" {
		return external_id, nil
	}
	parsed_url, err := url.Parse(strings.TrimSpace(account.ProfileURL))
	if err == nil {
		parts := strings.Split(strings.Trim(parsed_url.Path, "/"), "/")
		if len(parts) >= 2 && parts[0] == "u" {
			return parts[1], nil
		}
	}
	return "", fmt.Errorf("微博账号 external_id 不能为空")
}
