package vqqadapter

import (
	"encoding/json"
	"testing"
	"wx_channel/internal/adapter"
	"wx_channel/internal/adapter/internal/vod"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper"
	"wx_channel/pkg/scraper/vqq"
)

func TestNativeResultAdaptsToDownloaderModels(t *testing.T) {
	raw := &vqq.FetchResult{Platform: PlatformID, SourceURL: "https://v.qq.com/x/cover/mzc002001w361jz.html", Profile: scraper.Profile{ID: "mzc002001w361jz", Kind: "series", Title: "作品", Description: "简介", ReleaseDate: "2026-01-02", EpisodeCount: 2}, Episodes: []scraper.Episode{{ID: "n3371za9efh", SourceURL: "https://v.qq.com/x/cover/mzc002001w361jz/n3371za9efh.html", Title: "分集", Order: 1, Duration: 120, Width: 1920, Height: 1080}, {ID: "v4102hrb7cq", SourceURL: "https://v.qq.com/x/cover/mzc002001w361jz/v4102hrb7cq.html", Title: "分集", Order: 2, Duration: 120, Width: 1920, Height: 1080}}}
	handler := adapter.Get(PlatformID)
	builder := handler.(adapter.FetchDownloadTaskBuilder)
	encoded, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var generic map[string]any
	if err := json.Unmarshal(encoded, &generic); err != nil {
		t.Fatal(err)
	}
	for _, data := range []any{raw, *raw, json.RawMessage(encoded), generic} {
		content, err := handler.ToContent(data)
		if err != nil {
			t.Fatal(err)
		}
		if content.PlatformId != PlatformID || content.Subtype != "series" {
			t.Fatalf("wrong normalized content: %+v", content)
		}
		info, err := builder.BuildDownloadTaskFromFetch(data, json.RawMessage(`{"filename":"输出"}`))
		if err != nil {
			t.Fatal(err)
		}
		if info.Task.PlatformId != PlatformID || info.Task.Name != "输出" || len(info.Resources) != len(raw.Episodes) || info.Account.PlatformId != PlatformID {
			t.Fatalf("wrong downloader task: %+v", info)
		}
		for index, resource := range info.Resources {
			if resource.Resource.ContentId == nil || *resource.Resource.ContentId != PlatformID+":"+raw.Episodes[index].ID || resource.Resource.Kind != "video/mp4" {
				t.Fatalf("wrong resource owner: %+v", resource)
			}
			if len(resource.Endpoints) != 1 || resource.Endpoints[0].Protocol != vod.SourceProtocol || len(resource.ContentAssets) != 1 || resource.ContentAssets[0].Role != model.ContentAssetRoleVideoVariant {
				t.Fatal("missing executable endpoint or video asset link")
			}
			var input vod.Input
			if err := json.Unmarshal([]byte(resource.Endpoints[0].URL), &input); err != nil || input.Platform != PlatformID || input.Episode.ID != raw.Episodes[index].ID || input.SourceURL != raw.Episodes[index].SourceURL {
				t.Fatalf("invalid downloader input: %+v %v", input, err)
			}
		}
		details, err := handler.ToContentDetails(data)
		if err != nil {
			t.Fatal(err)
		}
		relations := 0
		for _, detail := range details {
			if detail.Relation != nil {
				relations++
				if detail.Relation.Type != model.ContentRelationEpisodeOf || detail.Relation.TargetContentId != PlatformID+":series:"+raw.Profile.ID {
					t.Fatal("wrong episode-to-series relation")
				}
			}
		}
		wanted_relations := 0
		if raw.Profile.Kind == "series" {
			wanted_relations = len(raw.Episodes)
		}
		if relations != wanted_relations {
			t.Fatalf("relations=%d want=%d", relations, wanted_relations)
		}
	}
	if _, err := handler.BuildDownloadTask(json.RawMessage(encoded), json.RawMessage(`{}`)); err != nil {
		t.Fatalf("serialized source payload was not reusable: %v", err)
	}
	bad := *raw
	bad.Platform = "foreign"
	if _, err := builder.BuildDownloadTaskFromFetch(&bad, json.RawMessage(`{}`)); err == nil {
		t.Fatal("accepted another platform's payload")
	}
	bad = *raw
	bad.Episodes = append([]scraper.Episode(nil), raw.Episodes...)
	bad.Episodes[0].ID = "wrong"
	if _, err := builder.BuildDownloadTaskFromFetch(&bad, json.RawMessage(`{}`)); err == nil {
		t.Fatal("accepted a mismatched episode source")
	}
}

func TestIntroKeepsItsBadgeWithoutBecomingFirstEpisode(t *testing.T) {
	page := "https://v.qq.com/x/cover/mzc002001w361jz/n3371za9efh.html"
	raw := &vqq.FetchResult{Platform: PlatformID, SourceURL: page, Profile: scraper.Profile{ID: "mzc002001w361jz", Kind: "series", Title: "尚公主", Description: "剧集简介", EpisodeCount: 40, AvailableEpisodeCount: 30, ReleaseYear: 2026, Genres: []string{"古装"}}, Episodes: []scraper.Episode{{ID: "n3371za9efh", SourceURL: page, Title: "55秒故事前情", Badge: "先导片", Duration: 54}}}
	info, err := adapter.Get(PlatformID).(adapter.FetchDownloadTaskBuilder).BuildDownloadTaskFromFetch(raw, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if info.Content.Id != "tencent:n3371za9efh" || len(info.Resources) != 1 || info.Resources[0].Endpoints[0].Protocol != vod.SourceProtocol {
		t.Fatal("intro identity or deferred source changed")
	}
	var episode *model.ContentEpisode
	var series *model.ContentSeries
	for _, detail := range info.ContentDetails {
		switch value := detail.Data.(type) {
		case *model.ContentEpisode:
			episode = value
		case *model.ContentSeries:
			series = value
		}
	}
	if episode == nil || episode.Badge != "先导片" || episode.EpisodeNumber != "" || episode.SortOrder != 0 || episode.Duration != 54 {
		t.Fatalf("intro mislabeled as a full numbered episode: %+v", episode)
	}
	if series == nil || series.Name != "尚公主" || series.EpisodeCount != 40 || series.SourceCount != 30 || series.GenresJSON != `["古装"]` {
		t.Fatalf("parent catalogue metadata lost: %+v", series)
	}
}
