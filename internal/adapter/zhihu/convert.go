package zhihuadapter

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/zhihu"
)

func (h *handler) ToContent(data any) (*model.Content, error) {
	switch page := data.(type) {
	case *zhihu.AnswerPage:
		return ToContent(page)
	case zhihu.AnswerPage:
		return ToContent(&page)
	case *zhihu.QuestionPage:
		return QuestionToContent(page)
	case zhihu.QuestionPage:
		return QuestionToContent(&page)
	case *zhihu.ArticlePage:
		return ArticleToContent(page)
	case zhihu.ArticlePage:
		return ArticleToContent(&page)
	default:
		return nil, fmt.Errorf("unsupported zhihu fetch data type %T", data)
	}
}

func (h *handler) ToAccount(data any) (*model.Account, error) {
	switch page := data.(type) {
	case *zhihu.AnswerPage:
		if page == nil {
			return nil, fmt.Errorf("zhihu answer page is nil")
		}
		return ToAccount(&page.Answer.Author)
	case zhihu.AnswerPage:
		return ToAccount(&page.Answer.Author)
	case *zhihu.QuestionPage:
		if page == nil {
			return nil, fmt.Errorf("zhihu question page is nil")
		}
		return ToAccount(&page.Question.Author)
	case zhihu.QuestionPage:
		return ToAccount(&page.Question.Author)
	case *zhihu.ArticlePage:
		if page == nil {
			return nil, fmt.Errorf("zhihu article page is nil")
		}
		return ToAccount(&page.Article.Author)
	case zhihu.ArticlePage:
		return ToAccount(&page.Article.Author)
	default:
		return nil, fmt.Errorf("unsupported zhihu fetch data type %T", data)
	}
}

func (h *handler) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	content, err := h.ToContent(data)
	if err != nil {
		return nil, err
	}
	root_detail := adapter.ContentDetail{
		Type:    content.Type,
		Key:     content.Id,
		Content: content,
	}
	switch page := data.(type) {
	case *zhihu.AnswerPage:
		return answer_content_details(content, page)
	case zhihu.AnswerPage:
		return answer_content_details(content, &page)
	case *zhihu.QuestionPage:
		return question_content_details(content, page)
	case zhihu.QuestionPage:
		return question_content_details(content, &page)
	case *zhihu.ArticlePage:
		if page != nil {
			root_detail.Data = &model.ContentArticle{Id: content.Id, Type: model.ContentArticleTypeHTML, HTML: page.Article.Content}
		}
	case zhihu.ArticlePage:
		root_detail.Data = &model.ContentArticle{Id: content.Id, Type: model.ContentArticleTypeHTML, HTML: page.Article.Content}
	}
	return []adapter.ContentDetail{root_detail}, nil
}

func answer_content_details(content *model.Content, page *zhihu.AnswerPage) ([]adapter.ContentDetail, error) {
	if page == nil {
		return nil, fmt.Errorf("zhihu answer page is nil")
	}
	answer_detail := adapter.ContentDetail{
		Type:    "answer",
		Key:     content.Id,
		Content: content,
		Data:    &model.ContentArticle{Id: content.Id, Type: model.ContentArticleTypeHTML, HTML: page.Answer.Content},
	}
	question_page := &zhihu.QuestionPage{
		URL: zhihu.QuestionURL{
			QuestionID: page.Question.ID,
			Canonical:  answer_question_url(page),
		},
		Source:   answer_question_url(page),
		Question: page.Question,
	}
	question_content, err := QuestionToContent(question_page)
	if err != nil {
		return nil, err
	}
	question_account, err := ToAccount(&page.Question.Author)
	if err != nil {
		return nil, fmt.Errorf("convert zhihu question author: %w", err)
	}
	question_detail := adapter.ContentDetail{
		Type:    "question",
		Key:     question_content.Id,
		Content: question_content,
		Accounts: []adapter.ContentAccountReference{{
			Account: question_account,
			Role:    "owner",
		}},
		Data: &model.ContentArticle{
			Id:   question_content.Id,
			Type: model.ContentArticleTypeHTML,
			HTML: page.Question.Detail,
		},
		Relation: &model.ContentRelation{
			SourceContentId: content.Id,
			TargetContentId: question_content.Id,
			Type:            model.ContentRelationAnswerOf,
			CreatedAt:       content.CreatedAt,
		},
	}
	return []adapter.ContentDetail{answer_detail, question_detail}, nil
}

