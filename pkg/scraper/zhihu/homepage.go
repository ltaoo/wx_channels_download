package zhihu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"wx_channel/pkg/minib"
)

const (
	user_content_page_size = 20
	max_user_content_page  = 1000000
)

var user_content_list_url_re = regexp.MustCompile(`^/people/([^/]+)/(answers|posts|zvideos|columns)/?$`)

type user_content_list_url struct {
	OwnerURLToken string
	Kind          string
	Page          int
	Canonical     string
}

type user_content_api_response struct {
	Data           []user_content_api_item `json:"data"`
	Paging         user_content_api_paging `json:"paging"`
	NeedForceLogin bool                    `json:"need_force_login"`
}

type user_content_api_paging struct {
	IsEnd  bool   `json:"is_end"`
	Totals int    `json:"totals"`
	Next   string `json:"next"`
}

type user_content_api_item struct {
	ID                 string                  `json:"id"`
	Type               string                  `json:"type"`
	Title              string                  `json:"title"`
	Excerpt            string                  `json:"excerpt"`
	Description        string                  `json:"description"`
	ImageURL           string                  `json:"image_url"`
	Created            int64                   `json:"created"`
	CreatedTime        int64                   `json:"created_time"`
	Updated            int64                   `json:"updated"`
	UpdatedTime        int64                   `json:"updated_time"`
	UpdatedAt          int64                   `json:"updated_at"`
	PublishedAt        int64                   `json:"published_at"`
	CommentCount       int                     `json:"comment_count"`
	VoteupCount        int                     `json:"voteup_count"`
	FavlistsCount      int                     `json:"favlists_count"`
	PlayCount          int                     `json:"play_count"`
	ContributionsCount int                     `json:"contributions_count"`
	Question           user_content_question   `json:"question"`
	Column             user_content_api_column `json:"column"`
}

type user_content_question struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type user_content_api_column struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	Title         string `json:"title"`
	Excerpt       string `json:"excerpt"`
	Intro         string `json:"intro"`
	ImageURL      string `json:"image_url"`
	Updated       int64  `json:"updated"`
	VoteupCount   int    `json:"voteup_count"`
	Followers     int    `json:"followers"`
	ArticlesCount int    `json:"articles_count"`
	ItemsCount    int    `json:"items_count"`
}

func parse_user_content_list_url(raw_url string) (user_content_list_url, bool, error) {
	parsed_url, err := url.Parse(strings.TrimSpace(ResolveRealURL(raw_url)))
	if err != nil || parsed_url.Hostname() == "" {
		return user_content_list_url{}, false, nil
	}
	if parsed_url.Scheme != "https" && parsed_url.Scheme != "http" {
		return user_content_list_url{}, false, nil
	}
	if !strings.EqualFold(parsed_url.Hostname(), "www.zhihu.com") {
		return user_content_list_url{}, false, nil
	}
	matches := user_content_list_url_re.FindStringSubmatch(parsed_url.EscapedPath())
	if len(matches) != 3 {
		return user_content_list_url{}, false, nil
	}
	owner_url_token, err := url.PathUnescape(matches[1])
	if err != nil || strings.TrimSpace(owner_url_token) == "" || strings.Contains(owner_url_token, "/") {
		return user_content_list_url{}, true, fmt.Errorf("invalid zhihu user token")
	}
	page := 1
	if raw_page := strings.TrimSpace(parsed_url.Query().Get("page")); raw_page != "" {
		page, err = strconv.Atoi(raw_page)
		if err != nil || page < 1 || page > max_user_content_page {
			return user_content_list_url{}, true, fmt.Errorf("zhihu user content page must be between 1 and %d", max_user_content_page)
		}
	}
	kind := matches[2]
	return user_content_list_url{
		OwnerURLToken: owner_url_token,
		Kind:          kind,
		Page:          page,
		Canonical:     canonical_user_content_list_url(owner_url_token, kind, page),
	}, true, nil
}

func canonical_user_content_list_url(owner_url_token string, kind string, page int) string {
	canonical := SourceURL + "people/" + url.PathEscape(strings.TrimSpace(owner_url_token)) + "/" + kind
	if page > 1 {
		canonical += "?page=" + strconv.Itoa(page)
	}
	return canonical
}

