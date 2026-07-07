package douyin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const exampleSecUID = "MS4wLjABAAAAOE57npukdzs0SH_Fk5RHB-qfUlnZ5jQT2R_KPH4Sd8s"

func TestParseProfileURL(t *testing.T) {
	tests := map[string]string{
		"https://www.douyin.com/user/" + exampleSecUID:                   exampleSecUID,
		"https://www.iesdouyin.com/share/user/" + exampleSecUID + "?x=1": exampleSecUID,
		"https://www.douyin.com/?sec_user_id=" + exampleSecUID:           exampleSecUID,
		exampleSecUID: exampleSecUID,
	}
	for rawURL, want := range tests {
		got, ok := ParseProfileURL(rawURL)
		if !ok {
			t.Fatalf("ParseProfileURL(%q) returned false", rawURL)
		}
		if got.SecUserID != want {
			t.Fatalf("ParseProfileURL(%q).SecUserID = %q, want %q", rawURL, got.SecUserID, want)
		}
		if got.Canonical != CanonicalProfileURL(want) {
			t.Fatalf("canonical = %q", got.Canonical)
		}
	}
}

func TestParseProfilePageHTMLFromExample(t *testing.T) {
	body := readExampleFile(t, "user.html")
	page, err := ParseProfilePageHTML("https://www.douyin.com/user/"+exampleSecUID, string(body))
	if err != nil {
		t.Fatalf("ParseProfilePageHTML() error = %v", err)
	}
	if page.User.SecUID != exampleSecUID {
		t.Fatalf("sec uid = %q", page.User.SecUID)
	}
	if page.User.Nickname != "一菲（南哥助理）" {
		t.Fatalf("nickname = %q", page.User.Nickname)
	}
	if page.User.AwemeCount != 53 {
		t.Fatalf("aweme count = %d", page.User.AwemeCount)
	}
	if page.User.FollowerCount == 0 || page.User.AvatarURL == "" {
		t.Fatalf("incomplete profile: %+v", page.User)
	}
}

func TestParseAwemePostResponseFromExample(t *testing.T) {
	body := extractMarkdownJSON(t, string(readExampleFile(t, "post.md")))
	response, err := ParseAwemePostResponse([]byte(body))
	if err != nil {
		t.Fatalf("ParseAwemePostResponse() error = %v", err)
	}
	if len(response.AwemeList) == 0 {
		t.Fatal("expected aweme list")
	}
	first := SummarizeAweme(response.AwemeList[0])
	if first.ID != "7647936565273451227" {
		t.Fatalf("first id = %q", first.ID)
	}
	if first.Author.Nickname != "一菲（南哥助理）" || first.VideoURL == "" || first.CoverURL == "" {
		t.Fatalf("incomplete first summary: %+v", first)
	}
	var imageAlbum AwemeSummary
	for _, post := range SummarizeAwemes(response.AwemeList) {
		if post.ContentType == "image_album" {
			imageAlbum = post
			break
		}
	}
	if imageAlbum.ID == "" || len(imageAlbum.ImageURLs) == 0 {
		t.Fatalf("expected image album summary")
	}
}

func TestFetchUserProfile(t *testing.T) {
	secUID := "MS4wLjABAAAAUnitTestSecUserID1234567890"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user/" + secUID:
			if r.Header.Get("User-Agent") == "" {
				t.Fatal("missing profile user-agent")
			}
			http.SetCookie(w, &http.Cookie{Name: "UIFID", Value: "uifid_from_page"})
			_, _ = w.Write([]byte(testProfileHTML(secUID)))
		case defaultAwemePostPath:
			if r.URL.Query().Get("sec_user_id") != secUID {
				t.Fatalf("sec_user_id query = %q", r.URL.Query().Get("sec_user_id"))
			}
			if r.Header.Get("Referer") == "" || r.Header.Get("Cookie") == "" {
				t.Fatalf("missing post headers: referer=%q cookie=%q", r.Header.Get("Referer"), r.Header.Get("Cookie"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status_code":0,"max_cursor":123,"has_more":1,"aweme_list":[{"aweme_id":"1","desc":"hello","create_time":1781799676,"author":{"uid":"uid1","sec_uid":"` + secUID + `","nickname":"作者","avatar_thumb":{"url_list":["https://example.com/avatar.jpg"]}},"video":{"play_addr":{"url_list":["https://example.com/playwm.mp4"]},"cover":{"url_list":["https://example.com/cover.jpg"]}},"statistics":{"digg_count":7}}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient()
	client.HTTPClient = server.Client()
	client.BaseURL = server.URL
	client.Cookie = "sessionid=test"

	page, err := client.FetchUserProfile(context.Background(), "https://www.douyin.com/user/"+secUID, ProfileOptions{Count: 1})
	if err != nil {
		t.Fatalf("FetchUserProfile() error = %v", err)
	}
	if page.User.Nickname != "作者" || len(page.Posts) != 1 {
		t.Fatalf("unexpected page: user=%+v posts=%d warnings=%v", page.User, len(page.Posts), page.Warnings)
	}
	if page.Posts[0].VideoURL != "https://example.com/playwm.mp4" || page.Posts[0].DiggCount != 7 {
		t.Fatalf("unexpected post summary: %+v", page.Posts[0])
	}
	if !page.HasMore || page.MaxCursor != 123 {
		t.Fatalf("pagination = has_more:%v max:%d", page.HasMore, page.MaxCursor)
	}
}

func testProfileHTML(secUID string) string {
	payload := fmt.Sprintf(`7:["$","$L9",null,{"user":{"statusCode":0,"user":{"uid":"uid1","secUid":%q,"shortId":"0","nickname":"作者","desc":"签名","avatarUrl":"https://example.com/avatar.jpg","followerCount":3,"followingCount":2,"awemeCount":1,"shareInfo":{"shareUrl":"www.iesdouyin.com/share/user/%s"}}},"statusCode":0}]`+"\n", secUID, secUID)
	return `<html><body><script>self.__pace_f.push([1, ` + strconv.Quote(payload) + `])</script></body></html>`
}

func readExampleFile(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "..", "scraper_examples", "douyinweb", "260619", name)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return body
}

func extractMarkdownJSON(t *testing.T, markdown string) string {
	t.Helper()
	idx := strings.Index(markdown, "```json")
	if idx < 0 {
		t.Fatal("missing json fence")
	}
	body := markdown[idx+len("```json"):]
	start := strings.Index(body, "{")
	end := strings.LastIndex(body, "```")
	if start < 0 || end < 0 || end <= start {
		t.Fatal("invalid json fence")
	}
	return body[start:end]
}
