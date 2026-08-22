package zhihuadapter

import (
	"encoding/json"
	"testing"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/zhihu"
)

func TestEmbeddedZhihuVideoIDs(t *testing.T) {
	content_html := `<p>完美嘛嘿嘿</p>
		<a class="video-box" href="https://link.zhihu.com/?target=https%3A//www.zhihu.com/video/2072359233719482104"
			data-video-id="" data-lens-id="2072359233719482104">https://www.zhihu.com/video/2072359233719482104</a>
		<a href="https://www.zhihu.com/video/2072359233719482105">另一个视频</a>`
	video_ids := embedded_zhihu_video_ids(content_html)
	if len(video_ids) != 2 {
		t.Fatalf("video ids = %#v, want 2 unique ids", video_ids)
	}
	if video_ids[0] != "2072359233719482104" || video_ids[1] != "2072359233719482105" {
		t.Fatalf("video ids = %#v", video_ids)
	}
}

func TestBuildZhihuEmbeddedVideosPersistsVariantsAndSelectsCompatibleHD(t *testing.T) {
	root_content := &model.Content{
		Id:          BuildTypedContentID("answer", "2072359423868248228"),
		PlatformId:  PlatformID,
		Type:        "answer",
		ExternalId:  "2072359423868248228",
		Title:       "你理想中的完美户型长什么样？",
		URL:         "https://www.zhihu.com/question/277577266/answer/2072359423868248228",
		SourceURL:   "https://www.zhihu.com/question/277577266/answer/2072359423868248228",
		PublishTime: int64_ptr(1_700_000_000_000),
		Timestamps:  model.Timestamps{CreatedAt: 1_700_000_000_000, UpdatedAt: 1_700_000_000_000},
	}
	play_info := zhihu_video_play_info_fixture(t)
	video_infos := []zhihu_embedded_video_info{{
		video_id: "2072359233719482104",
		info:     play_info,
	}}

	resources, details := build_zhihu_embedded_videos(
		root_content,
		video_infos,
		"answer_2072359423868248228",
		"",
		"",
	)
	if len(resources) != 1 || len(details) != 1 {
		t.Fatalf("resources = %d, details = %d; want 1, 1", len(resources), len(details))
	}
	content_video, ok := details[0].Data.(*model.ContentVideo)
	if !ok {
		t.Fatalf("detail type = %T, want *model.ContentVideo", details[0].Data)
	}
	if len(content_video.Variants) != 5 {
		t.Fatalf("variant count = %d, want 5", len(content_video.Variants))
	}
	if content_video.Variants[4].Spec != "20012" || content_video.Variants[4].IsDefault != 1 {
		t.Fatalf("selected variant = %#v, want H264 HD 20012", content_video.Variants[4])
	}
	if content_video.URL != content_video.Variants[4].URL || content_video.Codec != "H264" {
		t.Fatalf("content video = %#v", content_video)
	}
	if details[0].Content.Id != BuildTypedContentID("video", "2072359233719482104") ||
		details[0].Relation == nil ||
		details[0].Relation.Type != model.ContentRelationContains ||
		details[0].Relation.SourceContentId != root_content.Id {
		t.Fatalf("video detail = %#v", details[0])
	}

	resource := resources[0]
	if resource.Resource.Kind != "video/mp4" || resource.Resource.Size != 1_096_396 {
		t.Fatalf("resource = %#v", resource.Resource)
	}
	if len(resource.Endpoints) != 2 {
		t.Fatalf("endpoint count = %d, want 2 mirrors", len(resource.Endpoints))
	}
	var headers map[string]string
	if err := json.Unmarshal([]byte(resource.Endpoints[0].Headers), &headers); err != nil {
		t.Fatal(err)
	}
	if headers["Referer"] != root_content.SourceURL || headers["User-Agent"] == "" {
		t.Fatalf("endpoint headers = %#v", headers)
	}
	if len(resource.ContentAssets) != 1 || resource.ContentAssets[0].AssetKey != "2072359233719482104:format:20012" {
		t.Fatalf("resource assets = %#v", resource.ContentAssets)
	}
	if content_video.Variants[4].URLExpiresAt == nil || *content_video.Variants[4].URLExpiresAt != 1_787_343_788_000 {
		t.Fatalf("URL expiration = %#v", content_video.Variants[4].URLExpiresAt)
	}
}

func TestBuildZhihuEmbeddedVideosSelectsConfiguredVariant(t *testing.T) {
	root_content := &model.Content{
		Id:         BuildTypedContentID("answer", "content"),
		PlatformId: PlatformID,
		Type:       "answer",
		ExternalId: "content",
		Title:      "回答",
		SourceURL:  "https://www.zhihu.com/question/1/answer/content",
	}
	play_info := zhihu_video_play_info_fixture(t)
	resources, details := build_zhihu_embedded_videos(
		root_content,
		[]zhihu_embedded_video_info{{video_id: play_info.VideoPlay.ID, info: play_info}},
		"answer_content",
		"",
		"21013",
	)
	if len(resources) != 1 || len(details) != 1 {
		t.Fatalf("resources = %d, details = %d", len(resources), len(details))
	}
	content_video := details[0].Data.(*model.ContentVideo)
	if content_video.Variants[2].IsDefault != 1 || content_video.Variants[2].Spec != "21013" {
		t.Fatalf("selected variant = %#v, want 21013", content_video.Variants[2])
	}
	if resources[0].ContentAssets[0].AssetKey != content_video.Variants[2].VariantKey {
		t.Fatalf("resource asset = %#v", resources[0].ContentAssets[0])
	}
}

