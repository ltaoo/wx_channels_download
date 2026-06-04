package zhihu

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
	zhihupkg "wx_channel/pkg/zhihu"
)

type fakePageFetcher struct{}

func (fakePageFetcher) FetchAnswerPage(rawURL string) (*zhihupkg.AnswerPage, error) {
	return &zhihupkg.AnswerPage{
		URL: zhihupkg.AnswerURL{
			QuestionID: "1",
			AnswerID:   "2",
			Canonical:  "https://www.zhihu.com/question/1/answer/2",
		},
		Question: zhihupkg.Question{Title: "question", Detail: "<p>question body</p>"},
		Answer: zhihupkg.Answer{
			ID:      "2",
			Content: "<p>answer body</p>",
			Excerpt: "excerpt",
			Author:  zhihupkg.User{Name: "author", AvatarURL: "https://example.com/answer-avatar.jpg"},
		},
	}, nil
}

func (fakePageFetcher) FetchQuestionPage(rawURL string) (*zhihupkg.QuestionPage, error) {
	return &zhihupkg.QuestionPage{
		URL: zhihupkg.QuestionURL{
			QuestionID: "1",
			Canonical:  "https://www.zhihu.com/question/1",
		},
		Question: zhihupkg.Question{
			ID:      "1",
			Title:   "question",
			Detail:  "<p>question body</p>",
			Excerpt: "question excerpt",
			Author:  zhihupkg.User{Name: "question author", AvatarURL: "https://example.com/question-avatar.jpg"},
		},
	}, nil
}

func (fakePageFetcher) FetchArticlePage(rawURL string) (*zhihupkg.ArticlePage, error) {
	return &zhihupkg.ArticlePage{
		URL: zhihupkg.ArticleURL{
			ArticleID: "680224567",
			Canonical: "https://zhuanlan.zhihu.com/p/680224567",
		},
		Article: zhihupkg.Article{
			ID:       "680224567",
			Title:    "article",
			Content:  "<p>article body</p>",
			Excerpt:  "article excerpt",
			ImageURL: "https://example.com/cover.jpg",
			Author:   zhihupkg.User{Name: "article author", AvatarURL: "https://example.com/article-avatar.jpg"},
		},
	}, nil
}

