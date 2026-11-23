package vod

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"wx_channel/pkg/scraper"

	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"wx_channel/internal/adapter"
	"wx_channel/internal/database"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/hermes"
	"wx_channel/pkg/hermes/protocol"
)

func prepare_db(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "task.db")+"?_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	connection, _ := db.DB()
	connection.SetMaxOpenConns(1)
	t.Cleanup(func() { connection.Close() })
	entries, _ := database.Migrations.ReadDir("migrations")
	var names []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".up.sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		raw, _ := database.Migrations.ReadFile("migrations/" + name)
		if err := db.Exec(string(raw)).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func fixture_spec(client *http.Client, resolve func(context.Context, string, scraper.Episode, string, *http.Client) (scraper.Video, error)) Specification {
	return Specification{PlatformID: "postprocess_fixture", HTTPClient: client, Resolve: func(ctx context.Context, page string, episode scraper.Episode, cookie string, client *http.Client) (scraper.Video, error) {
		video, err := resolve(ctx, page, episode, cookie, client)
		if err == nil {
			video.CheckMediaAddress = fixture_address
		}
		return video, err
	}, ParseURL: func(address string) (string, string, string, error) {
		if address != "https://page.example/episode" {
			return "", "", "", fmt.Errorf("invalid page")
		}
		return address, "", "episode", nil
	}}
}

func create_deferred_task(t *testing.T, db *gorm.DB, directory string) *hermes.TaskJob {
	t.Helper()
	input := Input{Platform: "postprocess_fixture", SourceURL: "https://page.example/episode", Episode: scraper.Episode{ID: "episode"}}
	row := model.DownloadTask{Name: "episode", PlatformId: input.Platform, UniqueID: "episode", ConfigJSON: string_marshal(map[string]any{"download_dir": directory}), MetadataJSON: "{}", Timestamps: model.Timestamps{CreatedAt: 1, UpdatedAt: 1}}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	resource := model.DownloadResource{TaskId: &row.Id, Name: "episode", Kind: "video/mp4", UniqueID: "episode", Type: "FILE", Extra: string_marshal(map[string]string{"vod_input": string_marshal(input)})}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.DownloadEndpoint{ResourceId: resource.Id, Protocol: SourceProtocol, URL: string_marshal(input), Headers: `{"Cookie":"account=private"}`, Enabled: 1}).Error; err != nil {
		t.Fatal(err)
	}
	job, err := database.NewDBTaskStore(db, nil).LoadTask(row.Id)
	if err != nil {
		t.Fatal(err)
	}
	return job
}

