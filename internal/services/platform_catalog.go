package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
)

type ScraperPlatformInfo struct {
	ID     string                   `json:"id"`
	Name   string                   `json:"name"`
	Source string                   `json:"source"`
	Detail bool                     `json:"detail"`
	List   bool                     `json:"list"`
	Tabs   []adapter.HomeContentTab `json:"tabs"`
}

func ScraperPlatforms() []ScraperPlatformInfo {
	items := make([]ScraperPlatformInfo, 0)
	for _, platform_id := range adapter.IDs() {
		item := ScraperPlatformInfo{ID: platform_id, Name: platform_display_name(platform_id), Source: platform_id, Detail: true, Tabs: []adapter.HomeContentTab{}}
		if platform_id == "tencent" {
			item.Source = "vqq"
		}
		if builder, ok := adapter.Get(platform_id).(adapter.HomeContentsBuilder); ok {
			item.List = true
			item.Tabs = builder.HomeContentTabs(&model.Account{PlatformId: platform_id})
		}
		items = append(items, item)
	}
	return items
}

type ScraperPlatformFetchRequest struct {
	Platform  string `json:"platform"`
	Operation string `json:"operation"`
	URL       string `json:"url"`
	Profile   string `json:"profile"`
	UserID    string `json:"user_id"`
	Scope     string `json:"scope"`
	Page      string `json:"page"`
}

type ScraperPlatformListResult struct {
	Platform   string                   `json:"platform"`
	Scope      string                   `json:"scope"`
	List       []model.Content          `json:"list"`
	Total      int                      `json:"total"`
	NextMarker string                   `json:"next_marker"`
	Tabs       []adapter.HomeContentTab `json:"tabs"`
}

// FetchScraperPlatform calls the selected implementation without creating a
// download task or requiring a saved account.
func FetchScraperPlatform(fetch_context context.Context, request ScraperPlatformFetchRequest) (any, error) {
	platform_id := strings.TrimSpace(request.Platform)
	handler := adapter.Get(platform_id)
	if handler == nil {
		return nil, fmt.Errorf("未注册的平台: %s", platform_id)
	}
	fetch_context, cancel := context.WithTimeout(fetch_context, time.Duration(default_platform_fetch_timeout_seconds)*time.Second)
	defer cancel()
	switch request.Operation {
	case "detail":
		raw_url := strings.TrimSpace(request.URL)
		if raw_url == "" {
			return nil, fmt.Errorf("请填写内容链接")
		}
		if platform_id != "singlefile" && platform_id != "webpage" {
			resolution, err := ResolveScraperPlatform(raw_url)
			if err != nil {
				return nil, err
			}
			if resolution.Platform != platform_id {
				return nil, fmt.Errorf("链接不属于所选平台 %s", platform_display_name(platform_id))
			}
		}
		if contextual, ok := handler.(adapter.ContextProgressFetchAdapter); ok {
			return contextual.FetchWithProgressContext(fetch_context, raw_url, adapter.FetchOptions{})
		}
		return handler.Fetch(raw_url)
	case "list":
		builder, ok := handler.(adapter.HomeContentsBuilder)
		if !ok {
			return nil, fmt.Errorf("平台 %s 尚未实现获取列表", platform_display_name(platform_id))
		}
		account, err := build_platform_home_account(platform_id, request.Profile, request.UserID)
		if err != nil {
			return nil, err
		}
		scope := strings.TrimSpace(request.Scope)
		if scope == "" {
			scope = default_platform_home_scope(builder, account)
		}
		items, next_marker, used_scope, err := fetch_platform_home_list(builder, handler, account, scope, strings.TrimSpace(request.Page), true)
		if err != nil {
			return nil, err
		}
		return &ScraperPlatformListResult{Platform: platform_id, Scope: used_scope, List: items, Total: len(items), NextMarker: next_marker, Tabs: builder.HomeContentTabs(account)}, nil
	default:
		return nil, fmt.Errorf("operation 必须为 detail 或 list")
	}
}
