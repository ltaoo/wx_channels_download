package services

import (
	"encoding/json"
	"strings"
	"testing"

	"gorm.io/gorm"
	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper"
	"wx_channel/pkg/scraper/iqiyi"
)

func TestIQiyiMetadataOnlyCapturePersistsWithoutPlayableStream(t *testing.T) {
	db := drama_test_db(t)
	page := "https://www.iqiyi.com/v_19rrjaajp0.html"
	raw := &iqiyi.FetchResult{Platform: "iqiyi", SourceURL: page, Profile: scraper.Profile{
		ID: "101738001", Kind: "series", Title: "西游记", Description: "剧集简介", CoverURL: "https://pic0.iqiyipic.com/poster.jpg",
		EpisodeCount: 25, AvailableEpisodeCount: 25, ReleaseYear: 1986, Status: "已完结",
	}, Episodes: []scraper.Episode{{ID: "100077400", SourceURL: page, Title: "猴王初问世", Order: 1}}}
	builder := adapter.Get("iqiyi").(adapter.FetchDownloadTaskBuilder)
	info, err := builder.BuildDownloadTaskFromFetch(raw, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if info.Content.Id != "iqiyi:100077400" || info.Content.Subtype != model.ContentSubtypeEpisode || len(info.Resources) != 1 {
		t.Fatalf("metadata-only capture lost the selected episode: %+v", info.Content)
	}
	endpoints := info.Resources[0].Endpoints
	if len(endpoints) != 1 || endpoints[0].Protocol != "media" || !strings.Contains(endpoints[0].URL, page) {
		t.Fatal("download source was not deferred to playback preparation")
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, err := persist_download_task_graph(tx, info, info.Task, t.TempDir(), nil, "", 1000, false)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var series model.ContentSeries
	if err := db.First(&series, "id = ?", "iqiyi:series:101738001").Error; err != nil || series.Name != "西游记" || series.EpisodeCount != 25 || series.SourceCount != 25 || series.AirDate != "" || !strings.Contains(series.MetadataJSON, `"release_year":1986`) {
		t.Fatalf("catalogue metadata lost or air date invented: %+v %v", series, err)
	}
	var episode model.ContentEpisode
	if err := db.First(&episode, "id = ?", "iqiyi:100077400").Error; err != nil || episode.Name != "猴王初问世" || episode.EpisodeNumber != "1" {
		t.Fatalf("selected episode metadata lost: %+v %v", episode, err)
	}
	var video model.ContentVideo
	if err := db.First(&video, "id = ?", episode.Id).Error; err != nil || video.URL != "" || video.Duration != 0 || video.Width != 0 || video.Height != 0 {
		t.Fatalf("unavailable media was given invented playback facts: %+v %v", video, err)
	}
	var relation model.ContentRelation
	if err := db.First(&relation, "source_content_id = ? AND type = ?", episode.Id, model.ContentRelationEpisodeOf).Error; err != nil || relation.TargetContentId != series.Id {
		t.Fatalf("episode parent was lost: %+v %v", relation, err)
	}
}
