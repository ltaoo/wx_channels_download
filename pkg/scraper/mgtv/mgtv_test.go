package mgtv

import "testing"

func TestParseTVProfileHTML(t *testing.T) {
	html := `<script>window.__INITIAL_STATE__={"playPage":{"videoinfo":{"6":"简介：内容","seriesId":"s1","seriesName":"剧名","image":"poster.jpg","clipName":"第一季"}}}</script><`
	got, err := ParseTVProfileHTML("https://m.mgtv.com/h/1/2.html", html)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "第一季" || got.Overview != "内容" || got.PosterPath != "poster.jpg" {
		t.Fatalf("unexpected profile %#v", got)
	}
}

func TestParsePlayURL(t *testing.T) {
	clipID, videoID, ok := ParsePlayURL("https://www.mgtv.com/b/648984/23111391.html?_source_=B")
	if !ok || clipID != "648984" || videoID != "23111391" {
		t.Fatalf("unexpected ids: clip=%q video=%q ok=%v", clipID, videoID, ok)
	}
}

func TestParseVODInfoResponse(t *testing.T) {
	resp := vodInfoResponse{Code: 200}
	resp.Data.Info.ShareInfo.Title = "分享标题"
	resp.Data.Info.ShareInfo.Image = "share.jpg"
	resp.Data.Info.Video.VideoID = "23111391"
	resp.Data.Info.Video.PartName = "双人海报拍摄花絮"
	resp.Data.Info.Video.ReleaseTime = "2025-05-17 00:00:00"
	resp.Data.Info.Clip.ClipID = "648984"
	resp.Data.Info.Clip.ClipName = "韶华若锦"
	resp.Data.Info.Clip.Kind = "古装/爱情"
	resp.Data.Info.Clip.Story = "剧情简介"
	resp.Data.Info.Clip.VImgURL = "poster.jpg"
	resp.Data.Info.Clip.SerialCount = "30"
	resp.Data.Info.Template.Modules = []vodModule{{
		Title: "视频",
		Media: &vodMedia{List: []vodMediaItem{
			{VideoID: "22892149", ClipID: "648984", SerialNo: "1", Title: "预约破200万", Duration: "00:13", HImgURL: "1.jpg"},
			{VideoID: "23111391", ClipID: "648984", SerialNo: "2", Title: "双人海报拍摄花絮", Duration: "00:45", HImgURL: "2.jpg"},
		}},
	}}

	got, err := parseVODInfoResponse(resp)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "648984" || got.VideoID != "23111391" || got.Name != "韶华若锦" {
		t.Fatalf("unexpected profile %#v", got)
	}
	if got.CurrentEpisode == nil || got.CurrentEpisode.Name != "双人海报拍摄花絮" {
		t.Fatalf("unexpected current episode %#v", got.CurrentEpisode)
	}
	if len(got.Seasons) != 1 || len(got.Seasons[0].Episodes) != 2 {
		t.Fatalf("unexpected seasons %#v", got.Seasons)
	}
}

func TestParseSearchResponse(t *testing.T) {
	resp := searchResponse{}
	resp.Data.Contents = append(resp.Data.Contents, struct {
		Type string `json:"type"`
		Name string `json:"name"`
		Data []struct {
			Title  string `json:"title"`
			Desc   any    `json:"desc"`
			Img    string `json:"img"`
			URL    string `json:"url"`
			Source string `json:"source"`
		} `json:"data"`
	}{
		Type: "media",
		Data: []struct {
			Title  string `json:"title"`
			Desc   any    `json:"desc"`
			Img    string `json:"img"`
			URL    string `json:"url"`
			Source string `json:"source"`
		}{
			{Title: "<B>剧</B>名", Desc: "类型: 综艺/大陆/2023", Img: "img.jpg", URL: "/h/1.html", Source: "mgtv"},
		},
	})
	got, err := parseSearchResponse(resp)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.List) != 1 || got.List[0].Name != "剧名" || got.List[0].OriginCountry[0] != "CN" {
		t.Fatalf("unexpected result %#v", got)
	}
}