func TestResolve(t *testing.T) {
	h := New(fakePageFetcher{})
	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{URL: "https://www.zhihu.com/question/1/answer/2"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Platform != PlatformID {
		t.Fatalf("platform = %s", resolved.Platform)
	}
	if resolved.Download.Protocol != "zhihu" {
		t.Fatalf("protocol = %s", resolved.Download.Protocol)
	}
	if resolved.Pipeline == nil || len(resolved.Pipeline.Nodes) == 0 {
		t.Fatal("expected pipeline plan")
	}
	if resolved.Content == nil || contentdownload.ContentType(resolved.Content) != "answer" {
		t.Fatalf("content type = %#v, want answer", resolved.Content)
	}
}

func TestExecutorLocalizesImagesAndVideosFromMetadataPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/image.png":
			w.Header().Set("content-type", "image/png")
			_, _ = w.Write([]byte{0x89, 0x50, 0x4e, 0x47})
		case "/video.mp4":
			w.Header().Set("content-type", "video/mp4")
			_, _ = w.Write([]byte("video-body"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	page := &zhihupkg.AnswerPage{
		URL: zhihupkg.AnswerURL{
			QuestionID: "1",
			AnswerID:   "2",
			Canonical:  "https://www.zhihu.com/question/1/answer/2",
		},
		Source:   server.URL + "/answer",
		Question: zhihupkg.Question{Title: "question"},
		Answer: zhihupkg.Answer{
			ID:      "2",
			Content: `<p><img src="/image.png"></p><video src="/video.mp4"></video>`,
			Author:  zhihupkg.User{Name: "author"},
		},
	}
	destPath := filepath.Join(t.TempDir(), "answer.html")
	executor := NewExecutor(&zhihupkg.Client{HTTPClient: server.Client()})
	err := executor.Execute(context.Background(), contentdownload.ExecuteRequest{
		Resolved: &contentdownload.ResolvedRequest{
			Metadata: map[string]any{"page": page},
		},
		Source:   contentdownload.DownloadSpec{URL: "zhihu://" + server.URL + "/answer", Protocol: "zhihu"},
		DestPath: destPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	out := string(html)
	if !strings.Contains(out, "data:image/png;base64,") {
		t.Fatalf("image was not inlined: %s", out)
	}
	if !strings.Contains(out, `src="answer_files/video_01.mp4"`) || !strings.Contains(out, `controls="controls"`) {
		t.Fatalf("video was not localized as playable media: %s", out)
	}
	video, err := os.ReadFile(filepath.Join(filepath.Dir(destPath), "answer_files", "video_01.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	if string(video) != "video-body" {
		t.Fatalf("video body = %q", video)
	}
}

func TestProbeAnswerReturnsBodyOutput(t *testing.T) {
	h := New(fakePageFetcher{})
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.zhihu.com/question/1/answer/2"})
	if err != nil {
		t.Fatalf("Probe answer: %v", err)
	}
	if probe.Content == nil {
		t.Fatal("expected probe content")
	}
	data, ok := contentdownload.ContentDataOf(probe.Content).(AnswerContent)
	if !ok {
		t.Fatalf("content data = %#v, want AnswerContent", contentdownload.ContentDataOf(probe.Content))
	}
	if data.Answer.Content != "<p>answer body</p>" || data.Question.Detail != "<p>question body</p>" {
		t.Fatalf("answer content = %#v", data)
	}
	if contentdownload.ContentAuthor(probe.Content) != "author" ||
		contentdownload.ContentAuthorNickname(probe.Content) != "author" ||
		contentdownload.ContentAuthorAvatarURL(probe.Content) != "https://example.com/answer-avatar.jpg" ||
		contentdownload.ContentDescription(probe.Content) != "excerpt" {
		t.Fatalf("answer content summary = %#v", contentdownload.ContentSummaryOf(probe.Content))
	}
	output := contentdownload.ContentOutputOf(probe.Content)
	if output["content_type"] != "answer" {
		t.Fatalf("content_type = %#v", output["content_type"])
	}
	if output["body_html"] != "<p>answer body</p>" {
		t.Fatalf("body_html = %#v", output["body_html"])
	}
	if output["question_html"] != "<p>question body</p>" {
		t.Fatalf("question_html = %#v", output["question_html"])
	}
}

func TestProbeQuestionAndArticleContentFields(t *testing.T) {
	h := New(fakePageFetcher{})

	question, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.zhihu.com/question/1"})
	if err != nil {
		t.Fatalf("Probe question: %v", err)
	}
	if question.Content == nil ||
		contentdownload.ContentType(question.Content) != "question" ||
		contentdownload.ContentAuthorNickname(question.Content) != "question author" ||
		contentdownload.ContentAuthorAvatarURL(question.Content) != "https://example.com/question-avatar.jpg" {
		t.Fatalf("question probe = %#v", question)
	}
	if output := contentdownload.ContentOutputOf(question.Content); output["body_html"] != "<p>question body</p>" {
		t.Fatalf("question body_html = %#v", output["body_html"])
	}

	article, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://zhuanlan.zhihu.com/p/680224567"})
	if err != nil {
		t.Fatalf("Probe article: %v", err)
	}
	if article.Content == nil ||
		contentdownload.ContentType(article.Content) != "article" ||
		contentdownload.ContentAuthorNickname(article.Content) != "article author" ||
		contentdownload.ContentAuthorAvatarURL(article.Content) != "https://example.com/article-avatar.jpg" {
		t.Fatalf("article probe = %#v", article)
	}
	if contentdownload.ContentCoverURL(article.Content) != "https://example.com/cover.jpg" {
		t.Fatalf("article cover = %q", contentdownload.ContentCoverURL(article.Content))
	}
	if output := contentdownload.ContentOutputOf(article.Content); output["body_html"] != "<p>article body</p>" {
		t.Fatalf("article body_html = %#v", output["body_html"])
	}
}

func TestResolvePreservesProbeSummaryAndContentIDs(t *testing.T) {
	h := New(fakePageFetcher{})

	answer, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.zhihu.com/question/1/answer/2"})
	if err != nil {
		t.Fatalf("Probe answer: %v", err)
	}
	answerResolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{URL: answer.SourceURL, Probe: answer})
	if err != nil {
		t.Fatalf("Resolve answer: %v", err)
	}
	if answerResolved.Labels["question_id"] != "1" || answerResolved.Labels["answer_id"] != "2" {
		t.Fatalf("answer labels = %#v", answerResolved.Labels)
	}
	answerSummary := contentdownload.ContentSummaryOf(answerResolved.Content)
	if answerSummary.Description != "excerpt" ||
		answerSummary.Author != "author" ||
		answerSummary.CoverURL != "https://example.com/answer-avatar.jpg" {
		t.Fatalf("answer resolved content summary = %#v", answerSummary)
	}

	article, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://zhuanlan.zhihu.com/p/680224567"})
	if err != nil {
		t.Fatalf("Probe article: %v", err)
	}
	articleResolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{URL: article.SourceURL, Probe: article})
	if err != nil {
		t.Fatalf("Resolve article: %v", err)
	}
	if articleResolved.Labels["article_id"] != "680224567" || articleResolved.Metadata["article_id"] != "680224567" {
		t.Fatalf("article labels=%#v metadata=%#v", articleResolved.Labels, articleResolved.Metadata)
	}
	articleSummary := contentdownload.ContentSummaryOf(articleResolved.Content)
	if articleSummary.Description != "article excerpt" ||
		articleSummary.CoverURL != "https://example.com/cover.jpg" ||
		articleSummary.URL != "https://zhuanlan.zhihu.com/p/680224567" {
		t.Fatalf("article resolved content summary = %#v", articleSummary)
	}
}
