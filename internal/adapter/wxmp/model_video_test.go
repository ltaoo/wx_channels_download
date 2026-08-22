package wxmpadapter

import (
	"testing"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/wxmp"
)

func TestParseContentVideos(t *testing.T) {
	article := &wxmp.ArticleCgiDataNew{
		ContentNoencode: `<section><iframe class="video_iframe rich_pages" data-mpvid="wxv_first"></iframe></section>` +
			`<iframe class="video_iframe" data-src="https://mp.weixin.qq.com/mp/readtemplate?action=mpvideo&amp;vid=wxv_second"></iframe>` +
			`<iframe class="video_iframe" data-mpvid="wxv_first"></iframe>`,
		VideoPageInfos: []wxmp.VideoPageInfoItem{
			{
				VideoID: "wxv_second",
				MpVideoTransInfo: []wxmp.MpVideoTransInfo{{
					DurationMs: 9_000,
					Filesize:   float64(2_048),
					FormatID:   10002,
					Url:        "https://mpvideo.qpic.cn/second.mp4",
				}},
			},
			{
				VideoID: "wxv_first",
				MpVideoTransInfo: []wxmp.MpVideoTransInfo{
					{
						DurationMs: 53_000,
						Filesize:   "7810219",
						FormatID:   10002,
						Url:        "http://mpvideo.qpic.cn/first.mp4?token=one&amp;amp;quality=high",
					},
					{
						DurationMs: 53_000,
						Filesize:   "2508015",
						FormatID:   10004,
						Url:        "https://mpvideo.qpic.cn/first-low.mp4",
					},
				},
			},
		},
	}

	resources := parse_content_videos(article, "wxmp:content", "external", `{}`)
	if len(resources) != 2 {
		t.Fatalf("parse_content_videos() returned %d resources, want 2", len(resources))
	}

	first_resource := resources[0]
	if first_resource.Resource.Name != "video_01" {
		t.Errorf("first resource name = %q, want video_01", first_resource.Resource.Name)
	}
	if first_resource.Resource.Kind != "video/mp4" {
		t.Errorf("first resource kind = %q, want video/mp4", first_resource.Resource.Kind)
	}
	if first_resource.Resource.Size != 7_810_219 {
		t.Errorf("first resource size = %d, want 7810219", first_resource.Resource.Size)
	}
	if first_resource.Resource.Duration != 53 {
		t.Errorf("first resource duration = %d, want 53", first_resource.Resource.Duration)
	}
	if len(first_resource.Endpoints) != 1 {
		t.Fatalf("first resource has %d endpoints, want 1", len(first_resource.Endpoints))
	}
	if first_resource.Endpoints[0].URL != "https://mpvideo.qpic.cn/first.mp4?token=one&quality=high" {
		t.Errorf("first endpoint URL = %q", first_resource.Endpoints[0].URL)
	}
	if first_resource.Endpoints[0].Protocol != "https" {
		t.Errorf("first endpoint protocol = %q, want https", first_resource.Endpoints[0].Protocol)
	}
	if len(first_resource.ContentAssets) != 1 {
		t.Fatalf("first resource has %d assets, want 1", len(first_resource.ContentAssets))
	}
	asset := first_resource.ContentAssets[0]
	if asset.Kind != model.ContentAssetKindVideo ||
		asset.Role != model.ContentAssetRoleVideoVariant ||
		asset.AssetKey != "wxv_first:format:10002" {
		t.Errorf("first resource asset = %#v", asset)
	}

	second_resource := resources[1]
	if second_resource.Resource.UniqueID != "external_video_wxv_second_format_10002" {
		t.Errorf("second resource unique ID = %q", second_resource.Resource.UniqueID)
	}
}

func TestBuildWxmpEmbeddedVideosSelectsContentVideoVariant(t *testing.T) {
	article := &wxmp.ArticleCgiDataNew{
		ContentNoencode: `<iframe class="video_iframe" data-mpvid="wxv_video"></iframe>`,
		VideoPageInfos: []wxmp.VideoPageInfoItem{{
			VideoID: "wxv_video",
			MpVideoTransInfo: []wxmp.MpVideoTransInfo{
				{FormatID: 10002, Width: 720, Height: 1280, Filesize: "7810219", Url: "https://mpvideo.qpic.cn/high.mp4"},
				{FormatID: 10004, Width: 480, Height: 854, Filesize: "2508015", Url: "https://mpvideo.qpic.cn/low.mp4"},
			},
		}},
	}
	root_content := &model.Content{Id: "wxmp:article", PlatformId: PlatformID, Type: "article", Title: "文章"}

	resources, details := build_wxmp_embedded_videos(
		article,
		root_content,
		"article",
		`{}`,
		"wxv_video:format:10004",
		"10004",
	)
	if len(resources) != 1 || len(details) != 1 {
		t.Fatalf("resources = %d, details = %d; want 1, 1", len(resources), len(details))
	}
	if resources[0].Endpoints[0].URL != "https://mpvideo.qpic.cn/low.mp4" {
		t.Errorf("selected resource URL = %q", resources[0].Endpoints[0].URL)
	}
	content_video, ok := details[0].Data.(*model.ContentVideo)
	if !ok {
		t.Fatalf("detail data type = %T, want *model.ContentVideo", details[0].Data)
	}
	if len(content_video.Variants) != 2 {
		t.Fatalf("variant count = %d, want 2", len(content_video.Variants))
	}
	if content_video.Variants[1].IsDefault != 1 || content_video.URL != "https://mpvideo.qpic.cn/low.mp4" {
		t.Errorf("selected content video = %#v", content_video)
	}
	if details[0].Relation == nil || details[0].Relation.Type != model.ContentRelationContains {
		t.Errorf("video relation = %#v", details[0].Relation)
	}
}

func TestParseContentVideosFallsBackToVideoPageInfo(t *testing.T) {
	article := &wxmp.ArticleCgiDataNew{
		VideoPageInfos: []wxmp.VideoPageInfoItem{{
			HitVid: "wxv_fallback",
			MpVideoTransInfo: []wxmp.MpVideoTransInfo{{
				Url: "//mpvideo.qpic.cn/fallback.mp4",
			}},
		}},
	}

	resources := parse_content_videos(article, "wxmp:content", "external", `{}`)
	if len(resources) != 1 {
		t.Fatalf("parse_content_videos() returned %d resources, want 1", len(resources))
	}
	if resources[0].Endpoints[0].URL != "https://mpvideo.qpic.cn/fallback.mp4" {
		t.Errorf("fallback endpoint URL = %q", resources[0].Endpoints[0].URL)
	}
}
