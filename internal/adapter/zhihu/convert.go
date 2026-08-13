package zhihuadapter

import (
	"encoding/json"
	"fmt"

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
		if page != nil {
			root_detail.Data = &model.ContentArticle{Id: content.Id, Type: model.ContentArticleTypeHTML, HTML: page.Question.Detail}
		}
	case zhihu.QuestionPage:
		root_detail.Data = &model.ContentArticle{Id: content.Id, Type: model.ContentArticleTypeHTML, HTML: page.Question.Detail}
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
	question_detail := adapter.ContentDetail{
		Type:    "question",
		Key:     question_content.Id,
		Content: question_content,
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

// BuildDownloadTaskFromFetch serializes a supported structured page and lets
// BuildDownloadTask produce the same task shape used by normal task creation.
func (h *handler) BuildDownloadTaskFromFetch(data any, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	if _, err := h.ToContent(data); err != nil {
		return nil, err
	}
	content_json, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("encode zhihu download task content: %w", err)
	}
	return h.BuildDownloadTask(content_json, config_json)
}
