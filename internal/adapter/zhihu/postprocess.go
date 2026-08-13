package zhihuadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"wx_channel/internal/adapter"
	"wx_channel/pkg/hermes"
	"wx_channel/pkg/scraper/zhihu"
)

const (
	postprocess_type_answer   = "answer"
	postprocess_type_question = "question"
	postprocess_type_article  = "article"
	postprocess_marker_key    = "zhihu_postprocess"
	postprocess_marker_value  = "page_to_html"
)

var _ adapter.Postprocessor = (*handler)(nil)

// postprocess_payload is the scraper output persisted as an inline resource.
// Hermes downloads this DTO first; Postprocess turns it into the final HTML file.
type postprocess_payload struct {
	Type     string              `json:"type"`
	Answer   *zhihu.AnswerPage   `json:"answer,omitempty"`
	Question *zhihu.QuestionPage `json:"question,omitempty"`
	Article  *zhihu.ArticlePage  `json:"article,omitempty"`
}

func marshal_postprocess_payload(page_data any) ([]byte, error) {
	payload := postprocess_payload{}
	switch page := page_data.(type) {
	case *zhihu.AnswerPage:
		if page == nil {
			return nil, fmt.Errorf("zhihu answer page is nil")
		}
		page_copy := compact_answer_page(*page)
		payload.Type = postprocess_type_answer
		payload.Answer = &page_copy
	case zhihu.AnswerPage:
		page_copy := compact_answer_page(page)
		payload.Type = postprocess_type_answer
		payload.Answer = &page_copy
	case *zhihu.QuestionPage:
		if page == nil {
			return nil, fmt.Errorf("zhihu question page is nil")
		}
		page_copy := compact_question_page(*page)
		payload.Type = postprocess_type_question
		payload.Question = &page_copy
	case zhihu.QuestionPage:
		page_copy := compact_question_page(page)
		payload.Type = postprocess_type_question
		payload.Question = &page_copy
	case *zhihu.ArticlePage:
		if page == nil {
			return nil, fmt.Errorf("zhihu article page is nil")
		}
		page_copy := compact_article_page(*page)
		payload.Type = postprocess_type_article
		payload.Article = &page_copy
	case zhihu.ArticlePage:
		page_copy := compact_article_page(page)
		payload.Type = postprocess_type_article
		payload.Article = &page_copy
	default:
		return nil, fmt.Errorf("unsupported zhihu postprocess data type %T", page_data)
	}
	return json.Marshal(payload)
}

func compact_answer_page(page zhihu.AnswerPage) zhihu.AnswerPage {
	page.PageHTML = ""
	page.InitialData = nil
	page.InitialDataJSON = nil
	return page
}

func compact_question_page(page zhihu.QuestionPage) zhihu.QuestionPage {
	page.PageHTML = ""
	page.InitialData = nil
	page.InitialDataJSON = nil
	return page
}

func compact_article_page(page zhihu.ArticlePage) zhihu.ArticlePage {
	page.PageHTML = ""
	page.InitialData = nil
	page.InitialDataJSON = nil
	return page
}

// Postprocess converts downloaded Zhihu page data into a standalone HTML document.
func (h *handler) Postprocess(ctx context.Context, info *hermes.TaskJob, deps adapter.PostprocessDeps) error {
	if info == nil {
		return fmt.Errorf("zhihu postprocess: task is nil")
	}
	for resource_index := range info.Resources {
		if err := context.Cause(ctx); err != nil {
			return err
		}
		resource := &info.Resources[resource_index]
		if resource.Extra[postprocess_marker_key] != postprocess_marker_value {
			continue
		}
		file_path := strings.TrimSpace(resource.FilePath)
		if file_path == "" {
			return fmt.Errorf("zhihu postprocess: resource %d has no downloaded file", resource.ID)
		}
		page_data, err := os.ReadFile(file_path)
		if err != nil {
			return fmt.Errorf("zhihu postprocess: read resource %q: %w", resource.Name, err)
		}
		var payload postprocess_payload
		if err := json.Unmarshal(page_data, &payload); err != nil {
			return fmt.Errorf("zhihu postprocess: decode resource %q: %w", resource.Name, err)
		}
		html_content, source_url, err := build_payload_html(payload)
		if err != nil {
			return fmt.Errorf("zhihu postprocess: build resource %q: %w", resource.Name, err)
		}
		html_content, err = h.scraper_client().InlineRemoteImages(html_content, source_url)
		if err != nil {
			return fmt.Errorf("zhihu postprocess: inline images for resource %q: %w", resource.Name, err)
		}
		processed_data := []byte(html_content)
		if err := replace_postprocess_file(file_path, processed_data); err != nil {
			return fmt.Errorf("zhihu postprocess: write resource %q: %w", resource.Name, err)
		}

		resource.Name = strings.TrimSpace(info.Name)
		if resource.Name == "" {
			resource.Name = "知乎内容"
		}
		resource.Kind = "text/html"
		resource.Size = int64(len(processed_data))
		resource.Downloaded = resource.Size
		resource.Extra["postprocessed"] = "true"
		deps.Logger.Info().
			Int("task_id", info.ID).
			Int("resource_id", resource.ID).
			Str("resource_name", resource.Name).
			Int64("resource_size", resource.Size).
			Msg("Postprocessor.zhihu: page data converted to HTML")
	}
	return nil
}

