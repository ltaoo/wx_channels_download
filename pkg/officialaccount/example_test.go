package officialaccount

import (
	"fmt"
	"testing"
)

func TestFetchOfficialAccountArticle(t *testing.T) {
	url := "https://mp.weixin.qq.com/s/nGoM1bVQ6zjv2i-zLHLyaw"

	oa := &OfficialAccountDownload{}
	article, err := oa.FetchArticle(url)
	if err != nil {
		t.Fatalf("FetchArticle failed: %v", err)
	}

	fmt.Printf("Title: %s\n", article.Title)
	fmt.Printf("Type: %d\n", article.Type)
	fmt.Printf("Creator: %s\n", article.AuthorNickname)
	fmt.Printf("Content length: %d\n", len(article.Content))
	fmt.Printf("Images count: %d\n", len(article.Images))
	fmt.Printf("Videos count: %d\n", len(article.Videos))

	if article.Title == "" {
		t.Error("article title is empty")
	}
	if article.Content == "" {
		t.Error("article content is empty")
	}
}

func TestBuildHTMLFromURL(t *testing.T) {
	url := "https://mp.weixin.qq.com/s/nGoM1bVQ6zjv2i-zLHLyaw"

	oa := &OfficialAccountDownload{}
	html, err := oa.BuildHTMLFromURL(url, true)
	if err != nil {
		t.Fatalf("BuildHTMLFromURL failed: %v", err)
	}

	fmt.Printf("HTML length: %d\n", len(html))

	if len(html) == 0 {
		t.Error("generated HTML is empty")
	}
}

func TestSaveURLAsMarkdown(t *testing.T) {
	url := "https://mp.weixin.qq.com/s/nGoM1bVQ6zjv2i-zLHLyaw"

	oa := &OfficialAccountDownload{}
	err := oa.SaveURLAsMarkdown(url, t.TempDir())
	if err != nil {
		t.Fatalf("SaveURLAsMarkdown failed: %v", err)
	}
}


func TestExtractArticleID(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "short path",
			url:  "https://mp.weixin.qq.com/s/SXyNocq1-K4WkFcI-0aD6w",
			want: "SXyNocq1-K4WkFcI-0aD6w",
		},
		{
			name: "biz sn query",
			url:  "https://mp.weixin.qq.com/s?__biz=xz&sn=abc",
			want: "xz_abc",
		},
		{
			name: "officialaccount scheme",
			url:  "officialaccount://https://mp.weixin.qq.com/s/SXyNocq1-K4WkFcI-0aD6w",
			want: "SXyNocq1-K4WkFcI-0aD6w",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractArticleID(tt.url); got != tt.want {
				t.Fatalf("ExtractArticleID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFlexibleIntUnmarshalString(t *testing.T) {
	var data CgiDataNew
	if err := json.Unmarshal([]byte(`{"user_uin":"69477998648217","mid":"123","idx":"1"}`), &data); err != nil {
		t.Fatal(err)
	}
	if data.UserUin != FlexibleInt(69477998648217) || data.Mid != 123 || data.Idx != 1 {
		t.Fatalf("unexpected flexible ints: user_uin=%d mid=%d idx=%d", data.UserUin, data.Mid, data.Idx)
	}
}

func TestNewWechatOfficialArticleUsesAvatarFallback(t *testing.T) {
	article := newWechatOfficialArticle(&CgiDataNew{OriHeadImgUrl: "https://example.com/ori.jpg"}, "")
	if article.AuthorAvatar != "https://example.com/ori.jpg" {
		t.Fatalf("AuthorAvatar = %q", article.AuthorAvatar)
	}
}

func TestPicturePageInfoListFromGlobalScript(t *testing.T) {
	data, err := parse_cgi_datanew(`
		<html><body>
		<script>
		window.cgiDataNew = {
			title: "live photo article",
			content_noencode: "",
			page_type: 2,
			author: "creator",
			nick_name: "nickname",
			round_head_img: "https://example.com/avatar.jpg",
			user_name: "gh_demo"
		};
		</script>
		<script>
		var picture_page_info_list = [{
			cdn_url: "https://example.com/only.jpg",
			live_photo: {
				format_info: [{ url: "https://example.com/only.mp4" }]
			}
		}];
		</script>
		</body></html>
	`)
	if err != nil {
		t.Fatalf("parse_cgi_datanew: %v", err)
	}
	if len(data.PicturePageInfoList) != 1 {
		t.Fatalf("picture_page_info_list len = %d, want 1", len(data.PicturePageInfoList))
	}
	if got := data.PicturePageInfoList[0].CdnUrl; got != "https://example.com/only.jpg" {
		t.Fatalf("cdn_url = %q", got)
	}
	if got := len(data.PicturePageInfoList[0].LivePhoto.FormatInfo); got != 1 {
		t.Fatalf("live_photo.format_info len = %d, want 1", got)
	}

	article := newWechatOfficialArticle(data, "")
	if len(article.Images) != 1 || article.Images[0] != "https://example.com/only.jpg" {
		t.Fatalf("article images = %#v", article.Images)
	}
	if len(article.PicturePageInfoList) != 1 {
		t.Fatalf("article picture_page_info_list len = %d, want 1", len(article.PicturePageInfoList))
	}

	html, err := (&OfficialAccountDownload{}).BuildHTMLFromArticle(article, false)
	if err != nil {
		t.Fatalf("BuildHTMLFromArticle: %v", err)
	}
	if !strings.Contains(html, `<video src="https://example.com/only.mp4" poster="https://example.com/only.jpg"`) {
		t.Fatalf("live photo video missing from html: %s", html)
	}
}
