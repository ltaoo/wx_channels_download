package youku

import "testing"

func TestParseURL(t *testing.T) {
	for _, raw_url := range []string{"https://v.youku.com/v_show/id_XNjU0MjIxOTc4NA==.html?from=test", "m.youku.com/video?vid=XNjU0MjIxOTc4NA=="} {
		page, cover, id, err := ParseURL(raw_url)
		if err != nil || cover != "" || id != "XNjU0MjIxOTc4NA==" || page != "https://v.youku.com/v_show/id_XNjU0MjIxOTc4NA==.html" {
			t.Fatalf("parse: %s %s %s %v", page, cover, id, err)
		}
	}
	for _, raw_url := range []string{"https://v.youku.com.evil.example/v_show/id_XNjU0MjIxOTc4NA==.html", "https://user@v.youku.com/v_show/id_XNjU0MjIxOTc4NA==.html", "file://v.youku.com/v_show/id_XNjU0MjIxOTc4NA==.html", "https://v.youku.com:8443/v_show/id_XNjU0MjIxOTc4NA==.html"} {
		if _, _, _, err := ParseURL(raw_url); err == nil {
			t.Fatalf("accepted invalid URL: %s", raw_url)
		}
	}
}

func TestParseSeriesURL(t *testing.T) {
	page, show_id, video_id, err := ParseURL("https://v.youku.com/v_nextstage/id_" + test_show_id + ".html?spm=tracking")
	if err != nil || page != "https://v.youku.com/v_nextstage/id_"+test_show_id+".html" || show_id != test_show_id || video_id != "" {
		t.Fatalf("series URL was not kept distinct from a video: %s %s %s %v", page, show_id, video_id, err)
	}
	for _, page := range []string{"https://v.youku.com/v_nextstage/id_.html", "https://v.youku.com.evil.example/v_nextstage/id_" + test_show_id + ".html", "https://user@v.youku.com/v_nextstage/id_" + test_show_id + ".html"} {
		if _, _, _, err := ParseURL(page); err == nil {
			t.Fatal("invalid series URL accepted")
		}
	}
}
