package xiaohongshu

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
	xhspkg "wx_channel/pkg/scraper/xiaohongshu"
)

type fakeNoteFetcher struct {
	page *xhspkg.NotePage
}

func (f fakeNoteFetcher) FetchNotePage(ctx context.Context, rawURL string) (*xhspkg.NotePage, error) {
	return f.page, nil
}

func TestResolveVideo(t *testing.T) {
	h := New(fakeNoteFetcher{page: fakeVideoPage()})
	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{URL: "https://www.xiaohongshu.com/explore/note1"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Platform != PlatformID {
		t.Fatalf("platform = %s", resolved.Platform)
	}
	if resolved.Download.Protocol != "http" || resolved.Download.URL != "https://example.com/video.mp4" {
		t.Fatalf("download = %#v", resolved.Download)
	}
	if resolved.Suffix != ".mp4" {
		t.Fatalf("suffix = %s", resolved.Suffix)
	}
	if strings.Contains(resolved.Filename, "/") {
		t.Fatalf("filename was not sanitized: %q", resolved.Filename)
	}
	if resolved.Content == nil || contentdownload.ContentType(resolved.Content) != ContentTypeVideo {
		t.Fatalf("content = %#v", resolved.Content)
	}
	if resolved.Pipeline == nil || len(resolved.Pipeline.Nodes) == 0 {
		t.Fatal("expected pipeline plan")
	}
}

func TestResolveInitialStateJSON(t *testing.T) {
	h := New(fakeNoteFetcher{page: fakeVideoPage()})
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.xiaohongshu.com/explore/note1"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	found := false
	for _, variant := range probe.Variants {
		if variant.ID == "initial_state_json" && variant.Type == "json" && variant.Suffix == ".json" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing initial_state_json variant: %#v", probe.Variants)
	}
	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{
		URL:     "https://www.xiaohongshu.com/explore/note1",
		Probe:   probe,
		Options: contentdownload.Options{VariantID: "initial_state_json"},
	})
	if err != nil {
		t.Fatalf("Resolve json: %v", err)
	}
	if resolved.Download.Protocol != "inline_json" || resolved.Suffix != ".json" {
		t.Fatalf("json resolved = %#v", resolved)
	}
	raw, ok := resolved.Metadata["json"].(json.RawMessage)
	if !ok || !json.Valid(raw) {
		t.Fatalf("json metadata = %#v", resolved.Metadata["json"])
	}
}

func TestResolvePicturesZip(t *testing.T) {
	h := New(fakeNoteFetcher{page: fakeImagePage()})
	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{URL: "https://www.xiaohongshu.com/explore/note2"})
	if err != nil {
		t.Fatalf("Resolve pictures: %v", err)
	}
	if resolved.Download.Protocol != "zip" || resolved.Suffix != ".zip" {
		t.Fatalf("zip resolved = %#v", resolved)
	}
	u, err := url.Parse(resolved.Download.URL)
	if err != nil {
		t.Fatal(err)
	}
	filesJSON := u.Query().Get("files")
	var files []contentdownload.ZipFileItem
	if err := json.Unmarshal([]byte(filesJSON), &files); err != nil {
		t.Fatalf("files json: %v", err)
	}
	if len(files) != 2 || files[0].URL != "https://example.com/1.webp" || !strings.HasSuffix(files[1].Filename, ".jpg") {
		t.Fatalf("files = %#v", files)
	}
	if contentdownload.ContentType(resolved.Content) != ContentTypeImageAlbum {
		t.Fatalf("content type = %s", contentdownload.ContentType(resolved.Content))
	}
}

func fakeVideoPage() *xhspkg.NotePage {
	note := xhspkg.Note{
		NoteID:    "note1",
		Title:     "165/110 demo",
		Desc:      "desc",
		Type:      "video",
		Time:      1683296413000,
		XSecToken: "note-token",
		User: xhspkg.User{
			UserID:    "user1",
			Nickname:  "author",
			Avatar:    "https://example.com/avatar.jpg",
			XSecToken: "user-token",
		},
		ImageList: []xhspkg.Image{{
			URLDefault: "https://example.com/cover.webp",
			Width:      720,
			Height:     1280,
		}},
	}
	note.Video.Media.Stream.H264 = []xhspkg.VideoStreamInfo{{
		MasterURL:    "https://example.com/video.mp4",
		Format:       "mp4",
		Width:        720,
		Height:       1280,
		Size:         123,
		StreamType:   259,
		QualityType:  "HD",
		AvgBitrate:   800000,
		VideoBitrate: 760000,
	}}
	note.Video.Capa.Duration = 14
	return &xhspkg.NotePage{
		URL:              xhspkg.NoteURL{NoteID: "note1", Canonical: "https://www.xiaohongshu.com/explore/note1"},
		Source:           "https://www.xiaohongshu.com/explore/note1",
		PageHTML:         "<html>page</html>",
		InitialStateJSON: json.RawMessage(`{"note":{"firstNoteId":"note1"}}`),
		Note:             note,
	}
}

func fakeImagePage() *xhspkg.NotePage {
	note := xhspkg.Note{
		NoteID: "note2",
		Title:  "image note",
		Type:   "normal",
		User:   xhspkg.User{UserID: "user2", Nickname: "author2"},
		ImageList: []xhspkg.Image{
			{URLDefault: "https://example.com/1.webp"},
			{URLDefault: "https://example.com/2.jpg"},
		},
	}
	return &xhspkg.NotePage{
		URL:              xhspkg.NoteURL{NoteID: "note2", Canonical: "https://www.xiaohongshu.com/explore/note2"},
		Source:           "https://www.xiaohongshu.com/explore/note2",
		InitialStateJSON: json.RawMessage(`{"note":{"firstNoteId":"note2"}}`),
		Note:             note,
	}
}
