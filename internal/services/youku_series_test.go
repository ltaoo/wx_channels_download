package services

import (
	"encoding/json"
	"strings"
	"testing"

	"gorm.io/gorm"
	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper"
	"wx_channel/pkg/scraper/youku"
)

func TestYoukuSeriesDetailsAndCreditsPersistWithExistingTables(t *testing.T) {
	db := drama_test_db(t)
	page := "https://v.youku.com/v_nextstage/id_bcaa87c7834c46f99e7b.html"
	raw := &youku.FetchResult{Platform: "youku", SourceURL: page, Profile: scraper.Profile{
		ID: "736526", Kind: "series", Title: "剧集资料", Description: "完整简介", Homepage: page,
		CoverURL: "https://m.ykimg.com/poster.jpg", BackdropURL: "https://m.ykimg.com/backdrop.jpg", EpisodeCount: 30, AvailableEpisodeCount: 2,
		ReleaseYear: 2026, Genres: []string{"古装"}, Countries: []string{"中国"}, Status: "连载中", Tips: "更新至2集", Tagline: "剧集副标题",
		Credits: []scraper.Credit{{ID: "person-a", Name: "导演甲", Role: "director", SortOrder: 0}, {ID: "person-b", Name: "演员乙", Role: "actor", Character: "角色乙", AvatarURL: "https://m.ykimg.com/actor.jpg", SortOrder: 1}},
	}, Episodes: []scraper.Episode{
		{ID: "XNjU3MzkyOTA0NA==", SourceURL: "https://v.youku.com/v_show/id_XNjU3MzkyOTA0NA==.html", Title: "第一集", Order: 1, CoverURL: "https://m.ykimg.com/first.jpg", ReleaseDate: "2026-03-04", Description: "分集简介", Badge: "VIP"},
		{ID: "XNjU3MzkzMjAwOA==", SourceURL: "https://v.youku.com/v_show/id_XNjU3MzkzMjAwOA==.html", Title: "第二集", Order: 2},
	}}
	handler := adapter.Get("youku")
	builder := handler.(adapter.FetchDownloadTaskBuilder)
	info, err := builder.BuildDownloadTaskFromFetch(raw, json.RawMessage(`{}`))
	if err != nil || info.Content.Subtype != model.ContentSubtypeSeries || len(info.Resources) != 2 {
		t.Fatalf("series was not adapted to a series task: %v", err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, err := persist_download_task_graph(tx, info, info.Task, t.TempDir(), nil, "", 1000, false)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var series model.ContentSeries
	if err := db.First(&series, "id = ?", "youku:series:736526").Error; err != nil || series.EpisodeCount != 30 || series.SourceCount != 2 || series.GenresJSON != `["古装"]` || series.OriginCountryJSON != `["中国"]` || series.Status != "连载中" || series.InProduction != 1 || series.Homepage != page || series.AirDate != "" || !strings.Contains(series.MetadataJSON, `"release_year":2026`) {
		t.Fatalf("series fields were lost or an air date was invented: %+v %v", series, err)
	}
	var episode model.ContentEpisode
	if err := db.First(&episode, "id = ?", "youku:"+raw.Episodes[0].ID).Error; err != nil || episode.StillPath != raw.Episodes[0].CoverURL || episode.Overview != "分集简介" || episode.AirDate != "2026-03-04" || episode.Badge != "VIP" || episode.EpisodeNumber != "1" {
		t.Fatalf("episode details were lost: %+v %v", episode, err)
	}
	var people []model.ContentInfluencer
	if err := db.Where("content_id = ?", series.Id).Order("sort_order").Find(&people).Error; err != nil || len(people) != 2 || people[0].Role != "director" || people[1].Role != "actor" || !strings.Contains(people[1].MetadataJSON, "角色乙") {
		t.Fatalf("credits and character roles were not persisted: %+v %v", people, err)
	}
	// A selected episode keeps the same parent catalogue and exactly one resource.
	selected := *raw
	selected.SourceURL, selected.Episodes = raw.Episodes[1].SourceURL, raw.Episodes[1:]
	selected_info, err := builder.BuildDownloadTaskFromFetch(&selected, json.RawMessage(`{}`))
	if err != nil || selected_info.Content.Id != "youku:"+raw.Episodes[1].ID || len(selected_info.Resources) != 1 {
		t.Fatalf("selected episode unexpectedly became an entire-show download: %v", err)
	}
	var selected_parent *model.ContentSeries
	for _, detail := range selected_info.ContentDetails {
		if series, ok := detail.Data.(*model.ContentSeries); ok {
			selected_parent = series
		}
	}
	if selected_parent == nil || selected_parent.Id != "youku:series:736526" || selected_parent.SourceCount != 2 {
		t.Fatal("selected episode lost its parent series details")
	}
}
