package fanqienoveladapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/pkg/scraper/fanqienovel"
)

const platform_id_fanqienovel = "fanqienovel"

// PlatformID is the platform identifier for Fanqie Novel.
const PlatformID = platform_id_fanqienovel

// FanqieNovelAdapter converts Fanqie scraper results into shared models.
type FanqieNovelAdapter struct {
	runtime_mu sync.RWMutex
	bus        *events.Bus
	work_dir   string
}

var _ adapter.PlatformAdapter = (*FanqieNovelAdapter)(nil)
var _ adapter.ProgressFetchAdapter = (*FanqieNovelAdapter)(nil)
var _ adapter.ContextProgressFetchAdapter = (*FanqieNovelAdapter)(nil)
var _ adapter.FetchCacheAdapter = (*FanqieNovelAdapter)(nil)
var _ adapter.RuntimeAdapter = (*FanqieNovelAdapter)(nil)
var _ adapter.RuntimeHandle = (*FanqieNovelAdapter)(nil)
var _ adapter.Postprocessor = (*FanqieNovelAdapter)(nil)

func init() {
	adapter.Register(NewFanqieNovelAdapter())
}

func NewFanqieNovelAdapter() *FanqieNovelAdapter {
	return &FanqieNovelAdapter{}
}

func (a *FanqieNovelAdapter) PlatformID() string { return PlatformID }

func (a *FanqieNovelAdapter) Fetch(raw_url string) (any, error) {
	return a.FetchWithProgress(raw_url, "")
}

// FetchWithProgress fetches a book and associates emitted progress events with
// request_id so WebSocket clients can select their own request.
func (a *FanqieNovelAdapter) FetchWithProgress(raw_url string, request_id string) (any, error) {
	return a.FetchWithProgressContext(context.Background(), raw_url, adapter.FetchOptions{
		RequestID: request_id,
	})
}

// FetchWithProgressContext fetches a book with cancellation and cache control.
func (a *FanqieNovelAdapter) FetchWithProgressContext(fetch_context context.Context, raw_url string, options adapter.FetchOptions) (any, error) {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return nil, fmt.Errorf("番茄小说 URL 不能为空")
	}
	client := fanqienovel.NewFanqieClient()
	client.SetProgressHandler(func(progress fanqienovel.FetchProgress) {
		a.publish_progress(progress)
		a.emit_fetch_artifacts(progress, options.ArtifactHandler)
	})
	client.SetWorkDir(a.runtime_work_dir())
	return client.Fetch(fanqienovel.FetchParams{
		URL:          raw_url,
		RequestID:    strings.TrimSpace(options.RequestID),
		ForceRefresh: options.ForceRefresh,
		Context:      fetch_context,
	})
}

// ClearFetchCache removes cached profile and chapter HTML for raw_url.
func (a *FanqieNovelAdapter) ClearFetchCache(raw_url string) (bool, error) {
	return fanqienovel.ClearHTMLCache(a.runtime_work_dir(), strings.TrimSpace(raw_url))
}

func (a *FanqieNovelAdapter) RegisterRuntime(runtime_deps adapter.RuntimeDeps) (adapter.RuntimeHandle, error) {
	a.runtime_mu.Lock()
	a.bus = runtime_deps.Bus
	if runtime_deps.Config != nil {
		a.work_dir = runtime_deps.Config.WorkDir
	}
	a.runtime_mu.Unlock()
	return a, nil
}

func (a *FanqieNovelAdapter) Stop() {
	a.runtime_mu.Lock()
	a.bus = nil
	a.work_dir = ""
	a.runtime_mu.Unlock()
}

func (a *FanqieNovelAdapter) runtime_work_dir() string {
	a.runtime_mu.RLock()
	work_dir := a.work_dir
	a.runtime_mu.RUnlock()
	return work_dir
}

func (a *FanqieNovelAdapter) publish_progress(progress fanqienovel.FetchProgress) {
	a.runtime_mu.RLock()
	bus := a.bus
	a.runtime_mu.RUnlock()
	if bus == nil {
		return
	}
	bus.Publish(events.ScraperFetchProgress{
		RequestID:    progress.RequestID,
		Platform:     progress.Platform,
		URL:          progress.URL,
		BookID:       progress.BookID,
		BookTitle:    progress.BookTitle,
		Stage:        progress.Stage,
		Status:       progress.Status,
		Current:      progress.Current,
		Total:        progress.Total,
		Percent:      progress.Percent,
		VolumeTitle:  progress.VolumeTitle,
		ChapterID:    progress.ChapterID,
		ChapterTitle: progress.ChapterTitle,
		Message:      progress.Message,
		Error:        progress.Error,
		Cached:       progress.Cached,
		CacheHits:    progress.CacheHits,
	})
}

