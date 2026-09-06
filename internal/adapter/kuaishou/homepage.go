package kuaishouadapter

import (
	"fmt"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/kuaishou"
	"wx_channel/pkg/util"
)

func (a *KuaishouAdapter) HomeContentTabs(_ *model.Account) []adapter.HomeContentTab {
	return []adapter.HomeContentTab{{Scope: "posts", Name: "作品", ContentTypes: []string{model.ContentTypeVideo, model.ContentTypeAlbum}}}
}

func (a *KuaishouAdapter) BuildHomeContents(account *model.Account, scope string) (*adapter.HomeContents, error) {
	if account == nil {
		return nil, fmt.Errorf("快手账号不能为空")
	}
	if strings.TrimSpace(scope) != "posts" {
		return nil, fmt.Errorf("快手不支持主页 scope: %s", scope)
	}
	user_id := strings.TrimSpace(account.ExternalId)
	if user_id == "" {
		return nil, fmt.Errorf("快手账号 external_id 不能为空")
	}
	client := kuaishou.NewClient()
	defer client.Close()
	client.SetCookie(a.kuaishou_cookie())
	a.runtime_mu.RLock()
	client.SetCookieProvider(a.cookie_reader)
	client.SetPersistentCache(a.file_cache)
	a.runtime_mu.RUnlock()
	feeds, err := client.FetchUserFeedContext(nil, user_id)
	if err != nil {
		return nil, fmt.Errorf("获取快手主页作品失败: %w", err)
	}
	return &adapter.HomeContents{Scope: scope, Contents: kuaishou_home_contents(feeds)}, nil
}

func kuaishou_home_contents(feeds []kuaishou.Feed) []model.Content {
	now := util.NowMillis()
	contents := make([]model.Content, 0, len(feeds))
	for _, feed := range feeds {
		photo_id := strings.TrimSpace(feed.Photo.ID)
		if photo_id == "" {
			continue
		}
		title := strings.TrimSpace(feed.Photo.Caption)
		if title == "" {
			title = photo_id
		}
		content_type := model.ContentTypeVideo
		if strings.Contains(strings.ToUpper(fmt.Sprint(feed.Type)), "IMAGE") {
			content_type = model.ContentTypeAlbum
		}
		var publish_time *int64
		if timestamp := feed.Photo.TimestampValue(); timestamp > 0 {
			if timestamp < 100000000000 {
				timestamp *= 1000
			}
			publish_time = &timestamp
		}
		source_url := "https://www.kuaishou.com/short-video/" + photo_id
		contents = append(contents, model.Content{
			Id: PlatformID + ":" + photo_id, PlatformId: PlatformID, Type: content_type,
			ExternalId: photo_id, ExternalId2: strings.TrimSpace(feed.Author.ID),
			Title: title, Description: title, URL: source_url, SourceURL: source_url,
			CoverURL: strings.TrimSpace(feed.Photo.CoverURL), PublishTime: publish_time,
			ViewCount: feed.Photo.ViewCountValue(), LikeCount: feed.Photo.LikeCountValue(),
			CommentCount: feed.Photo.CommentCountValue(),
			Timestamps:   model.Timestamps{CreatedAt: now, UpdatedAt: now},
		})
	}
	return contents
}
