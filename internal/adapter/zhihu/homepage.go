package zhihuadapter

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/zhihu"
	"wx_channel/pkg/util"
)

type zhihu_home_content_reader interface {
	FetchAnswerListOfUser(raw_url string, page int) (*zhihu.UserContentList, error)
	FetchPostListOfUser(raw_url string, page int) (*zhihu.UserContentList, error)
	FetchZvideoListOfUser(raw_url string, page int) (*zhihu.UserContentList, error)
	FetchColumnListOfUser(raw_url string, page int) (*zhihu.UserContentList, error)
}

// HomeContentTabs returns the independently fetchable Zhihu profile sections.
func (h *handler) HomeContentTabs(_ *model.Account) []adapter.HomeContentTab {
	return zhihu_home_content_tabs()
}

func zhihu_home_content_tabs() []adapter.HomeContentTab {
	return []adapter.HomeContentTab{
		{Scope: zhihu.UserContentKindAnswers, Name: "回答", ContentTypes: []string{"answer"}},
		{Scope: zhihu.UserContentKindPosts, Name: "文章", ContentTypes: []string{model.ContentTypeArticle}},
		{Scope: zhihu.UserContentKindZvideos, Name: "视频", ContentTypes: []string{"zvideo"}},
		{Scope: zhihu.UserContentKindColumns, Name: "专栏", ContentTypes: []string{model.ContentTypeCollection}},
	}
}

// BuildHomeContents fetches only the selected Zhihu profile section.
func (h *handler) BuildHomeContents(account *model.Account, scope string) (*adapter.HomeContents, error) {
	return build_zhihu_home_contents(h.scraper_client(), account, scope)
}

// FetchHomeDetails fetches a page of Zhihu profile content without persisting
// it. Answers are the default because Zhihu has no generic profile feed.
func (h *handler) FetchHomeDetails(account *model.Account, scope string, page string) (*adapter.HomeDetails, error) {
	return fetch_zhihu_home_details(h.scraper_client(), account, scope, page)
}

func fetch_zhihu_home_details(reader zhihu_home_content_reader, account *model.Account, scope string, page_marker string) (*adapter.HomeDetails, error) {
	scope = strings.TrimSpace(scope)
	if scope == "" || scope == "feed" {
		scope = zhihu.UserContentKindAnswers
	}
	page := 1
	if page_marker = strings.TrimSpace(page_marker); page_marker != "" {
		parsed_page, err := strconv.Atoi(page_marker)
		if err != nil || parsed_page < 1 {
			return nil, fmt.Errorf("知乎主页 page 必须是正整数: %s", page_marker)
		}
		page = parsed_page
	}

	content_list, name, err := fetch_zhihu_home_content_list(reader, account, scope, page)
	if err != nil {
		return nil, err
	}
	contents, err := zhihu_home_content_scope(content_list, scope, name)
	if err != nil {
		return nil, err
	}
	scopes := make([]adapter.HomeDetailsScope, 0, len(zhihu_home_content_tabs()))
	for _, tab := range zhihu_home_content_tabs() {
		scopes = append(scopes, adapter.HomeDetailsScope{Label: tab.Name, Value: tab.Scope})
	}
	next_marker := ""
	if content_list.HasNext && content_list.NextPage > 0 {
		next_marker = strconv.Itoa(content_list.NextPage)
	}
	return &adapter.HomeDetails{
		Scopes: scopes, Scope: scope, Contents: contents, NextMarker: next_marker,
	}, nil
}

func build_zhihu_home_contents(reader zhihu_home_content_reader, account *model.Account, scope string) (*adapter.HomeContents, error) {
	content_list, name, err := fetch_zhihu_home_content_list(reader, account, scope, 1)
	if err != nil {
		return nil, err
	}
	contents, err := zhihu_home_content_scope(content_list, strings.TrimSpace(scope), name)
	if err != nil {
		return nil, err
	}
	return &adapter.HomeContents{Scope: strings.TrimSpace(scope), Contents: contents}, nil
}

