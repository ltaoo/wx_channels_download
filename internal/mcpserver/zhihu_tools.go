package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"wx_channel/pkg/cookies"
	"wx_channel/pkg/scraper/zhihu"
)

const (
	get_zhihu_credential_status_tool_name   = "get_zhihu_credential_status"
	get_my_zhihu_collections_tool_name      = "get_my_zhihu_collections"
	get_zhihu_collection_contents_tool_name = "get_zhihu_collection_contents"
	get_my_zhihu_answers_tool_name          = "get_my_zhihu_answers"
	get_my_zhihu_posts_tool_name            = "get_my_zhihu_posts"
	get_my_zhihu_zvideos_tool_name          = "get_my_zhihu_zvideos"
	get_my_zhihu_columns_tool_name          = "get_my_zhihu_columns"
	zhihu_current_user_url                  = "https://www.zhihu.com/api/v4/me"
	zhihu_login_cookie_name                 = "z_c0"
)

// ZhihuCollectionReader supplies authenticated Zhihu collection and profile
// data to MCP tools.
type ZhihuCollectionReader interface {
	FetchCurrentUser() (*zhihu.User, error)
	FetchCollectionList(raw_url string) (*zhihu.CollectionList, error)
	FetchContentListOfCollection(collection_id string, page int) (*zhihu.CollectionContentList, error)
	FetchAnswerListOfUser(raw_url string, page int) (*zhihu.UserContentList, error)
	FetchPostListOfUser(raw_url string, page int) (*zhihu.UserContentList, error)
	FetchZvideoListOfUser(raw_url string, page int) (*zhihu.UserContentList, error)
	FetchColumnListOfUser(raw_url string, page int) (*zhihu.UserContentList, error)
}

// ZhihuCredentialReader reads URL-matched credentials from cookies.json.
type ZhihuCredentialReader interface {
	HeaderForURL(raw_url string) (string, error)
}

type zhihu_collection_contents_arguments struct {
	CollectionID string `json:"collection_id"`
	Page         int    `json:"page"`
}

type zhihu_page_arguments struct {
	Page int `json:"page"`
}

func zhihu_tool_definitions() []any {
	return []any{
		zhihu_tool_definition(
			get_zhihu_credential_status_tool_name,
			"检查知乎凭证",
			"检查 cookies.json 是否包含未过期且适用于 zhihu.com 的 z_c0 登录 Cookie，并通过当前用户接口验证登录态。不会返回 Cookie 明文。",
			map[string]any{
				"type":                 "object",
				"additionalProperties": false,
			},
		),
		zhihu_tool_definition(
			get_my_zhihu_collections_tool_name,
			"获取我的知乎收藏夹",
			"使用 cookies.json 中的知乎登录态识别当前账号，获取该账号可见的公开与私密收藏夹列表。调用前会验证 z_c0 Cookie 和当前登录账号。",
			map[string]any{
				"type":                 "object",
				"additionalProperties": false,
			},
		),
		zhihu_tool_definition(
			get_zhihu_collection_contents_tool_name,
			"获取知乎收藏夹内容",
			"按 collection_id 获取指定知乎收藏夹的一页内容。page 从 1 开始；响应 has_next=true 时，应将 next_page 作为下一次调用的 page。调用前会验证 cookies.json 中的知乎登录态。",
			map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"collection_id": map[string]any{
						"type":        "string",
						"pattern":     "^[0-9]+$",
						"description": "知乎收藏夹 ID，例如收藏夹链接 /collection/972293341 中的 972293341。",
					},
					"page": map[string]any{
						"type":        "integer",
						"minimum":     1,
						"maximum":     1000000,
						"default":     1,
						"description": "页码，从 1 开始。",
					},
				},
				"required": []string{"collection_id"},
			},
		),
		zhihu_tool_definition(
			get_my_zhihu_answers_tool_name,
			"获取我的知乎回答",
			"使用 cookies.json 中的知乎登录态识别当前账号，分页获取该账号发布的回答。page 从 1 开始；响应 has_next=true 时，应将 next_page 作为下一次调用的 page。",
			zhihu_page_input_schema(),
		),
		zhihu_tool_definition(
			get_my_zhihu_posts_tool_name,
			"获取我的知乎文章",
			"使用 cookies.json 中的知乎登录态识别当前账号，分页获取该账号发布的文章。page 从 1 开始；响应 has_next=true 时，应将 next_page 作为下一次调用的 page。",
			zhihu_page_input_schema(),
		),
		zhihu_tool_definition(
			get_my_zhihu_zvideos_tool_name,
			"获取我的知乎视频",
			"使用 cookies.json 中的知乎登录态识别当前账号，分页获取该账号发布的视频。page 从 1 开始；响应 has_next=true 时，应将 next_page 作为下一次调用的 page。",
			zhihu_page_input_schema(),
		),
		zhihu_tool_definition(
			get_my_zhihu_columns_tool_name,
			"获取我的知乎专栏",
			"使用 cookies.json 中的知乎登录态识别当前账号，分页获取该账号参与的专栏。page 从 1 开始；响应 has_next=true 时，应将 next_page 作为下一次调用的 page。",
			zhihu_page_input_schema(),
		),
	}
}