func question_content_details(content *model.Content, page *zhihu.QuestionPage) ([]adapter.ContentDetail, error) {
	if page == nil {
		return nil, fmt.Errorf("zhihu question page is nil")
	}
	root_detail := adapter.ContentDetail{
		Type:    "question",
		Key:     content.Id,
		Content: content,
		Data: &model.ContentArticle{
			Id:   content.Id,
			Type: model.ContentArticleTypeHTML,
			HTML: page.Question.Detail,
		},
	}
	details := []adapter.ContentDetail{root_detail}
	if page.InitialData == nil {
		return details, nil
	}

	answers := page.InitialData.InitialState.Entities.Answers
	answer_ids := make([]string, 0, len(answers))
	for answer_id, answer := range answers {
		if strings.TrimSpace(answer.ID) == "" {
			continue
		}
		answer_ids = append(answer_ids, answer_id)
	}
	sort.Strings(answer_ids)
	for answer_order, answer_id := range answer_ids {
		answer := answers[answer_id]
		answer_url := build_answer_url(page.Question.ID, answer.ID)
		answer_page := &zhihu.AnswerPage{
			URL: zhihu.AnswerURL{
				QuestionID: page.Question.ID,
				AnswerID:   answer.ID,
				Canonical:  answer_url,
			},
			Source:   answer_url,
			Question: page.Question,
			Answer:   answer,
		}
		answer_content, err := ToContent(answer_page)
		if err != nil {
			return nil, err
		}
		details = append(details, adapter.ContentDetail{
			Type:    "answer",
			Key:     answer_content.Id,
			Content: answer_content,
			Data: &model.ContentArticle{
				Id:   answer_content.Id,
				Type: model.ContentArticleTypeHTML,
				HTML: answer.Content,
			},
			Relation: &model.ContentRelation{
				SourceContentId: answer_content.Id,
				TargetContentId: content.Id,
				Type:            model.ContentRelationAnswerOf,
				SortOrder:       answer_order,
				CreatedAt:       content.CreatedAt,
			},
		})
	}
	return details, nil
}

func build_answer_url(question_id, answer_id string) string {
	return "https://www.zhihu.com/question/" + url.PathEscape(strings.TrimSpace(question_id)) +
		"/answer/" + url.PathEscape(strings.TrimSpace(answer_id))
}

func zhihu_page_from_fetch(data any) (any, error) {
	var raw_json json.RawMessage
	switch value := data.(type) {
	case json.RawMessage:
		raw_json = value
	case *json.RawMessage:
		if value == nil {
			return nil, fmt.Errorf("zhihu fetch JSON is nil")
		}
		raw_json = *value
	case []byte:
		raw_json = json.RawMessage(value)
	default:
		return data, nil
	}

	if len(strings.TrimSpace(string(raw_json))) == 0 {
		return nil, fmt.Errorf("zhihu fetch JSON is empty")
	}
	if page, ok := parse_zhihu_page_content(raw_json); ok {
		return page, nil
	}

	var decoded any
	if err := json.Unmarshal(raw_json, &decoded); err != nil {
		return nil, fmt.Errorf("decode zhihu fetch data: %w", err)
	}
	return nil, fmt.Errorf("unsupported zhihu fetch JSON")
}

// BuildDownloadTaskFromFetch builds a task directly from the structured page,
// preserving any video play info already collected during fetch.
func (h *handler) BuildDownloadTaskFromFetch(data any, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	page_data, err := zhihu_page_from_fetch(data)
	if err != nil {
		return nil, err
	}
	var config map[string]any
	if err := json.Unmarshal(config_json, &config); err != nil {
		return nil, fmt.Errorf("解析下载配置失败: %w", err)
	}
	return h.build_download_task(page_data, config, "", "")
}
