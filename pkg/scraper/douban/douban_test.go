package douban

import (
	"context"
	"os"
	"testing"
)

func TestSplitNameAndOriginalName(t *testing.T) {
	tests := []struct {
		name         string
		wantName     string
		wantOriginal string
	}{
		{"周游记 第二季", "周游记 第二季", ""},
		{"姜食堂 第二季 강식당  시즌2", "姜食堂 第二季", "강식당  시즌2"},
		{"地心历险记2：神秘岛 Journey 2: The Mysterious Island", "地心历险记2：神秘岛", "Journey 2: The Mysterious Island"},
		{"最游记 RELOAD 最遊記RELOAD", "最游记", "RELOAD 最遊記RELOAD"},
		{"老友记 第六季 Friends Season 6", "老友记 第六季", "Friends Season 6"},
	}
	for _, tt := range tests {
		got := SplitNameAndOriginalName(tt.name)
		if got.Name != tt.wantName || got.OriginalName != tt.wantOriginal {
			t.Fatalf("SplitNameAndOriginalName(%q) = %#v", tt.name, got)
		}
	}
}

func TestCleanName(t *testing.T) {
	got := CleanName("老友记 第六季 Friends Season 6\u200e (1999)")
	if got != "老友记 第六季 Friends Season 6 (1999)" {
		t.Fatalf("unexpected clean name %q", got)
	}
}

func TestParseSearchHTMLPlainData(t *testing.T) {
	html := `<script>window.__DATA__ = {"items":[{"id":123,"title":"老友记 第六季 Friends Season 6‎ (1999)","abstract":"1999 / 喜剧 / 美国","cover_url":"cover.jpg","labels":[{"text":"剧集"}],"rating":{"value":9.8}}]};</script>`
	got, err := ParseSearchHTML(html)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.List) != 1 {
		t.Fatalf("expected 1 item, got %d", len(got.List))
	}
	item := got.List[0]
	if item.ID != "123" || item.Name != "老友记 第六季" || item.OriginalName != "Friends Season 6" || item.Type != "tv" {
		t.Fatalf("unexpected item %#v", item)
	}
	if len(item.Genres) != 1 || item.Genres[0].Label != "喜剧" {
		t.Fatalf("unexpected genres %#v", item.Genres)
	}
}

func TestParseProfilePageHTML(t *testing.T) {
	html := `<html><body>
		<span property="v:itemreviewed">老友记 第六季 Friends Season 6</span>
		<img src="https://img1.doubanio.com/view/photo/s_ratio_poster/public/p2186920269.webp" rel="v:image" />
		<div id="info">
			<span>导演</span>: <a href="https://www.douban.com/personage/1/">导演A</a><br/>
			<span>主演</span>: <a href="https://www.douban.com/personage/2/">演员B</a><br/>
			<span>类型:</span> <span property="v:genre">喜剧</span><br/>
			<span>制片国家/地区:</span> 美国<br/>
			<span>首播:</span> 1999-09-23<br/>
			<span>集数:</span> 25<br/>
			<span>IMDb:</span> tt0108778
		</div>
		<span property="v:summary">简介<br/>内容</span>
		<strong class="ll rating_num" property="v:average">9.8</strong>
	</body></html>`
	got, err := ParseProfilePageHTML(html)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "老友记 第六季" || got.OriginalName != "Friends Season 6" || got.SourceCount != 25 || got.IMDB != "tt0108778" {
		t.Fatalf("unexpected profile %#v", got)
	}
	if got.CoverURL != "https://img1.doubanio.com/view/photo/s_ratio_poster/public/p2186920269.webp" || got.PosterPath != got.CoverURL {
		t.Fatalf("unexpected profile cover %#v", got)
	}
	if len(got.Actors) != 1 || got.Actors[0].Name != "演员B" {
		t.Fatalf("unexpected actors %#v", got.Actors)
	}
}

