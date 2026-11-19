package x

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

func TestArticleStream(t *testing.T) {
	// Query results reduced from the supplied 2026-10-29 article capture.
	source, err := os.ReadFile("testdata/article_stream.html")
	if err != nil {
		t.Fatal(err)
	}
	document, err := html.Parse(strings.NewReader(string(source)))
	if err != nil {
		t.Fatal(err)
	}
	status_id := "2115638135208260002"
	result, err := extract_post(string(source), document, status_id, "https://x.com/giaaibusi/status/"+status_id)
	if err != nil {
		t.Fatal(err)
	}
	result, err = normalize_result(result, status_id, result.SourceURL)
	if err != nil {
		t.Fatal(err)
	}
	if result.Article == nil {
		t.Fatal("article missing from streamed post")
	}
	article := result.Article
	if article.ID != "2115613940881465344" {
		t.Errorf("lost article ID: %q", article.ID)
	}
	if article.Title != "【1,407の衣装を無料配布】AI美女「衣装」データベース爆誕！" {
		t.Errorf("unexpected title: %q", article.Title)
	}
	if !strings.HasSuffix(article.Text, "無検閲AIでの画像生成などプレゼントを受け取れます") {
		t.Errorf("article text is truncated: %q", article.Text)
	}
	article_document, err := goquery.NewDocumentFromReader(strings.NewReader(article.HTML))
	if err != nil {
		t.Fatal(err)
	}
	if title := article_document.Find(`[data-n="article-title"]`).Text(); title != article.Title {
		t.Errorf("HTML missing article title: %q", title)
	}
	for _, selector := range []string{
		`article[data-n="x-article"]`,
		`img[data-n="article-cover"]`,
		`a[href="https://ai-costume-gallery.pages.dev/"]`,
		`a[href="http://x.gd/9BOJA"]`,
		`a[href="https://x.com/i/status/2115431641766359244"]`,
		`h1[data-n="article-heading"]`, `h2[data-n="article-heading"]`, `strong`, `ol > li`,
	} {
		if article_document.Find(selector).Length() == 0 {
			t.Errorf("HTML missing %q", selector)
		}
	}
	if count := article_document.Find(`img[data-n="article-image"]`).Length(); count != 4 {
		t.Errorf("got %d article images, want 4", count)
	}
	for index, expected_url := range []string{
		"https://pbs.twimg.com/media/HVwsOsXbUAAwgAL.png",
		"https://pbs.twimg.com/media/HVwsgP3bwAIXW1V.jpg",
		"https://pbs.twimg.com/media/HVwsnNrbEAAEu_w.jpg",
		"https://pbs.twimg.com/media/HVwszCyaYAA6pAn.jpg",
	} {
		if source_url, _ := article_document.Find(`img[data-n="article-image"]`).Eq(index).Attr("src"); source_url != expected_url {
			t.Errorf("image %d: got %q, want %q", index, source_url, expected_url)
		}
	}
	if count := article_document.Find(`article *:not([data-n])`).Length(); count != 0 {
		t.Errorf("got %d nodes without semantic identifiers", count)
	}
	if article.CoverURL != "https://pbs.twimg.com/media/HVwrs-UbQAANl9w.jpg" {
		t.Errorf("unexpected cover: %q", article.CoverURL)
	}
	if result.AuthorID != "1946341290385563648" || result.AuthorUsername != "giaaibusi" || result.AuthorName != "ギア" {
		t.Errorf("lost author metadata: %+v", result)
	}
	if result.PublishTime == 0 || result.ViewCount == 0 || result.LikeCount == 0 {
		t.Errorf("lost post metadata: %+v", result)
	}
}

func TestOrdinaryPostStream(t *testing.T) {
	for _, source := range []string{
		`$R[0]={__typename:"Tweet",rest_id:"123",details:$R[1]={full_text:"Ordinary post"},media_entities2:$R[2]=[]};`,
		`$R[0] = {kind:"Result.Ok",value:"{\"data\":{\"tweet\":{\"__typename\":\"Tweet\",\"rest_id\":\"123\",\"details\":{\"full_text\":\"Ordinary post\"}}}}"};`,
	} {
		result, ok := tsr_extract_post(source, nil, "123", "https://x.com/user/status/123")
		if !ok || result.BodyText != "Ordinary post" || result.Article != nil {
			t.Fatalf("unexpected ordinary post: %+v", result)
		}
		if _, ok := tsr_extract_post(source, nil, "456", "https://x.com/user/status/456"); ok {
			t.Fatal("returned a different post")
		}
	}
}

