package douyinadapter

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/douyin"
	"wx_channel/pkg/util"
)

func (h *handler) HomeContentTabs(_ *model.Account) []adapter.HomeContentTab {
	return []adapter.HomeContentTab{{
		Scope: "posts", Name: "作品", ContentTypes: []string{model.ContentTypeVideo, model.ContentTypeAlbum},
	}}
}

func (h *handler) BuildHomeContents(account *model.Account, scope string) (*adapter.HomeContents, error) {
	if account == nil {
		return nil, fmt.Errorf("抖音账号不能为空")
	}
	if strings.TrimSpace(scope) != "posts" {
		return nil, fmt.Errorf("抖音不支持主页 scope: %s", scope)
	}
	page_url := "https://www.douyin.com/user/" + url.PathEscape(strings.TrimSpace(account.ExternalId))
	home, err := h.douyin_home_client().FetchHomeContents(page_url)
	if err != nil {
		return nil, err
	}
	contents := douyin_home_contents_from_items(home.Items, account)
	return &adapter.HomeContents{Scope: "posts", Contents: contents}, nil
}

func (h *handler) douyin_home_client() *douyin.Client {
	client := douyin.NewClientWithLoggerAndCookieReader(
		h.config_string("douyin.cookie"), h.cookie_reader(), h.get_logger(),
	)
	client.SetPersistentCache(h.runtime_file_cache())
	return client
}

type douyin_home_content_reader interface {
	FetchHomeContentsPage(raw_url string, page_marker string) (*douyin.HomeContentList, error)
}

func (h *handler) FetchHomeDetails(account *model.Account, scope string, page_marker string) (*adapter.HomeDetails, error) {
	return fetch_douyin_home_details(h.douyin_home_client(), account, scope, page_marker)
}

func fetch_douyin_home_details(reader douyin_home_content_reader, account *model.Account, scope string, page_marker string) (*adapter.HomeDetails, error) {
	if account == nil {
		return nil, fmt.Errorf("抖音账号不能为空")
	}
	if reader == nil {
		return nil, fmt.Errorf("抖音主页内容读取器不能为空")
	}
	scope = strings.TrimSpace(scope)
	if scope == "" || scope == "feed" {
		scope = "posts"
	}
	if scope != "posts" {
		return nil, fmt.Errorf("抖音不支持主页详情 scope: %s", scope)
	}
	account_id := strings.TrimSpace(account.ExternalId)
	if account_id == "" {
		return nil, fmt.Errorf("抖音账号 external_id 不能为空")
	}
	page_url := "https://www.douyin.com/user/" + url.PathEscape(account_id)
	home, err := reader.FetchHomeContentsPage(page_url, page_marker)
	if err != nil {
		return nil, fmt.Errorf("获取抖音账号 %s 的作品失败: %w", account_id, err)
	}
	if home == nil {
		return nil, fmt.Errorf("抖音账号 %s 的作品列表为空", account_id)
	}
	return &adapter.HomeDetails{
		Scopes:     []adapter.HomeDetailsScope{{Label: "作品", Value: "posts"}},
		Scope:      "posts",
		Contents:   douyin_home_contents_from_items(home.Items, account),
		NextMarker: home.NextMarker,
	}, nil
}

func douyin_home_contents_from_items(items []douyin.HomeContent, account *model.Account) []model.Content {
	now := util.NowMillis()
	contents := make([]model.Content, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		external_id := strings.TrimSpace(item.ID)
		if external_id == "" {
			continue
		}
		content_type := douyin_content_type_video
		if item.Type == "note" {
			content_type = douyin_content_type_album
		} else if item.Type != "video" {
			continue
		}
		content_id := BuildContentID(external_id)
		if _, exists := seen[content_id]; exists {
			continue
		}
		seen[content_id] = struct{}{}
		title := douyin_home_title(item.Title, account)
		if title == "" {
			title = external_id
		}
		source_url := strings.TrimSpace(item.URL)
		if source_url == "" {
			source_url = douyin_content_source_url(content_type, external_id)
		}
		contents = append(contents, model.Content{
			Id: content_id, PlatformId: PlatformID, Type: content_type,
			ExternalId: external_id, Title: title, Description: title,
			URL: source_url, SourceURL: source_url, CoverURL: strings.TrimSpace(item.CoverURL),
			Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
		})
	}
	return contents
}

func douyin_home_contents_from_html(document_html string, account *model.Account) ([]model.Content, error) {
	document, err := html.Parse(strings.NewReader(document_html))
	if err != nil {
		return nil, fmt.Errorf("解析抖音主页失败: %w", err)
	}
	post_list := douyin_home_find_node(document, "data-e2e", "user-post-list")
	if post_list == nil {
		return nil, fmt.Errorf("抖音主页缺少作品列表")
	}

	now := util.NowMillis()
	seen := make(map[string]struct{})
	contents := make([]model.Content, 0)
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, "a") {
			content, ok := douyin_home_content_from_anchor(node, account, now)
			if ok {
				if _, exists := seen[content.Id]; !exists {
					seen[content.Id] = struct{}{}
					contents = append(contents, content)
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(post_list)
	return contents, nil
}

func douyin_home_content_from_anchor(node *html.Node, account *model.Account, now int64) (model.Content, bool) {
	raw_url := strings.TrimSpace(douyin_home_attribute(node, "href"))
	parsed_url, err := url.Parse(raw_url)
	if err != nil {
		return model.Content{}, false
	}
	parts := strings.Split(strings.Trim(parsed_url.Path, "/"), "/")
	if len(parts) != 2 || (parts[0] != "video" && parts[0] != "note") || parts[1] == "" {
		return model.Content{}, false
	}

	external_id := parts[1]
	content_type := douyin_content_type_video
	if parts[0] == "note" {
		content_type = douyin_content_type_album
	}
	image := douyin_home_find_element(node, "img")
	title := external_id
	cover_url := ""
	if image != nil {
		title = douyin_home_title(douyin_home_attribute(image, "alt"), account)
		cover_url = strings.TrimSpace(douyin_home_attribute(image, "src"))
	}
	if title == "" {
		title = external_id
	}
	source_url := douyin_content_source_url(content_type, external_id)
	return model.Content{
		Id:          BuildContentID(external_id),
		PlatformId:  PlatformID,
		Type:        content_type,
		ExternalId:  external_id,
		Title:       title,
		Description: title,
		SourceURL:   source_url,
		CoverURL:    cover_url,
		Timestamps:  model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}, true
}

func douyin_home_title(value string, account *model.Account) string {
	title := strings.TrimSpace(value)
	if account != nil {
		for _, separator := range []string{"：", ":"} {
			title = strings.TrimPrefix(title, strings.TrimSpace(account.Nickname)+separator)
		}
	}
	return strings.TrimSpace(title)
}

func douyin_home_find_node(node *html.Node, attribute_name string, attribute_value string) *html.Node {
	if node.Type == html.ElementNode && douyin_home_attribute(node, attribute_name) == attribute_value {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := douyin_home_find_node(child, attribute_name, attribute_value); found != nil {
			return found
		}
	}
	return nil
}

func douyin_home_find_element(node *html.Node, name string) *html.Node {
	if node.Type == html.ElementNode && strings.EqualFold(node.Data, name) {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := douyin_home_find_element(child, name); found != nil {
			return found
		}
	}
	return nil
}

func douyin_home_attribute(node *html.Node, name string) string {
	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, name) {
			return attribute.Val
		}
	}
	return ""
}