func zhihu_page_input_schema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"page": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"maximum":     1000000,
				"default":     1,
				"description": "页码，从 1 开始。",
			},
		},
	}
}

func zhihu_tool_definition(name string, title string, description string, input_schema map[string]any) map[string]any {
	return map[string]any{
		"name":        name,
		"title":       title,
		"description": description,
		"inputSchema": input_schema,
		"annotations": map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
			"openWorldHint":   true,
		},
	}
}

func (s *Server) get_zhihu_credential_status(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments struct{}
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.check_local_zhihu_credential(); err != nil {
		return successful_tool_result(map[string]any{
			"cookie_present": false,
			"authenticated":  false,
			"message":        err.Error(),
		})
	}
	user, err := s.zhihu_collections.FetchCurrentUser()
	if err != nil {
		return successful_tool_result(map[string]any{
			"cookie_present": true,
			"authenticated":  false,
			"message":        "知乎登录凭证验证失败，请更新 cookies.json 后重试: " + err.Error(),
		})
	}
	url_token := zhihu_user_url_token(user)
	if url_token == "" {
		return successful_tool_result(map[string]any{
			"cookie_present": true,
			"authenticated":  false,
			"message":        "知乎当前用户响应缺少 url_token，请更新 cookies.json 后重试",
		})
	}
	return successful_tool_result(map[string]any{
		"cookie_present": true,
		"authenticated":  true,
		"user": map[string]any{
			"id":        user.ID,
			"name":      user.Name,
			"url_token": url_token,
		},
	})
}

func (s *Server) get_my_zhihu_collections(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments struct{}
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	user, err := s.authenticated_zhihu_user(ctx)
	if err != nil {
		return nil, err
	}
	collections_url := zhihu.SourceURL + "people/" + url.PathEscape(zhihu_user_url_token(user)) + "/collections"
	result, err := s.zhihu_collections.FetchCollectionList(collections_url)
	if err != nil {
		return nil, fmt.Errorf("获取我的知乎收藏夹失败: %w", err)
	}
	return successful_tool_result(result)
}

func (s *Server) get_zhihu_collection_contents(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments zhihu_collection_contents_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	collection_id := strings.TrimSpace(arguments.CollectionID)
	if collection_id == "" {
		return nil, fmt.Errorf("collection_id 不能为空")
	}
	for _, character := range collection_id {
		if character < '0' || character > '9' {
			return nil, fmt.Errorf("collection_id 必须只包含数字")
		}
	}
	page := arguments.Page
	if page == 0 {
		page = 1
	}
	if page < 1 || page > 1000000 {
		return nil, fmt.Errorf("page 必须在 1 到 1000000 之间")
	}
	if _, err := s.authenticated_zhihu_user(ctx); err != nil {
		return nil, err
	}
	result, err := s.zhihu_collections.FetchContentListOfCollection(collection_id, page)
	if err != nil {
		return nil, fmt.Errorf("获取知乎收藏夹 %s 第 %d 页失败: %w", collection_id, page, err)
	}
	return successful_tool_result(result)
}