func build_payload_html(payload postprocess_payload) (string, string, error) {
	switch payload.Type {
	case postprocess_type_answer:
		if payload.Answer == nil {
			return "", "", fmt.Errorf("answer page is empty")
		}
		return build_html(payload.Answer), payload.Answer.Source, nil
	case postprocess_type_question:
		if payload.Question == nil {
			return "", "", fmt.Errorf("question page is empty")
		}
		return build_question_html(payload.Question), payload.Question.Source, nil
	case postprocess_type_article:
		if payload.Article == nil {
			return "", "", fmt.Errorf("article page is empty")
		}
		return build_article_html(payload.Article), payload.Article.Source, nil
	default:
		return "", "", fmt.Errorf("unsupported page type %q", payload.Type)
	}
}

func build_html(page *zhihu.AnswerPage) string {
	if page == nil {
		return ""
	}
	var builder strings.Builder
	title := strings.TrimSpace(page.Question.Title)
	if title == "" {
		title = "知乎回答"
	}
	builder.WriteString("<!doctype html><html lang=\"zh-CN\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>")
	builder.WriteString(html.EscapeString(title))
	builder.WriteString("</title><style>")
	builder.WriteString(`body{margin:0;background:#f6f6f6;color:#1f2329;font:16px/1.75 -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}main{max-width:760px;margin:0 auto;padding:32px 18px 56px;background:#fff;min-height:100vh}h1{font-size:28px;line-height:1.35;margin:0 0 12px}h2{font-size:20px;margin:34px 0 12px;border-top:1px solid #e7e9ee;padding-top:24px}.meta{color:#69707a;font-size:14px;margin:0 0 18px}.author{display:flex;gap:12px;align-items:center;margin:0 0 18px;color:#69707a;font-size:14px}.avatar{width:42px;height:42px;border-radius:50%;object-fit:cover;background:#edf0f3;flex:0 0 auto}.author-name{font-weight:600;color:#1f2329}.content p{margin:0 0 14px}.content img{max-width:100%;height:auto}.comment{border-top:1px solid #edf0f3;padding:14px 0}.reply{margin-left:18px;border-left:3px solid #edf0f3;padding-left:12px}.source{word-break:break-all}a{color:#175199;text-decoration:none}a:hover{text-decoration:underline}`)
	builder.WriteString("</style></head><body><main><h1>")
	builder.WriteString(html.EscapeString(title))
	builder.WriteString("</h1><p class=\"meta\">问题作者：")
	builder.WriteString(html.EscapeString(zhihu.UserDisplayName(page.Question.Author)))
	builder.WriteString(" · 问题原始链接：<a href=\"")
	builder.WriteString(html.EscapeString(answer_question_url(page)))
	builder.WriteString("\">")
	builder.WriteString(html.EscapeString(answer_question_url(page)))
	builder.WriteString("</a></p>")
	if strings.TrimSpace(page.Question.Detail) != "" {
		builder.WriteString("<section class=\"content\">")
		builder.WriteString(sanitize_fragment(page.Question.Detail))
		builder.WriteString("</section>")
	} else if strings.TrimSpace(page.Question.Excerpt) != "" {
		builder.WriteString("<p>")
		builder.WriteString(html.EscapeString(page.Question.Excerpt))
		builder.WriteString("</p>")
	}
	builder.WriteString("<h2>回答</h2>")
	write_author_block(&builder, "回答作者", page.Answer.Author, page.Source)
	if page.Answer.CreatedTime > 0 {
		builder.WriteString("<p class=\"meta\">发布于 ")
		builder.WriteString(html.EscapeString(format_time(page.Answer.CreatedTime)))
		builder.WriteString("</p>")
	}
	builder.WriteString("<section class=\"content\">")
	builder.WriteString(sanitize_fragment(page.Answer.Content))
	builder.WriteString("</section>")
	if len(page.Comments) > 0 {
		builder.WriteString("<h2>回答评论</h2>")
		for _, comment := range page.Comments {
			write_comment(&builder, comment, false)
		}
	}
	write_source(&builder, page.Source)
	return builder.String()
}

