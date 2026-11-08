package services

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"wx_channel/pkg/scraper"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"wx_channel/internal/adapter"
	_ "wx_channel/internal/adapter/hongguo"
	_ "wx_channel/internal/adapter/iqiyi"
	_ "wx_channel/internal/adapter/mgtv"
	_ "wx_channel/internal/adapter/vqq"
	_ "wx_channel/internal/adapter/youku"
	"wx_channel/internal/database"
	"wx_channel/internal/database/model"

	"wx_channel/pkg/hermes"
	"wx_channel/pkg/scraper/hongguo"
	"wx_channel/pkg/scraper/youku"
)

func drama_test_db(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "source.db")+"?_foreign_keys=on"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sql_db, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sql_db.Close() })
	entries, err := database.Migrations.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".up.sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		raw, err := database.Migrations.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(string(raw)).Error; err != nil {
			t.Fatalf("migration %s: %v", name, err)
		}
	}
	return db
}

func TestDramaSeriesAndMoviePersistWithExistingSchema(t *testing.T) {
	db := drama_test_db(t)
	result := &hongguo.FetchResult{Platform: "hongguo", SourceURL: "https://hongguoduanju.com/player/123456789", Profile: scraper.Profile{ID: "123456789", Kind: "series", Title: "系列", EpisodeCount: 2}, Episodes: []scraper.Episode{
		{ID: "9007199254740993", SourceURL: "https://hongguoduanju.com/player/123456789/9007199254740993", Title: "第一集", Order: 1, Duration: 120, Width: 720, Height: 1280},
		{ID: "9007199254740994", SourceURL: "https://hongguoduanju.com/player/123456789/9007199254740994", Title: "第二集", Order: 2},
	}}
	persist := func(platform string, result any) *adapter.DownloadTaskResult {
		t.Helper()
		handler := adapter.Get(platform)
		builder := handler.(adapter.FetchDownloadTaskBuilder)
		info, err := builder.BuildDownloadTaskFromFetch(result, json.RawMessage(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		// This is the same transaction used by ordinary download task creation.
		err = db.Transaction(func(tx *gorm.DB) error {
			_, err := persist_download_task_graph(tx, info, info.Task, t.TempDir(), nil, "", 1000, false)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return info
	}
	info := persist("hongguo", result)
	if info.Content.Subtype != model.ContentSubtypeSeries || len(info.Resources) != 2 {
		t.Fatalf("invalid series task: %+v", info)
	}
	for table, wanted := range map[string]int64{"content": 3, "content_series": 1, "content_episode": 2, "content_video": 2, "content_relation": 2, "content_video_variant": 2, "download_resource_asset": 2} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil || count != wanted {
			t.Fatalf("%s count=%d want=%d err=%v", table, count, wanted, err)
		}
	}
	var episode model.ContentEpisode
	if err := db.First(&episode, "id = ?", "hongguo:9007199254740994").Error; err != nil || episode.EpisodeNumber != "2" {
		t.Fatalf("episode ID/order lost: %+v %v", episode, err)
	}
	var relation model.ContentRelation
	if err := db.First(&relation, "source_content_id = ?", episode.Id).Error; err != nil || relation.TargetContentId != "hongguo:series:123456789" || relation.Type != model.ContentRelationEpisodeOf {
		t.Fatalf("parent relation lost: %+v %v", relation, err)
	}
	archived, err := export_task_content(db, episode.Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(archived.Details["episode"]) == 0 || len(archived.Details["video"]) == 0 {
		t.Fatal("archive lost episode or media details")
	}
	movie := &youku.FetchResult{Platform: "youku", SourceURL: "https://v.youku.com/v_show/id_XNjU0MjIxOTc4NA==.html", Profile: scraper.Profile{ID: "film", Kind: "movie", Title: "电影", Description: "简介", ReleaseDate: "2026-01-02", Metadata: json.RawMessage(`{"@type":"Movie","actor":[{"name":"演员"}]}`)}, Episodes: []scraper.Episode{{ID: "XNjU0MjIxOTc4NA==", SourceURL: "https://v.youku.com/v_show/id_XNjU0MjIxOTc4NA==.html", Duration: 7200, Width: 1920, Height: 1080}}}
	movie_info := persist("youku", movie)
	saved, err := export_task_content(db, movie_info.Content.Id)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Content.Type != model.ContentTypeVideo || saved.Content.Subtype != model.ContentSubtypeMovie || !strings.Contains(saved.Content.Metadata, "2026-01-02") || !strings.Contains(saved.Content.Metadata, "演员") {
		t.Fatalf("movie metadata lost: %+v", saved.Content)
	}
	if len(saved.Details["video"]) == 0 || len(saved.Details["episode"]) != 0 {
		t.Fatal("movie was stored as an episode")
	}
	// Re-fetching a selected episode keeps its parent and stable large identifiers.
	result.SourceURL = result.Episodes[1].SourceURL
	result.Episodes = result.Episodes[1:]
	selected := persist("hongguo", result)
	if selected.Content.Id != episode.Id || selected.Content.Subtype != model.ContentSubtypeEpisode {
		t.Fatal("single-episode root changed identity")
	}
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		var resource model.DownloadResource
		if err := db.Where("task_id = ?", selected.Task.Id).First(&resource).Error; err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join("..", "adapter", "internal", "vod", "testdata", "tiny.mp4"))
		if err != nil {
			t.Fatal(err)
		}
		filename := filepath.Join(t.TempDir(), "episode.mp4")
		if err := os.WriteFile(filename, raw, 0600); err != nil {
			t.Fatal(err)
		}
		plan := map[string]any{"version": 1, "info": scraper.PlayInfo{Width: 32, Height: 32, Duration: 2}, "items": []map[string]any{{"unique_id": resource.UniqueID, "path": "input.mp4"}}, "fingerprint": "fixture"}
		encoded, _ := json.Marshal(plan)
		job := &hermes.TaskJob{Resources: []hermes.ResourceJob{{ID: resource.Id, UniqueID: resource.UniqueID, Kind: "video/mp4", FilePath: filename, Size: int64(len(raw)), Extra: map[string]string{"vod_plan": string(encoded)}}}}
		processor := adapter.Get("hongguo").(adapter.Postprocessor)
		if err := processor.Postprocess(context.Background(), job, adapter.PostprocessDeps{DB: db}); err != nil {
			t.Fatal(err)
		}
		final, err := os.ReadFile(filename)
		if err != nil || string(final[4:8]) != "ftyp" || job.Resources[0].Size != int64(len(final)) {
			t.Fatalf("postprocess did not publish final MP4: %v", err)
		}
		var video model.ContentVideo
		if err := db.First(&video, "id = ?", episode.Id).Error; err != nil || video.Duration != 2 || video.Width != 32 || video.Height != 32 || video.Size != int64(len(final)) {
			t.Fatalf("lazy media facts were not archived: %+v %v", video, err)
		}
	}
	var count int64
	if err := db.Table("content_episode").Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("repeat created duplicate episodes: %d %v", count, err)
	}
}

func TestDramaPlatformRouting(t *testing.T) {
	for platform, raw_url := range map[string]string{
		"iqiyi":   "https://www.iqiyi.com/v_eienwprun0.html",
		"youku":   "https://v.youku.com/v_show/id_XNjU0MjIxOTc4NA==.html",
		"tencent": "https://v.qq.com/x/cover/mzc002001w361jz/n3371za9efh.html",
		"mgtv":    "https://www.mgtv.com/b/914227/24771585.html",
		"hongguo": "https://hongguoduanju.com/detail?series_id=123456789",
	} {
		detected, err := DetectScraperPlatform(raw_url)
		if err != nil || detected != platform || adapter.Get(detected) == nil {
			t.Fatalf("routing %s: %s %v", raw_url, detected, err)
		}
	}
	if detected, err := DetectScraperPlatform("https://v.qq.com.evil.example/x/page/n3371za9efh.html"); err != nil || detected == "tencent" {
		t.Fatal("accepted spoofed platform host")
	}
}