func TestZhihuArticleEmbeddedVideo(t *testing.T) {
	article_id := "1992203318399898641"
	video_id := "1992214443061424967"
	article_page := &zhihu.ArticlePage{
		Source: "https://zhuanlan.zhihu.com/p/" + article_id,
		Article: zhihu.Article{
			ID:      article_id,
			Title:   "《自然》：为什么百闻不如一见？",
			Content: `<p>完整正文</p><a class="video-box" data-lens-id="` + video_id + `" href="https://www.zhihu.com/video/` + video_id + `">视频</a>`,
		},
	}
	video_ids := embedded_zhihu_video_ids(zhihu_primary_content_html(article_page))
	if len(video_ids) != 1 || video_ids[0] != video_id {
		t.Fatalf("article video ids = %#v", video_ids)
	}
	if scene_code := zhihu_video_scene_code("article"); scene_code != "article_detail_web" {
		t.Fatalf("article scene code = %q", scene_code)
	}

	content, err := ArticleToContent(article_page)
	if err != nil {
		t.Fatal(err)
	}
	var play_info zhihu.VideoPlayInfo
	if err := json.Unmarshal([]byte(`{
		"za":{"content_type":5,"content_token":"1992203318399898641"},
		"video_play":{
			"id":"1992214443061424967",
			"default_cover":"https://pic1.zhimg.com/cover.jpg",
			"meta":{"mime":"video/mp4","duration":7.566667,"resolution":{"quality":"LD","width":512,"height":512}},
			"playlist":{"mp4":[{"key":20010,"name":"360P","label":"流畅 360P","quality":"LD","format":"mp4","codec":"H264","bitrate":30,"duration":7.566667,"width":512,"height":512,"size":28783,"fps":30,"url":["https://vdn3.vzuu.com/video.mp4?expiration=1787340618"]}]}
		}
	}`), &play_info); err != nil {
		t.Fatal(err)
	}
	resources, details := build_zhihu_embedded_videos(
		content,
		[]zhihu_embedded_video_info{{video_id: video_id, info: &play_info}},
		"article_"+article_id,
		"",
		"",
	)
	if len(resources) != 1 || len(details) != 1 {
		t.Fatalf("resources = %d, details = %d", len(resources), len(details))
	}
	content_video := details[0].Data.(*model.ContentVideo)
	if details[0].Relation == nil || details[0].Relation.SourceContentId != content.Id ||
		details[0].Relation.Type != model.ContentRelationContains ||
		content_video.Codec != "H264" || len(content_video.Variants) != 1 ||
		content_video.Variants[0].Spec != "20010" || content_video.Variants[0].IsDefault != 1 {
		t.Fatalf("article video detail = %#v, video = %#v", details[0], content_video)
	}
}

func zhihu_video_play_info_fixture(t *testing.T) *zhihu.VideoPlayInfo {
	t.Helper()
	fixture := `{
		"video_play": {
			"id": "2072359233719482104",
			"default_cover": "https://pic1.zhimg.com/cover.jpg",
			"meta": {"mime":"video/mp4","duration":12.133333,"resolution":{"quality":"HD","width":720,"height":1280},"hdr_type":"SDR"},
			"begin_frame": {"FHD":"https://picx.zhimg.com/frame.jpg","HD":"https://picx.zhimg.com/frame.jpg","SD":"https://picx.zhimg.com/frame.jpg"},
			"playlist": {"mp4": [
				{"key":21011,"name":"480P","label":"标清 480P","quality":"SD","format":"mp4","codec":"H265","maxbitrate":800,"bitrate":344,"duration":12.133333,"channels":2,"sample_rate":44100,"width":480,"height":852,"size":523078,"fps":30,"url":["https://vdn5.vzuu.com/480-h265.mp4?expiration=1787343788"]},
				{"key":21012,"name":"720P","label":"高清 720P","quality":"HD","format":"mp4","codec":"H265","maxbitrate":2300,"bitrate":512,"duration":12.133333,"channels":2,"sample_rate":44100,"width":720,"height":1280,"size":777126,"fps":30,"url":["https://vdn5.vzuu.com/720-h265.mp4?expiration=1787343788"]},
				{"key":21013,"name":"1080P","label":"超清 1080P","quality":"FHD","format":"mp4","codec":"H265","maxbitrate":3800,"bitrate":471,"duration":12.133333,"channels":2,"sample_rate":44100,"width":720,"height":1280,"size":715250,"fps":30,"url":["https://vdn5.vzuu.com/1080-h265.mp4?expiration=1787343788"]},
				{"key":20011,"name":"480P","label":"标清 480P","quality":"SD","format":"mp4","codec":"H264","maxbitrate":3000,"bitrate":421,"duration":12.133333,"channels":2,"sample_rate":44100,"width":478,"height":848,"size":638861,"fps":30,"url":["https://vdn6.vzuu.com/480-h264.mp4?expiration=1787343788"]},
				{"key":20012,"name":"720P","label":"高清 720P","quality":"HD","format":"mp4","codec":"H264","maxbitrate":5200,"bitrate":722,"duration":12.133333,"channels":2,"sample_rate":44100,"width":720,"height":1280,"size":1096396,"fps":30,"url":["https://vdn6.vzuu.com/720-h264.mp4?expiration=1787343788","https://backup.vzuu.com/720-h264.mp4?expiration=1787343788"]}
			]}
		}
	}`
	var info zhihu.VideoPlayInfo
	if err := json.Unmarshal([]byte(fixture), &info); err != nil {
		t.Fatal(err)
	}
	return &info
}
