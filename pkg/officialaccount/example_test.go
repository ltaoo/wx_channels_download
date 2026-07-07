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