func (s *Server) get_my_zhihu_answers(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	return s.get_my_zhihu_user_content(ctx, raw_arguments, zhihu.UserContentKindAnswers)
}

func (s *Server) get_my_zhihu_posts(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	return s.get_my_zhihu_user_content(ctx, raw_arguments, zhihu.UserContentKindPosts)
}

func (s *Server) get_my_zhihu_zvideos(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	return s.get_my_zhihu_user_content(ctx, raw_arguments, zhihu.UserContentKindZvideos)
}

func (s *Server) get_my_zhihu_columns(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	return s.get_my_zhihu_user_content(ctx, raw_arguments, zhihu.UserContentKindColumns)
}

func (s *Server) get_my_zhihu_user_content(ctx context.Context, raw_arguments json.RawMessage, kind string) (map[string]any, error) {
	var arguments zhihu_page_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	page := arguments.Page
	if page == 0 {
		page = 1
	}
	if page < 1 || page > 1000000 {
		return nil, fmt.Errorf("page 必须在 1 到 1000000 之间")
	}

	user, err := s.authenticated_zhihu_user(ctx)
	if err != nil {
		return nil, err
	}
	profile_url := zhihu.SourceURL + "people/" + url.PathEscape(zhihu_user_url_token(user)) + "/" + kind

	var result *zhihu.UserContentList
	var content_name string
	switch kind {
	case zhihu.UserContentKindAnswers:
		content_name = "回答"
		result, err = s.zhihu_collections.FetchAnswerListOfUser(profile_url, page)
	case zhihu.UserContentKindPosts:
		content_name = "文章"
		result, err = s.zhihu_collections.FetchPostListOfUser(profile_url, page)
	case zhihu.UserContentKindZvideos:
		content_name = "视频"
		result, err = s.zhihu_collections.FetchZvideoListOfUser(profile_url, page)
	case zhihu.UserContentKindColumns:
		content_name = "专栏"
		result, err = s.zhihu_collections.FetchColumnListOfUser(profile_url, page)
	default:
		return nil, fmt.Errorf("不支持的知乎用户内容类型: %s", kind)
	}
	if err != nil {
		return nil, fmt.Errorf("获取我的知乎%s第 %d 页失败: %w", content_name, page, err)
	}
	return successful_tool_result(result)
}

func (s *Server) authenticated_zhihu_user(ctx context.Context) (*zhihu.User, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.check_local_zhihu_credential(); err != nil {
		return nil, err
	}
	user, err := s.zhihu_collections.FetchCurrentUser()
	if err != nil {
		return nil, fmt.Errorf("知乎登录凭证验证失败，请更新 cookies.json 后重试: %w", err)
	}
	if zhihu_user_url_token(user) == "" {
		return nil, fmt.Errorf("知乎登录凭证验证失败：当前用户响应缺少 url_token，请更新 cookies.json 后重试")
	}
	return user, nil
}

func (s *Server) check_local_zhihu_credential() error {
	if s == nil || s.zhihu_credentials == nil {
		return fmt.Errorf("知乎凭证读取器未配置")
	}
	cookie_header, err := s.zhihu_credentials.HeaderForURL(zhihu_current_user_url)
	if err != nil {
		if errors.Is(err, cookies.ErrCookieNotFound) {
			return fmt.Errorf("cookies.json 中没有适用于 zhihu.com 的有效登录 Cookie")
		}
		return fmt.Errorf("读取 cookies.json 中的知乎凭证失败: %w", err)
	}
	request := &http.Request{Header: make(http.Header)}
	request.Header.Set("Cookie", cookie_header)
	login_cookie, err := request.Cookie(zhihu_login_cookie_name)
	if err != nil || strings.TrimSpace(login_cookie.Value) == "" {
		return fmt.Errorf("cookies.json 中没有有效的知乎登录凭证 z_c0")
	}
	return nil
}

func zhihu_user_url_token(user *zhihu.User) string {
	if user == nil {
		return ""
	}
	if url_token := strings.TrimSpace(user.URLTokenSnake); url_token != "" {
		return url_token
	}
	return strings.TrimSpace(user.URLToken)
}