func (c *Client) fetch_user_content_list_of_kind(raw_url string, expected_kind string, page int) (*UserContentList, error) {
	user_content_url, matched, err := parse_user_content_list_url(raw_url)
	if !matched {
		return nil, fmt.Errorf("unsupported zhihu %s list URL", expected_kind)
	}
	if err != nil {
		return nil, err
	}
	if user_content_url.Kind != expected_kind {
		return nil, fmt.Errorf("expected zhihu %s list URL, got %s", expected_kind, user_content_url.Kind)
	}
	if page < 1 || page > max_user_content_page {
		return nil, fmt.Errorf("zhihu user content page must be between 1 and %d", max_user_content_page)
	}
	user_content_url.Page = page
	user_content_url.Canonical = canonical_user_content_list_url(user_content_url.OwnerURLToken, user_content_url.Kind, page)
	return c.fetch_user_content_list(user_content_url)
}

func (c *Client) fetch_user_content_list(user_content_url user_content_list_url) (*UserContentList, error) {
	if c == nil {
		return nil, fmt.Errorf("zhihu client is nil")
	}
	endpoint, err := user_content_api_endpoint(user_content_url)
	if err != nil {
		return nil, err
	}
	var body []byte
	if c.profile_api_fetcher != nil {
		body, err = c.profile_api_fetcher(endpoint, user_content_url.Canonical)
	} else {
		body, err = c.do_profile_api_bytes(endpoint, user_content_url.Canonical)
	}
	if err != nil {
		return nil, fmt.Errorf(
			"fetch zhihu user %s page %d for %s: %w",
			user_content_url.Kind,
			user_content_url.Page,
			user_content_url.OwnerURLToken,
			err,
		)
	}
	var response user_content_api_response
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("parse zhihu user %s response: %w", user_content_url.Kind, err)
	}
	if response.NeedForceLogin {
		return nil, fmt.Errorf("authenticated zhihu cookies are required")
	}
	items := make([]UserContentItem, 0, len(response.Data))
	for _, api_item := range response.Data {
		item, ok := normalize_user_content_item(user_content_url.Kind, api_item)
		if ok {
			items = append(items, item)
		}
	}
	has_next := !response.Paging.IsEnd && len(response.Data) > 0
	result := &UserContentList{
		Source:        user_content_url.Canonical,
		OwnerURLToken: user_content_url.OwnerURLToken,
		Kind:          user_content_url.Kind,
		Page:          user_content_url.Page,
		PageSize:      user_content_page_size,
		TotalCount:    response.Paging.Totals,
		Items:         items,
		HasNext:       has_next,
	}
	if has_next {
		result.NextPage = user_content_url.Page + 1
	}
	return result, nil
}

func (c *Client) do_profile_api_bytes(endpoint string, referer string) ([]byte, error) {
	if !strings.HasPrefix(endpoint, "/") {
		return nil, fmt.Errorf("invalid zhihu profile api endpoint")
	}
	timeout := 2 * time.Minute
	if c != nil && c.http_client != nil && c.http_client.Timeout > 0 {
		timeout = c.http_client.Timeout
	}
	browser, err := minib.NewMiniBrowser(timeout, c.cookie_reader)
	if err != nil {
		return nil, fmt.Errorf("create minib browser: %w", err)
	}
	defer browser.Close()

	api_url := SourceURL + strings.TrimPrefix(endpoint, "/")
	headers := make(http.Header)
	headers.Set("Accept", "application/json, text/plain, */*")
	headers.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Pragma", "no-cache")
	headers.Set("Sec-Fetch-Dest", "empty")
	headers.Set("Sec-Fetch-Mode", "cors")
	headers.Set("Sec-Fetch-Site", "same-origin")
	headers.Set("X-Requested-With", "fetch")
	if strings.TrimSpace(referer) != "" {
		headers.Set("Referer", referer)
	}
	request_context, cancel_request := context.WithTimeout(context.Background(), timeout)
	defer cancel_request()
	response, err := browser.Get(request_context, api_url, headers)
	if err != nil {
		return nil, err
	}
	c.log_response(http.MethodGet, api_url, response.StatusCode)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("zhihu profile api status %d body=%s", response.StatusCode, debug_snippet(response.Body))
	}
	if c.OnProgress != nil {
		c.OnProgress(int64(len(response.Body)))
	}
	return response.Body, nil
}