func build_question_html(page *zhihu.QuestionPage) string {
	if page == nil {
		return ""
	}
	var builder strings.Builder
	title := strings.TrimSpace(page.Question.Title)
	if title == "" {
		title = "知乎问题"
	}
	write_document_start(&builder, title)
	write_author_block(&builder, "问题作者", page.Question.Author, page.Source)
	if strings.TrimSpace(page.Question.Detail) != "" {
		builder.WriteString("<section class=\"content\">")
		builder.WriteString(sanitize_fragment(page.Question.Detail))
		builder.WriteString("</section>")
	} else if strings.TrimSpace(page.Question.Excerpt) != "" {
		builder.WriteString("<p>")
		builder.WriteString(html.EscapeString(page.Question.Excerpt))
		builder.WriteString("</p>")
	}
	write_source(&builder, page.Source)
	return builder.String()
}

func build_article_html(page *zhihu.ArticlePage) string {
	if page == nil {
		return ""
	}
	var builder strings.Builder
	title := strings.TrimSpace(page.Article.Title)
	if title == "" {
		title = "知乎文章"
	}
	write_document_start(&builder, title)
	write_author_block(&builder, "文章作者", page.Article.Author, page.Source)
	if page.Article.CreatedTime > 0 {
		builder.WriteString("<p class=\"meta\">发布于 ")
		builder.WriteString(html.EscapeString(format_time(page.Article.CreatedTime)))
		builder.WriteString("</p>")
	}
	if strings.TrimSpace(page.Article.Content) != "" {
		builder.WriteString("<section class=\"content\">")
		builder.WriteString(sanitize_fragment(page.Article.Content))
		builder.WriteString("</section>")
	} else if strings.TrimSpace(page.Article.Excerpt) != "" {
		builder.WriteString("<p>")
		builder.WriteString(html.EscapeString(page.Article.Excerpt))
		builder.WriteString("</p>")
	}
	write_source(&builder, page.Source)
	return builder.String()
}

func write_document_start(builder *strings.Builder, title string) {
	builder.WriteString("<!doctype html><html lang=\"zh-CN\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>")
	builder.WriteString(html.EscapeString(title))
	builder.WriteString("</title><style>")
	builder.WriteString(`body{margin:0;background:#f6f6f6;color:#1f2329;font:16px/1.75 -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}main{max-width:760px;margin:0 auto;padding:32px 18px 56px;background:#fff;min-height:100vh}h1{font-size:28px;line-height:1.35;margin:0 0 12px}.meta{color:#69707a;font-size:14px;margin:0 0 18px}.author{display:flex;gap:12px;align-items:center;margin:0 0 18px;color:#69707a;font-size:14px}.avatar{width:42px;height:42px;border-radius:50%;object-fit:cover;background:#edf0f3;flex:0 0 auto}.author-name{font-weight:600;color:#1f2329}.content p{margin:0 0 14px}.content img{max-width:100%;height:auto}.source{word-break:break-all}a{color:#175199;text-decoration:none}a:hover{text-decoration:underline}`)
	builder.WriteString("</style></head><body><main><h1>")
	builder.WriteString(html.EscapeString(title))
	builder.WriteString("</h1>")
}

func write_source(builder *strings.Builder, source_url string) {
	builder.WriteString("<h2>来源</h2><p class=\"source\"><a href=\"")
	builder.WriteString(html.EscapeString(source_url))
	builder.WriteString("\">")
	builder.WriteString(html.EscapeString(source_url))
	builder.WriteString("</a></p></main></body></html>")
}

func write_comment(builder *strings.Builder, comment zhihu.Comment, reply bool) {
	class_name := "comment"
	if reply {
		class_name += " reply"
	}
	builder.WriteString("<div class=\"")
	builder.WriteString(class_name)
	builder.WriteString("\"><p class=\"meta\">评论作者：")
	builder.WriteString(html.EscapeString(zhihu.UserDisplayName(comment.Author)))
	if comment.ReplyTo != nil && zhihu.UserDisplayName(*comment.ReplyTo) != "" {
		builder.WriteString(" 回复 ")
		builder.WriteString(html.EscapeString(zhihu.UserDisplayName(*comment.ReplyTo)))
	}
	builder.WriteString("</p><div class=\"content\">")
	if comment.ContentHTML != "" {
		builder.WriteString(sanitize_fragment(comment.ContentHTML))
	} else {
		builder.WriteString("<p>")
		builder.WriteString(html.EscapeString(comment.ContentText))
		builder.WriteString("</p>")
	}
	builder.WriteString("</div>")
	for _, child := range comment.Replies {
		write_comment(builder, child, true)
	}
	builder.WriteString("</div>")
}

