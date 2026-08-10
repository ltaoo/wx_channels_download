package zhihuadapter

import (
	"fmt"

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
