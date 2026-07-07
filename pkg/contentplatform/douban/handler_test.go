package douban

import (
	"context"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
	doubanpkg "wx_channel/pkg/scraper/douban"
)

type fakeDoubanFetcher struct {
	id         any
	subjectURL string
	topicURL   string
	profile    *doubanpkg.MediaProfile
	topic      *doubanpkg.GroupTopicProfile
}

func (f *fakeDoubanFetcher) FetchMediaProfile(ctx context.Context, id any) (*doubanpkg.MediaProfile, error) {
	f.id = id
	return f.profile, nil
}

func (f *fakeDoubanFetcher) FetchSubjectProfile(ctx context.Context, rawURL string) (*doubanpkg.MediaProfile, error) {
	f.subjectURL = rawURL
	return f.profile, nil
}

func (f *fakeDoubanFetcher) FetchGroupTopic(ctx context.Context, rawURL string) (*doubanpkg.GroupTopicProfile, error) {
	f.topicURL = rawURL
	return f.topic, nil
}

func TestMatchDoubanSubjectURL(t *testing.T) {
	h := New(&fakeDoubanFetcher{})
	if !h.Match("https://movie.douban.com/subject/1393859/") {
		t.Fatalf("expected douban subject URL to match")
	}
	if !h.Match("https://book.douban.com/subject/1007305/") {
		t.Fatalf("expected douban book subject URL to match")
	}
	if !h.Match("https://www.douban.com/group/topic/490375064/?_spm_id=demo") {
		t.Fatalf("expected douban group topic URL to match")
	}
}

func TestProbeResolveDoubanJSON(t *testing.T) {
	fetcher := &fakeDoubanFetcher{profile: &doubanpkg.MediaProfile{
		Name:        "老友记 第一季",
		Type:        "tv",
		Overview:    "六个好友的故事",
		CoverURL:    "https://img1.doubanio.com/view/photo/s_ratio_poster/public/p2186920269.webp",
		VoteAverage: 9.7,
	}}
	h := New(fetcher)
	rawURL := "https://movie.douban.com/subject/1393859/"
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: rawURL})
	if err != nil {
		t.Fatal(err)
	}
	if probe.ContentID != "1393859" {
		t.Fatalf("unexpected probe id %q", probe.ContentID)
	}
	if fetcher.subjectURL != rawURL {
		t.Fatalf("unexpected subject URL %q", fetcher.subjectURL)
	}
	if title := contentdownload.ContentTitle(probe.Content); title != "老友记 第一季" {
		t.Fatalf("unexpected title: %q", title)
	}
	summary := contentdownload.ContentSummaryOf(probe.Content)
	if summary.Author != "豆瓣" || summary.AuthorNickname != "豆瓣" {
		t.Fatalf("unexpected official author summary %#v", summary)
	}
	if cover := contentdownload.ContentCoverURL(probe.Content); cover != "https://img1.doubanio.com/view/photo/s_ratio_poster/public/p2186920269.webp" {
		t.Fatalf("unexpected cover: %q", cover)
	}
	metadata := contentdownload.ContentMetadataOf(probe.Content)
	if metadata["account_external_id"] != "douban" || metadata["author_homepage_url"] != "https://www.douban.com/" {
		t.Fatalf("unexpected official metadata %#v", metadata)
	}
	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{URL: rawURL, Probe: probe})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Download.Protocol != "inline_json" || resolved.Suffix != ".json" {
		t.Fatalf("unexpected resolved download: %+v suffix=%q", resolved.Download, resolved.Suffix)
	}
}

func TestProbeDoubanGroupTopicUsesPublisherAuthor(t *testing.T) {
	fetcher := &fakeDoubanFetcher{topic: &doubanpkg.GroupTopicProfile{
		ID:              "490375064",
		GroupID:         "22692",
		GroupName:       "上班这件事",
		Title:           "分享下这几年身边的两个切实通过自己努力改变命运的例子",
		BodyText:        "正文",
		AuthorID:        "54021805",
		AuthorName:      "假的积木花",
		AuthorURL:       "https://www.douban.com/people/54021805/",
		AuthorAvatarURL: "https://img3.doubanio.com/icon/up54021805-3.jpg",
		CreatedAt:       "2026-06-10 15:43:57",
	}}
	h := New(fetcher)
	rawURL := "https://www.douban.com/group/topic/490375064/?_spm_id=demo"
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: rawURL})
	if err != nil {
		t.Fatal(err)
	}
	if fetcher.topicURL != rawURL {
		t.Fatalf("unexpected topic URL %q", fetcher.topicURL)
	}
	summary := contentdownload.ContentSummaryOf(probe.Content)
	if summary.Type != "topic" || summary.Author != "假的积木花" || summary.AuthorAvatarURL == "" {
		t.Fatalf("unexpected topic summary %#v", summary)
	}
	metadata := contentdownload.ContentMetadataOf(probe.Content)
	if metadata["account_external_id"] != "54021805" ||
		metadata["author_homepage_url"] != "https://www.douban.com/people/54021805/" ||
		metadata["group_id"] != "22692" {
		t.Fatalf("unexpected topic metadata %#v", metadata)
	}
}