func write_author_block(builder *strings.Builder, label string, user zhihu.User, fallback_url string) {
	profile_url := zhihu.UserURL(user)
	if profile_url == "" {
		profile_url = fallback_url
	}
	display_name := zhihu.UserDisplayName(user)
	builder.WriteString("<div class=\"author\">")
	if avatar := zhihu.UserAvatarURL(user); avatar != "" {
		builder.WriteString("<img class=\"avatar\" src=\"")
		builder.WriteString(html.EscapeString(avatar))
		builder.WriteString("\" alt=\"")
		builder.WriteString(html.EscapeString(display_name))
		builder.WriteString("\">")
	}
	builder.WriteString("<div>")
	builder.WriteString(html.EscapeString(label))
	builder.WriteString("：")
	if profile_url != "" {
		builder.WriteString("<a class=\"author-name\" href=\"")
		builder.WriteString(html.EscapeString(profile_url))
		builder.WriteString("\">")
		builder.WriteString(html.EscapeString(display_name))
		builder.WriteString("</a>")
	} else {
		builder.WriteString("<span class=\"author-name\">")
		builder.WriteString(html.EscapeString(display_name))
		builder.WriteString("</span>")
	}
	if strings.TrimSpace(user.Headline) != "" {
		builder.WriteString("<br>")
		builder.WriteString(html.EscapeString(user.Headline))
	}
	builder.WriteString("</div></div>")
}

func sanitize_fragment(fragment string) string {
	document, err := goquery.NewDocumentFromReader(strings.NewReader("<div id=\"wx-zhihu-root\">" + fragment + "</div>"))
	if err != nil {
		return html.EscapeString(html_to_text(fragment))
	}
	root := document.Find("#wx-zhihu-root")
	root.Find("script,style,iframe,button,svg").Remove()
	root.Find("img").Each(func(_ int, selection *goquery.Selection) {
		if source := best_zhihu_image_source(selection); source != "" {
			selection.SetAttr("src", source)
		}
	})
	root.Find("*").Each(func(_ int, selection *goquery.Selection) {
		for _, node := range selection.Nodes {
			sort.Slice(node.Attr, func(first_index, second_index int) bool {
				return node.Attr[first_index].Key < node.Attr[second_index].Key
			})
			attributes := node.Attr[:0]
			for _, attribute := range node.Attr {
				key := strings.ToLower(attribute.Key)
				if key == "href" || key == "src" || key == "alt" || key == "title" || key == "width" || key == "height" {
					attributes = append(attributes, attribute)
				}
			}
			node.Attr = attributes
		}
	})
	output, err := root.Html()
	if err != nil {
		return html.EscapeString(html_to_text(fragment))
	}
	return output
}

func best_zhihu_image_source(selection *goquery.Selection) string {
	for _, attribute := range []string{"data-original", "data-actualsrc", "data-default-watermark-src", "src"} {
		value, ok := selection.Attr(attribute)
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" || is_placeholder_image(value) {
			continue
		}
		return value
	}
	return ""
}

func is_placeholder_image(raw_url string) bool {
	lower_url := strings.ToLower(raw_url)
	return strings.Contains(lower_url, "data:image/svg") ||
		strings.Contains(lower_url, "placeholder") ||
		strings.Contains(lower_url, "loading") ||
		strings.Contains(lower_url, "blank")
}

func html_to_text(fragment string) string {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(fragment))
	if err != nil {
		return strings.TrimSpace(fragment)
	}
	return strings.TrimSpace(document.Text())
}

func answer_question_url(page *zhihu.AnswerPage) string {
	if page == nil {
		return ""
	}
	question_id := strings.TrimSpace(page.URL.QuestionID)
	if question_id == "" {
		question_id = strings.TrimSpace(page.Question.ID)
	}
	if question_id == "" {
		return ""
	}
	return "https://www.zhihu.com/question/" + url.PathEscape(question_id)
}

func format_time(unix_time int64) string {
	return time.Unix(unix_time, 0).Format("2006-01-02 15:04")
}

func replace_postprocess_file(file_path string, data []byte) error {
	file_info, err := os.Stat(file_path)
	if err != nil {
		return err
	}
	temporary_file, err := os.CreateTemp(filepath.Dir(file_path), ".zhihu-postprocess-*.tmp")
	if err != nil {
		return err
	}
	temporary_path := temporary_file.Name()
	defer os.Remove(temporary_path)
	if err := temporary_file.Chmod(file_info.Mode().Perm()); err != nil {
		_ = temporary_file.Close()
		return err
	}
	if _, err := temporary_file.Write(data); err != nil {
		_ = temporary_file.Close()
		return err
	}
	if err := temporary_file.Sync(); err != nil {
		_ = temporary_file.Close()
		return err
	}
	if err := temporary_file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary_path, file_path)
}