func (a *FanqieNovelAdapter) ToContent(data any) (*model.Content, error) {
	result, err := fanqie_result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return ToContent(result)
}

func (a *FanqieNovelAdapter) ToAccount(data any) (*model.Account, error) {
	result, err := fanqie_result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return ToAccount(result)
}

func (a *FanqieNovelAdapter) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	result, err := fanqie_result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return ToContentDetails(result)
}

func (a *FanqieNovelAdapter) emit_fetch_artifacts(progress fanqienovel.FetchProgress, artifact_handler adapter.FetchArtifactHandler) {
	if artifact_handler == nil {
		return
	}
	if progress.Stage == fanqienovel.FetchStageDirectory && progress.Status == fanqienovel.FetchStatusCompleted && progress.Profile != nil {
		partial_result := &fanqienovel.FanqieFetchResult{Profile: progress.Profile}
		content, content_err := ToContent(partial_result)
		if content_err != nil {
			return
		}
		artifact_handler(adapter.FetchArtifact{Stage: adapter.FetchArtifactStageContent, Content: content})
		if account, account_err := ToAccount(partial_result); account_err == nil && account != nil {
			artifact_handler(adapter.FetchArtifact{Stage: adapter.FetchArtifactStageAccount, Account: account})
		}
		novel, novel_err := ToContentNovel(partial_result, content.Id)
		if novel_err == nil {
			detail := adapter.ContentDetail{Type: "novel", Key: content.Id, Data: novel}
			artifact_handler(adapter.FetchArtifact{Stage: adapter.FetchArtifactStageContentDetail, ContentDetail: &detail})
		}
		volumes, volumes_err := ToContentNovelVolumes(partial_result, content.Id)
		if volumes_err == nil {
			for volume_index := range volumes {
				volume := volumes[volume_index]
				detail := adapter.ContentDetail{
					Type: "novel_volume",
					Key:  fmt.Sprintf("%s:volume:%d", content.Id, volume.Idx),
					Data: &volume,
				}
				artifact_handler(adapter.FetchArtifact{Stage: adapter.FetchArtifactStageContentDetail, ContentDetail: &detail})
			}
		}
		return
	}
	if progress.Stage == fanqienovel.FetchStageChapter && progress.Status == fanqienovel.FetchStatusCompleted && progress.Chapter != nil {
		content_id := BuildContentID(progress.BookID)
		detail := fetched_chapter_content_detail(*progress.Chapter, content_id)
		artifact_handler(adapter.FetchArtifact{
			Stage:         adapter.FetchArtifactStageContentDetail,
			ContentDetail: &detail,
			Current:       progress.Current,
			Total:         progress.Total,
		})
	}
}

func (a *FanqieNovelAdapter) BuildBrowseHistory(content_json json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	result, err := fanqie_result_from_json(content_json)
	if err != nil {
		return nil, err
	}
	content, err := ToContent(result)
	if err != nil {
		return nil, err
	}
	account, err := ToAccount(result)
	if err != nil {
		return nil, err
	}
	extra_data, _ := json.Marshal(map[string]any{
		"chapter_count": result.Profile.ChapterCount,
		"volume_count":  len(result.Profile.Volumes),
	})
	return &adapter.BrowseHistoryResult{
		BrowseHistory: &model.BrowseHistory{
			Id:           content.Id,
			PlatformId:   PlatformID,
			VisitedTimes: 1,
			Type:         content.Type,
			ExternalId:   content.ExternalId,
			Title:        content.Title,
			URL:          content.URL,
			SourceURL:    content.SourceURL,
			CoverURL:     content.CoverURL,
			ExtraData:    string(extra_data),
			Timestamps:   content.Timestamps,
		},
		Account: account,
	}, nil
}

func (a *FanqieNovelAdapter) BuildDownloadTask(content_json json.RawMessage, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	result, err := fanqie_result_from_json(content_json)
	if err != nil {
		var params fanqienovel.FetchParams
		if decode_err := json.Unmarshal(content_json, &params); decode_err != nil || strings.TrimSpace(params.URL) == "" {
			return nil, err
		}
		fetched, fetch_err := a.Fetch(params.URL)
		if fetch_err != nil {
			return nil, fetch_err
		}
		result, err = fanqie_result_from_fetch(fetched)
		if err != nil {
			return nil, err
		}
	}
	return build_download_task(result, config_json, a.runtime_work_dir())
}