func fetch_zhihu_home_content_list(reader zhihu_home_content_reader, account *model.Account, scope string, page int) (*zhihu.UserContentList, string, error) {
	if account == nil {
		return nil, "", fmt.Errorf("知乎账号不能为空")
	}
	if reader == nil {
		return nil, "", fmt.Errorf("知乎主页内容读取器不能为空")
	}
	url_token := zhihu_account_url_token(account)
	if url_token == "" {
		return nil, "", fmt.Errorf("知乎账号 external_id 不能为空")
	}

	scope = strings.TrimSpace(scope)
	var name string
	var fetch func(raw_url string, page int) (*zhihu.UserContentList, error)
	switch scope {
	case zhihu.UserContentKindAnswers:
		name, fetch = "回答", reader.FetchAnswerListOfUser
	case zhihu.UserContentKindPosts:
		name, fetch = "文章", reader.FetchPostListOfUser
	case zhihu.UserContentKindZvideos:
		name, fetch = "视频", reader.FetchZvideoListOfUser
	case zhihu.UserContentKindColumns:
		name, fetch = "专栏", reader.FetchColumnListOfUser
	default:
		return nil, "", fmt.Errorf("知乎不支持主页 scope: %s", scope)
	}
	profile_url := zhihu.SourceURL + "people/" + url.PathEscape(url_token) + "/" + scope
	content_list, err := fetch(profile_url, page)
	if err != nil {
		return nil, "", fmt.Errorf("获取知乎账号 %s 的%s失败: %w", url_token, name, err)
	}
	return content_list, name, nil
}

func zhihu_home_content_scope(content_list *zhihu.UserContentList, expected_scope string, name string) ([]model.Content, error) {
	if content_list == nil {
		return nil, fmt.Errorf("知乎%s列表为空", name)
	}
	if content_list.Kind != "" && content_list.Kind != expected_scope {
		return nil, fmt.Errorf(
			"知乎%s列表类型不匹配: want %s, got %s",
			name,
			expected_scope,
			content_list.Kind,
		)
	}

	now := util.NowMillis()
	contents := make([]model.Content, 0, len(content_list.Items))
	for _, item := range content_list.Items {
		content, ok := zhihu_user_content_item_to_content(item, expected_scope, now)
		if ok {
			contents = append(contents, content)
		}
	}
	return contents, nil
}

func zhihu_user_content_item_to_content(item zhihu.UserContentItem, scope string, now int64) (model.Content, bool) {
	external_id := strings.TrimSpace(item.ID)
	if external_id == "" {
		return model.Content{}, false
	}
	content_type := strings.ToLower(strings.TrimSpace(item.Type))
	if content_type == "" {
		content_type = zhihu_home_scope_content_type(scope)
	}
	if content_type == "" {
		return model.Content{}, false
	}
	metadata, _ := json.Marshal(item)
	content_url := strings.TrimSpace(item.URL)
	return model.Content{
		Id:           BuildTypedContentID(content_type, external_id),
		PlatformId:   PlatformID,
		Type:         content_type,
		ExternalId:   external_id,
		ExternalId2:  strings.TrimSpace(item.QuestionID),
		Title:        first_non_empty_str(item.Title, item.Excerpt, external_id),
		Description:  strings.TrimSpace(item.Excerpt),
		URL:          content_url,
		SourceURL:    content_url,
		CoverURL:     strings.TrimSpace(item.ImageURL),
		PublishTime:  int64_ptr(item.CreatedAt * 1000),
		UpdateTime:   int64_ptr(item.UpdatedAt * 1000),
		ViewCount:    int64(item.PlayCount),
		LikeCount:    int64(item.VoteupCount),
		CommentCount: int64(item.CommentCount),
		CollectCount: int64(item.FavoriteCount),
		Metadata:     string(metadata),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, true
}

func zhihu_home_scope_content_type(scope string) string {
	switch scope {
	case zhihu.UserContentKindAnswers:
		return "answer"
	case zhihu.UserContentKindPosts:
		return "article"
	case zhihu.UserContentKindZvideos:
		return "zvideo"
	case zhihu.UserContentKindColumns:
		return "column"
	default:
		return ""
	}
}

func zhihu_account_url_token(account *model.Account) string {
	if account == nil {
		return ""
	}
	if external_id := strings.TrimSpace(account.ExternalId); external_id != "" {
		return external_id
	}
	profile_url, err := url.Parse(strings.TrimSpace(account.ProfileURL))
	if err != nil || !strings.EqualFold(profile_url.Hostname(), "www.zhihu.com") {
		return ""
	}
	path_parts := strings.Split(strings.Trim(profile_url.EscapedPath(), "/"), "/")
	if len(path_parts) < 2 || path_parts[0] != "people" {
		return ""
	}
	url_token, err := url.PathUnescape(path_parts[1])
	if err != nil {
		return ""
	}
	return strings.TrimSpace(url_token)
}