func TestArticleLegacyRecords(t *testing.T) {
	source := `$R[0]={__id:"client:tweet:article",article_results:$R[1]={__ref:"results"}};
	$R[2]={__id:"results",result:$R[3]={__ref:"article"}};
	$R[4]={__id:"article",rest_id:"123",title:"Title",plain_text:"Hello 😀 world",content_state:$R[5]={__ref:"state"},cover_media_results:$R[6]={__ref:"cover"},media_entities:$R[7]={__refs:$R[8]=["media"]}};
	$R[9]={__id:"state",blocks:$R[10]={__refs:$R[11]=["block","image"]},entity_map:$R[12]={__refs:$R[13]=["entity"]}};
	$R[14]={__id:"block",text:"Hello 😀 world",type:"header-two",inline_style_ranges:$R[15]={__refs:$R[16]=["style"]}};
	$R[17]={__id:"style",offset:6,length:2,style:"Bold"};
	$R[18]={__id:"image",type:"atomic",entity_ranges:$R[19]={__refs:$R[20]=["range"]}};
	$R[21]={__id:"range",key:0,length:1,offset:0};
	$R[22]={__id:"entity",key:"0",value:$R[23]={__ref:"value"}};
	$R[24]={__id:"value",type:"MEDIA",data:$R[25]={__ref:"data"}};
	$R[26]={__id:"data",media_items:$R[27]={__refs:$R[28]=["item"]}};
	$R[29]={__id:"item",media_id:"photo"};
	$R[30]={__id:"media",media_id:"photo",media_info:$R[31]={__ref:"info"}};
	$R[32]={__id:"info",original_img_url:"https://pbs.twimg.com/media/photo.jpg"};
	$R[33]={__id:"cover",result:$R[34]={__ref:"media"}};`
	article := extract_article(source, "tweet")
	if article == nil {
		t.Fatal("legacy article missing")
	}
	if article.ID != "123" || article.Title != "Title" || article.Text != "Hello 😀 world" || article.CoverURL != "https://pbs.twimg.com/media/photo.jpg" {
		t.Errorf("unexpected legacy article: %+v", article)
	}
	if !strings.Contains(article.HTML, `<h2 data-n="article-heading">Hello <strong data-n="article-bold">😀</strong> world</h2>`) ||
		!strings.Contains(article.HTML, `<img data-n="article-image" src="https://pbs.twimg.com/media/photo.jpg" alt="">`) {
		t.Errorf("unexpected legacy HTML: %q", article.HTML)
	}
}

func TestArticleHTMLEscaping(t *testing.T) {
	var entity map[string]any
	err := json.Unmarshal([]byte(`{
		"title":"<script>alert(1)</script>",
		"content_state":{
			"blocks":[{
				"type":"unstyled","text":"😀<b>&\"\nend",
				"inline_style_ranges":[{"offset":0,"length":5,"style":"Bold"},{"offset":2,"length":5,"style":"Italic"}],
				"entity_ranges":[{"offset":0,"length":2,"key":0},{"offset":2,"length":5,"key":1}]
			}],
			"entity_map":[
				{"key":"0","value":{"type":"LINK","data":{"url":"javascript:alert(1)"}}},
				{"key":"1","value":{"type":"LINK","data":{"url":"https://example.com/?a=1&b=\"quoted\""}}}
			]
		}
	}`), &entity)
	if err != nil {
		t.Fatal(err)
	}
	article := article_from_object(entity)
	if article == nil {
		t.Fatal("article missing")
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(article.HTML))
	if err != nil {
		t.Fatal(err)
	}
	if document.Find(`script,b,[onclick],[onerror],a[href^="javascript:"]`).Length() != 0 {
		t.Fatalf("unsafe article HTML: %s", article.HTML)
	}
	if document.Find(`[data-n="article-title"]`).Text() != "<script>alert(1)</script>" || document.Find("p").Text() != "😀<b>&\"end" {
		t.Errorf("text changed while escaping: %s", article.HTML)
	}
	if document.Find(`br[data-n="article-line-break"]`).Length() != 1 || document.Find("strong").Text() != "😀<b>" {
		t.Errorf("lost line break or UTF-16 style offsets: %s", article.HTML)
	}
	if !strings.Contains(article.HTML, "</a></em></strong>") || document.Find("strong em a").Length() != 1 || document.Find("p > em > a").Length() != 1 {
		t.Errorf("overlapping inline ranges were not nested correctly: %s", article.HTML)
	}
	if href, _ := document.Find("a").First().Attr("href"); href != `https://example.com/?a=1&b="quoted"` {
		t.Errorf("link changed while escaping: %q", href)
	}
}