func TestParseBookProfilePageHTML(t *testing.T) {
	html := `<html><body>
		<div id="db-nav-book"></div>
		<h1 class="title"><span property="v:itemreviewed">红楼梦</span></h1>
		<div class="subject clearfix">
			<div id="mainpic">
				<img src="https://img1.doubanio.com/view/subject/s/public/s1070959.jpg" rel="v:photo" alt="红楼梦" />
			</div>
			<div id="info">
				<span><span class="pl"> 作者</span>: <a href="/author/4508611">曹雪芹</a></span><br/>
				<span class="pl">出版社:</span> 人民文学出版社<br/>
				<span class="pl">出版年:</span> 1996-12<br/>
				<span class="pl">ISBN:</span> 9787020002207<br/>
			</div>
		</div>
		<strong class="ll rating_num" property="v:average">9.7</strong>
		<div class="intro"><p>《红楼梦》是一部长篇小说。</p></div>
	</body></html>`
	got, err := ParseProfilePageHTML(html)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != string(MediaTypeBook) || got.Name != "红楼梦" {
		t.Fatalf("unexpected book profile %#v", got)
	}
	if got.AirDate != "1996-12" {
		t.Fatalf("unexpected book publish date %q", got.AirDate)
	}
	if got.CoverURL != "https://img1.doubanio.com/view/subject/s/public/s1070959.jpg" {
		t.Fatalf("unexpected book cover %q", got.CoverURL)
	}
	if got.Overview != "《红楼梦》是一部长篇小说。" {
		t.Fatalf("unexpected book overview %q", got.Overview)
	}
}

func TestParseGroupTopicHTML(t *testing.T) {
	html := `<html><body>
		<script>
			window._CONFIG.group = {"id":"22692","title":"上班这件事","name":"上班这件事"};
			window._CONFIG.topic = {"id":"490375064","title":"分享下这几年身边的两个切实通过自己努力改变命运的例子"};
		</script>
		<h1>分享下这几年身边的两个切实通过自己努力改变命运的例子</h1>
		<div class="topic-content clearfix" id="topic-content">
			<div class="user-face">
				<a href="https://www.douban.com/people/54021805/"><img class="pil" src="https://img3.doubanio.com/icon/up54021805-3.jpg" alt="假的积木花"/></a>
			</div>
			<div class="topic-doc">
				<h3>
					<span class="from"><a href="https://www.douban.com/people/54021805/">假的积木花</a></span>
					<div class="topic-meta">
						<span class="create-time">2026-06-10 15:43:57</span>
						<span class="update-time">已编辑</span>
					</div>
				</h3>
				<div id="link-report">
					<div class="topic-content">
						<div class="rich-content topic-richtext"><p>正文第一段</p><p>正文第二段</p></div>
					</div>
				</div>
			</div>
		</div>
	</body></html>`
	got, err := ParseGroupTopicHTML(html, "https://www.douban.com/group/topic/490375064/?_spm_id=demo")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "490375064" || got.GroupID != "22692" || got.GroupName != "上班这件事" {
		t.Fatalf("unexpected topic identity %#v", got)
	}
	if got.AuthorID != "54021805" || got.AuthorName != "假的积木花" {
		t.Fatalf("unexpected topic author %#v", got)
	}
	if got.AuthorAvatarURL != "https://img3.doubanio.com/icon/up54021805-3.jpg" {
		t.Fatalf("unexpected topic author avatar %q", got.AuthorAvatarURL)
	}
	if got.BodyText != "正文第一段正文第二段" {
		t.Fatalf("unexpected topic body text %q", got.BodyText)
	}
}

func TestLiveFetchMediaProfile1393859(t *testing.T) {
	if os.Getenv("DOUBAN_LIVE") != "1" {
		t.Skip("set DOUBAN_LIVE=1 to run live douban fetch")
	}
	client := NewClient()
	profile, err := client.FetchMediaProfile(context.Background(), "1393859")
	if err != nil {
		t.Fatal(err)
	}
	if profile.ID != "1393859" || profile.Name != "老友记 第一季" || profile.OriginalName != "Friends Season 1" {
		t.Fatalf("unexpected live profile %#v", profile)
	}
	if profile.Overview == "" || profile.SourceCount == 0 {
		t.Fatalf("live profile missing parsed content %#v", profile)
	}
}