func TestPrepareRefreshesURLsAndPreservesResourceProgress(t *testing.T) {
	db := prepare_db(t)
	directory := t.TempDir()
	job := create_deferred_task(t, db, directory)
	version := 1
	h := NewHandler(fixture_spec(fixture_client(nil), func(ctx context.Context, page string, episode scraper.Episode, cookie string, client *http.Client) (scraper.Video, error) {
		if cookie != "account=private" && version == 1 {
			t.Errorf("metadata cookie missing: %q", cookie)
		}
		return scraper.Video{Info: scraper.PlayInfo{Duration: 2, Width: 32, Height: 32}, Playlist: fmt.Sprintf("#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXT-X-MAP:URI=\"https://pl-ali.youku.com/init.mp4?token=%d\"\n#EXTINF:2,\nhttps://pl-ali.youku.com/part.m4s?token=%d\n#EXT-X-ENDLIST\n", version, version)}, nil
	}))
	deps := adapter.PostprocessDeps{DB: db, BasePath: directory}
	if err := h.PrepareDownload(context.Background(), job, deps); err != nil {
		t.Fatal(err)
	}
	if len(job.Resources) != 2 {
		t.Fatalf("resource list not expanded: %+v", job.Resources)
	}
	ids := []int{job.Resources[0].ID, job.Resources[1].ID}
	if err := db.Create(&model.DownloadSegment{ResourceId: ids[1], Index: 0, Size: 100, Downloaded: 17}).Error; err != nil {
		t.Fatal(err)
	}
	version = 2
	job, err := database.NewDBTaskStore(db, nil).LoadTask(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.PrepareDownload(context.Background(), job, deps); err != nil {
		t.Fatal(err)
	}
	for index, resource := range job.Resources {
		endpoint := resource.Endpoints[0]
		if resource.ID != ids[index] || endpoint.Protocol != "https" || !strings.HasSuffix(endpoint.URL, "token=2") || endpoint.Cookies != "" || endpoint.Headers["Cookie"] != "" || endpoint.HTTPClient == nil {
			t.Fatalf("refresh changed resource identity or leaked credentials: %+v", resource)
		}
	}
	var segment model.DownloadSegment
	if err := db.First(&segment, "resource_id = ?", ids[1]).Error; err != nil || segment.Downloaded != 17 {
		t.Fatalf("persisted progress lost: %+v %v", segment, err)
	}
	files, _ := os.ReadDir(directory)
	if len(files) != 0 {
		t.Fatal("resource preparation wrote media files")
	}
	// A different media structure must not reuse old bytes.
	h.spec.Resolve = func(context.Context, string, scraper.Episode, string, *http.Client) (scraper.Video, error) {
		return scraper.Video{CheckMediaAddress: fixture_address, Info: scraper.PlayInfo{Address: "https://pl-ali.youku.com/changed.mp4", Duration: 2, Width: 32, Height: 32}}, nil
	}
	if err := h.PrepareDownload(context.Background(), job, deps); err == nil {
		t.Fatal("mixed a changed source with existing partial resources")
	}
}

func TestHermesDownloadsEachRawResourceAndAdapterPublishesMP4(t *testing.T) {
	require_media_tools(t)
	db := prepare_db(t)
	directory := t.TempDir()
	job := create_deferred_task(t, db, directory)
	segment, err := os.ReadFile(media_fixture("tiny.ts"))
	if err != nil {
		t.Fatal(err)
	}
	key := []byte("0123456789abcdef")
	padding := aes.BlockSize - len(segment)%aes.BlockSize
	clear := append(append([]byte(nil), segment...), bytes.Repeat([]byte{byte(padding)}, padding)...)
	encrypted := make([]byte, len(clear))
	block, _ := aes.NewCipher(key)
	cipher.NewCBCEncrypter(block, make([]byte, aes.BlockSize)).CryptBlocks(encrypted, clear)
	files := map[string][]byte{"/key": key, "/part.ts": encrypted}
	started := make(chan struct{}, 1)
	var blocked, resumed atomic.Bool
	var reads atomic.Int32
	client := &http.Client{Transport: prepared_transport(func(req *http.Request) (*http.Response, error) {
		reads.Add(1)
		if req.Header.Get("Cookie") != "" || req.Header.Get("User-Agent") != "fixture-agent" {
			t.Errorf("bad raw resource headers: %v", req.Header)
		}
		data, ok := files[req.URL.Path]
		if !ok {
			return nil, fmt.Errorf("unexpected resource %s", req.URL)
		}
		start, end := 0, len(data)-1
		if value := req.Header.Get("Range"); value != "" {
			if _, err := fmt.Sscanf(value, "bytes=%d-%d", &start, &end); err != nil {
				return nil, err
			}
		}
		if end >= len(data) {
			end = len(data) - 1
		}
		header := make(http.Header)
		header.Set("Content-Length", fmt.Sprint(end-start+1))
		header.Set("Content-Type", "application/octet-stream")
		header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
		response := &http.Response{StatusCode: 206, Header: header, ContentLength: int64(end - start + 1), Body: io.NopCloser(bytes.NewReader(data[start : end+1])), Request: req}
		if req.URL.Path == "/part.ts" && end > 511 && start == 0 && blocked.CompareAndSwap(false, true) {
			response.Body = &paused_fixture_body{ctx: req.Context(), first: data[:1024], started: started}
		}
		if req.URL.Path == "/part.ts" && start == 1024 {
			resumed.Store(true)
		}
		return response, nil
	})}
	jar, _ := cookiejar.New(nil)
	host, _ := url.Parse("https://pl-ali.youku.com")
	jar.SetCookies(host, []*http.Cookie{{Name: "account", Value: "private"}})
	client.Jar = jar
	var resolves atomic.Int32
	h := NewHandler(fixture_spec(client, func(ctx context.Context, page string, episode scraper.Episode, cookie string, http_client *http.Client) (scraper.Video, error) {
		version := resolves.Add(1)
		return scraper.Video{UserAgent: "fixture-agent", Info: scraper.PlayInfo{Duration: 2, Width: 32, Height: 32}, Playlist: fmt.Sprintf("#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXT-X-KEY:METHOD=AES-128,URI=\"https://pl-ali.youku.com/key?token=%d\",IV=0x00000000000000000000000000000000\n#EXTINF:2,\nhttps://pl-ali.youku.com/part.ts?token=%d\n#EXT-X-ENDLIST\n", version, version)}, nil
	}))
	if previous := adapter.Get(h.platform); previous != nil {
		registered := previous.(*Handler)
		registered.spec = h.spec
		h = registered
	} else {
		adapter.Register(h)
	}
	engine := hermes.New(hermes.HermesNewConfig{Store: database.NewDBTaskStore(db, nil), Config: hermes.HermesEngineConfig{BasePath: directory, ResourceConcurrency: 2}})
	engine.RegisterProtocol(protocol.NewHTTPDriver())
	engine.RegisterProtocol(protocol.NewFileDriver())
	engine.SetPreprocessor(adapter.NewPlatformPreprocessor(db, zerolog.Nop(), directory))
	engine.SetPostprocessor(adapter.NewPlatformPostprocessor(db, zerolog.Nop(), directory))
	t.Cleanup(engine.PauseAllTask)
	done := make(chan error, 1)
	var distinct_progress atomic.Bool
	engine.OnEvent(func(event hermes.EventType, data hermes.EventData) {
		if event == hermes.EventProgress {
			progress := data.(hermes.TaskProgressEventData).Progress
			if progress.ResourceCount == 2 && progress.Downloaded > 0 {
				distinct_progress.Store(true)
			}
		}
		if event == hermes.EventFinished {
			result := data.(hermes.TaskFinishedEventData)
			if len(result.Resources) != 1 || !strings.HasSuffix(result.FilePaths[0], ".mp4") {
				done <- fmt.Errorf("raw resources leaked into final outputs: %+v", result)
				return
			}
			done <- nil
		}
		if event == hermes.EventFailed {
			done <- fmt.Errorf("task failed: %s", data.(hermes.TaskFailedEventData).Error)
		}
	})
	if err := engine.StartTask(job.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("raw transfer never started")
	}
	engine.PauseTask(job.ID)
	var partial model.DownloadSegment
	if err := db.Where("resource_id != ? AND downloaded > 0", job.Resources[0].ID).First(&partial).Error; err != nil || partial.Downloaded != 1024 {
		t.Fatalf("pause did not persist the raw offset: %+v %v", partial, err)
	}
	if err := engine.StartTask(job.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("task timed out")
	}
	if resolves.Load() != 2 || reads.Load() < 4 || !distinct_progress.Load() || !resumed.Load() {
		t.Fatalf("missing deferred resolution or raw resource progress: resolves=%d reads=%d progress=%v resumed=%v", resolves.Load(), reads.Load(), distinct_progress.Load(), resumed.Load())
	}
	var final []model.DownloadResource
	if err := db.Where("task_id = ?", job.ID).Find(&final).Error; err != nil || len(final) != 1 {
		t.Fatalf("raw rows not cleaned: %+v %v", final, err)
	}
	var extra map[string]string
	if json.Unmarshal([]byte(final[0].Extra), &extra) != nil || extra["postprocessed"] != "true" {
		t.Fatal("postprocess state not archived")
	}
	filename := filepath.Join(directory, final[0].Name)
	if err := validate_media(context.Background(), filename, scraper.PlayInfo{Width: 32, Height: 32, Duration: 2}); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(directory)
	if len(entries) != 1 {
		t.Fatalf("raw/intermediate files remain: %v", entries)
	}
}

type prepared_transport func(*http.Request) (*http.Response, error)

func (f prepared_transport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type paused_fixture_body struct {
	ctx     context.Context
	first   []byte
	started chan struct{}
}

func (b *paused_fixture_body) Read(buffer []byte) (int, error) {
	if len(b.first) > 0 {
		count := copy(buffer, b.first)
		b.first = b.first[count:]
		if len(b.first) == 0 {
			b.started <- struct{}{}
		}
		return count, nil
	}
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}
func (b *paused_fixture_body) Close() error { return nil }

func TestCancelledPreparationKeepsDeferredResources(t *testing.T) {
	db := prepare_db(t)
	job := create_deferred_task(t, db, t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	h := NewHandler(fixture_spec(fixture_client(nil), func(context.Context, string, scraper.Episode, string, *http.Client) (scraper.Video, error) {
		cancel()
		return scraper.Video{}, context.Canceled
	}))
	if err := h.PrepareDownload(ctx, job, adapter.PostprocessDeps{DB: db}); !errors.Is(err, context.Canceled) {
		t.Fatalf("lost preparation cancellation: %v", err)
	}
	var count int64
	db.Model(&model.DownloadResource{}).Where("task_id = ?", job.ID).Count(&count)
	var endpoint model.DownloadEndpoint
	db.First(&endpoint, "resource_id = ?", job.Resources[0].ID)
	if count != 1 || endpoint.Protocol != SourceProtocol {
		t.Fatalf("cancelled preparation persisted a partial resource graph: %d %+v", count, endpoint)
	}
}

func TestResourceTransportAndRedirectPolicies(t *testing.T) {
	db := prepare_db(t)
	job := create_deferred_task(t, db, t.TempDir())
	var base_reads, special_reads atomic.Int32
	response := func(req *http.Request, text string) *http.Response {
		return &http.Response{StatusCode: 200, Header: make(http.Header), ContentLength: int64(len(text)), Body: io.NopCloser(strings.NewReader(text)), Request: req}
	}
	base := &http.Client{Transport: prepared_transport(func(req *http.Request) (*http.Response, error) {
		base_reads.Add(1)
		return response(req, "0123456789abcdef"), nil
	})}
	special := prepared_transport(func(req *http.Request) (*http.Response, error) {
		special_reads.Add(1)
		return response(req, "raw transport bytes"), nil
	})
	h := NewHandler(fixture_spec(base, func(context.Context, string, scraper.Episode, string, *http.Client) (scraper.Video, error) {
		return scraper.Video{Transport: special, Info: scraper.PlayInfo{Duration: 2, Width: 32, Height: 32}, Playlist: "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXT-X-KEY:METHOD=AES-128,URI=\"https://pl-ali.youku.com/key\"\n#EXTINF:2,\nhttps://pl-ali.youku.com/part.ts\n#EXT-X-ENDLIST\n"}, nil
	}))
	if err := h.PrepareDownload(context.Background(), job, adapter.PostprocessDeps{DB: db}); err != nil {
		t.Fatal(err)
	}
	driver := protocol.NewHTTPDriver()
	for _, resource := range job.Resources {
		reader, err := driver.Open(context.Background(), resource.Endpoints[0], hermes.ReadRequest{})
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, reader)
		reader.Close()
	}
	if base_reads.Load() != 1 || special_reads.Load() != 1 {
		t.Fatal("source transport was not limited to media resources")
	}
	var escaped atomic.Bool
	blocked := media_client(&http.Client{Transport: prepared_transport(func(req *http.Request) (*http.Response, error) {
		if req.URL.Hostname() != "pl-ali.youku.com" {
			escaped.Store(true)
		}
		result := response(req, "")
		result.StatusCode = 302
		result.Header.Set("Location", "https://evil.example/part.ts")
		return result, nil
	})}, fixture_address, nil)
	if _, err := blocked.Get("https://pl-ali.youku.com/part.ts"); err == nil || escaped.Load() {
		t.Fatalf("redirect escaped the source policy: %v", err)
	}
}