func user_content_api_endpoint(user_content_url user_content_list_url) (string, error) {
	if user_content_url.Page < 1 || user_content_url.Page > max_user_content_page {
		return "", fmt.Errorf("zhihu user content page must be between 1 and %d", max_user_content_page)
	}
	endpoint_name := ""
	query := url.Values{
		"limit":  []string{strconv.Itoa(user_content_page_size)},
		"offset": []string{strconv.Itoa((user_content_url.Page - 1) * user_content_page_size)},
	}
	switch user_content_url.Kind {
	case UserContentKindAnswers:
		endpoint_name = "answers"
		query.Set("sort_by", "created")
		query.Set("include", "data[*].excerpt,comment_count,created_time,updated_time;data[*].question.title")
	case UserContentKindPosts:
		endpoint_name = "articles"
		query.Set("sort_by", "created")
		query.Set("include", "data[*].excerpt,comment_count,voteup_count,created,updated")
	case UserContentKindZvideos:
		endpoint_name = "zvideos"
		query.Set("similar_aggregation", "true")
	case UserContentKindColumns:
		endpoint_name = "column-contributions"
		query.Set("include", "data[*].column.intro,followers,articles_count,voteup_count,items_count")
	default:
		return "", fmt.Errorf("unsupported zhihu user content kind %q", user_content_url.Kind)
	}
	return "/api/v4/members/" + url.PathEscape(user_content_url.OwnerURLToken) + "/" + endpoint_name + "?" + query.Encode(), nil
}

func normalize_user_content_item(kind string, api_item user_content_api_item) (UserContentItem, bool) {
	switch kind {
	case UserContentKindAnswers:
		if strings.TrimSpace(api_item.ID) == "" {
			return UserContentItem{}, false
		}
		return UserContentItem{
			ID:           api_item.ID,
			Type:         first_non_empty(api_item.Type, "answer"),
			Title:        strings.TrimSpace(api_item.Question.Title),
			URL:          canonical_answer_url(api_item.Question.ID, api_item.ID),
			Excerpt:      strings.TrimSpace(html_to_text(api_item.Excerpt)),
			QuestionID:   api_item.Question.ID,
			AnswerID:     api_item.ID,
			CreatedAt:    api_item.CreatedTime,
			UpdatedAt:    api_item.UpdatedTime,
			CommentCount: api_item.CommentCount,
			VoteupCount:  api_item.VoteupCount,
		}, true
	case UserContentKindPosts:
		if strings.TrimSpace(api_item.ID) == "" {
			return UserContentItem{}, false
		}
		return UserContentItem{
			ID:           api_item.ID,
			Type:         first_non_empty(api_item.Type, "article"),
			Title:        strings.TrimSpace(api_item.Title),
			URL:          "https://zhuanlan.zhihu.com/p/" + url.PathEscape(api_item.ID),
			Excerpt:      strings.TrimSpace(html_to_text(api_item.Excerpt)),
			ImageURL:     strings.TrimSpace(api_item.ImageURL),
			ArticleID:    api_item.ID,
			CreatedAt:    api_item.Created,
			UpdatedAt:    api_item.Updated,
			CommentCount: api_item.CommentCount,
			VoteupCount:  api_item.VoteupCount,
		}, true
	case UserContentKindZvideos:
		if strings.TrimSpace(api_item.ID) == "" {
			return UserContentItem{}, false
		}
		return UserContentItem{
			ID:            api_item.ID,
			Type:          first_non_empty(api_item.Type, "zvideo"),
			Title:         strings.TrimSpace(api_item.Title),
			URL:           SourceURL + "zvideo/" + url.PathEscape(api_item.ID),
			Excerpt:       strings.TrimSpace(first_non_empty(api_item.Excerpt, api_item.Description)),
			ImageURL:      strings.TrimSpace(api_item.ImageURL),
			ZvideoID:      api_item.ID,
			CreatedAt:     api_item.PublishedAt,
			UpdatedAt:     api_item.UpdatedAt,
			CommentCount:  api_item.CommentCount,
			VoteupCount:   api_item.VoteupCount,
			FavoriteCount: api_item.FavlistsCount,
			PlayCount:     api_item.PlayCount,
		}, true
	case UserContentKindColumns:
		column := api_item.Column
		if strings.TrimSpace(column.ID) == "" {
			return UserContentItem{}, false
		}
		content_count := column.ArticlesCount
		if content_count == 0 {
			content_count = column.ItemsCount
		}
		return UserContentItem{
			ID:                 column.ID,
			Type:               first_non_empty(column.Type, "column"),
			Title:              strings.TrimSpace(column.Title),
			URL:                SourceURL + "column/" + url.PathEscape(column.ID),
			Excerpt:            strings.TrimSpace(first_non_empty(column.Intro, column.Excerpt)),
			ImageURL:           strings.TrimSpace(column.ImageURL),
			ColumnID:           column.ID,
			UpdatedAt:          column.Updated,
			VoteupCount:        column.VoteupCount,
			FollowerCount:      column.Followers,
			ContentCount:       content_count,
			ContributionsCount: api_item.ContributionsCount,
		}, true
	default:
		return UserContentItem{}, false
	}
}
