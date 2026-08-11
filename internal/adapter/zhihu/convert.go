package zhihuadapter

import (
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
	html_content := ""
	switch page := data.(type) {
	case *zhihu.AnswerPage:
		if page != nil {
			html_content = page.Answer.Content
		}
	case zhihu.AnswerPage:
		html_content = page.Answer.Content
	case *zhihu.QuestionPage:
		if page != nil {
			html_content = page.Question.Detail
		}
	case zhihu.QuestionPage:
		html_content = page.Question.Detail
	case *zhihu.ArticlePage:
		if page != nil {
			html_content = page.Article.Content
		}
	case zhihu.ArticlePage:
		html_content = page.Article.Content
	}
	return []adapter.ContentDetail{{
		Type: content.Type,
		Key:  content.Id,
		Data: &model.ContentArticle{Id: content.Id, Type: model.ContentArticleTypeHTML, HTML: html_content},
	}}, nil
}
