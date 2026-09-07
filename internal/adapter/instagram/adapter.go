package instagramadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/scraper/instagram"
)

const PlatformID = instagram.PlatformID

type handler struct {
	runtime_mu    sync.RWMutex
	cookie_reader *cookies.Reader
}

var (
	_ adapter.PlatformAdapter             = (*handler)(nil)
	_ adapter.ContextProgressFetchAdapter = (*handler)(nil)
	_ adapter.FetchDownloadTaskBuilder    = (*handler)(nil)
	_ adapter.RuntimeAdapter              = (*handler)(nil)
	_ adapter.RuntimeHandle               = (*handler)(nil)
	_ adapter.PlatformStatusDescriber     = (*handler)(nil)
	_ adapter.HomeContentsBuilder         = (*handler)(nil)
	_ adapter.HomeDetailsFetcher          = (*handler)(nil)
)

func init() { adapter.Register(&handler{}) }

func (h *handler) PlatformID() string { return PlatformID }

func (h *handler) PlatformStatuses() []adapter.PlatformStatusDescriptor {
	return []adapter.PlatformStatusDescriptor{{Platform: PlatformID, Key: PlatformID, Name: "Instagram"}}
}

func (h *handler) RegisterRuntime(options *adapter.AdapterOptions) (adapter.RuntimeHandle, error) {
	if options == nil {
		return nil, fmt.Errorf("instagram: runtime dependencies are nil")
	}
	h.runtime_mu.Lock()
	h.cookie_reader = options.Cookies
	h.runtime_mu.Unlock()
	if options.Bus != nil {
		options.Bus.Publish(events.PlatformStatusChanged{Platform: PlatformID, Key: PlatformID, Name: "Instagram", Status: "available", Available: true})
	}
	return h, nil
}

func (h *handler) Stop() {
	h.runtime_mu.Lock()
	h.cookie_reader = nil
	h.runtime_mu.Unlock()
}

func (h *handler) Fetch(raw_url string) (any, error) {
	return h.FetchWithProgressContext(context.Background(), raw_url, adapter.FetchOptions{})
}

func (h *handler) FetchWithProgressContext(fetch_context context.Context, raw_url string, _ adapter.FetchOptions) (any, error) {
	client, err := h.new_scraper_client()
	if err != nil {
		return nil, err
	}
	defer client.Close()
	return client.FetchContext(fetch_context, raw_url)
}

func (h *handler) new_scraper_client() (*instagram.Client, error) {
	h.runtime_mu.RLock()
	cookie_reader := h.cookie_reader
	h.runtime_mu.RUnlock()
	return instagram.NewClient(cookie_reader)
}

func (h *handler) ToContent(data any) (*model.Content, error) {
	result, err := result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return to_content(result), nil
}

func (h *handler) ToAccount(data any) (*model.Account, error) {
	result, err := result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return to_account(result.Account), nil
}

func (h *handler) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	result, err := result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return to_details(result, to_content(result), to_account(result.Account)), nil
}

func (h *handler) BuildDownloadTask(content_json json.RawMessage, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	var input struct {
		ExternalID string `json:"external_id"`
		URL        string `json:"url"`
		SourceURL  string `json:"source_url"`
	}
	if err := json.Unmarshal(content_json, &input); err != nil {
		return nil, fmt.Errorf("instagram: decode download data: %w", err)
	}
	if input.ExternalID != "" {
		return h.BuildDownloadTaskFromFetch(content_json, config_json)
	}
	raw_url := input.SourceURL
	if raw_url == "" {
		raw_url = input.URL
	}
	data, err := h.Fetch(raw_url)
	if err != nil {
		return nil, err
	}
	return h.BuildDownloadTaskFromFetch(data, config_json)
}

func (h *handler) BuildDownloadTaskFromFetch(data any, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	result, err := result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	config_text := strings.TrimSpace(string(config_json))
	if config_text == "" || config_text == "null" {
		config_text = "{}"
	}
	var config struct {
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal([]byte(config_text), &config); err != nil {
		return nil, fmt.Errorf("instagram: decode download config: %w", err)
	}
	content, account := to_content(result), to_account(result.Account)
	details := to_details(result, content, account)
	task_name := strings.TrimSpace(config.Filename)
	if task_name == "" {
		task_name = content.Title
	}
	headers, _ := json.Marshal(map[string]string{"Referer": "https://www.instagram.com/", "User-Agent": instagram.DefaultUserAgent, "Accept": "*/*"})
	resources := make([]*adapter.ResourceInfo, 0, len(result.Media))
	for media_index, media := range result.Media {
		content_id := content.Id
		resource_name := task_name
		if len(result.Media) > 1 {
			resource_name += fmt.Sprintf("_%02d", media_index+1)
		}
		kind := "image"
		image_key := model.BuildContentAlbumImageKey(media.ID, media.URL, media_index)
		asset := adapter.ContentAssetReference{
			Kind: model.ContentAssetKindImage, Role: model.ContentAssetRolePrimary,
			AssetKey: model.BuildContentAlbumImageAssetKey(image_key, "original"), Relation: model.DownloadResourceAssetRelationSource,
			SubjectType: model.ContentAssetSubjectAlbumImage, SubjectKey: image_key, SubjectRelation: model.ContentAssetSubjectRelationRepresentation,
		}
		if media.Type == "video" {
			kind = "video/mp4"
			if len(result.Media) > 1 {
				content_id = video_content_id(result, media)
			}
			asset = adapter.ContentAssetReference{Kind: model.ContentAssetKindVideo, Role: model.ContentAssetRoleVideoVariant, AssetKey: "default", Relation: model.DownloadResourceAssetRelationSource}
		}
		resources = append(resources, &adapter.ResourceInfo{
			Resource:      model.DownloadResource{ContentId: &content_id, Name: resource_name, Kind: kind, UniqueID: result.ExternalID + "_" + media.ID, MergeOrder: media_index},
			Endpoints:     []model.DownloadEndpoint{{Protocol: "https", URL: media.URL, Enabled: 1, Headers: string(headers)}},
			ContentAssets: []adapter.ContentAssetReference{asset},
		})
	}
	return &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{ContentId: &content.Id, Name: task_name, UniqueID: result.ExternalID, PlatformId: PlatformID,
			Status: model.TaskStatusWaiting, SourceURL: content.SourceURL, CoverURL: content.CoverURL,
			CoverWidth: content.CoverWidth, CoverHeight: content.CoverHeight, ConfigJSON: config_text, MetadataJSON: content.Metadata, Timestamps: content.Timestamps},
		Resources: resources, Content: content, Account: account, ContentDetail: details[0].Data, ContentDetails: details,
	}, nil
}

func (h *handler) BuildBrowseHistory(content_json json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	result, err := result_from_fetch(content_json)
	if err != nil {
		return nil, err
	}
	content := to_content(result)
	return &adapter.BrowseHistoryResult{
		BrowseHistory: &model.BrowseHistory{Id: content.Id, PlatformId: PlatformID, VisitedTimes: 1, Type: content.Type,
			ExternalId: content.ExternalId, Title: content.Title, URL: content.URL, SourceURL: content.SourceURL,
			CoverURL: content.CoverURL, CoverWidth: content.CoverWidth, CoverHeight: content.CoverHeight,
			PublishTime: content.PublishTime, ExtraData: content.Metadata, Timestamps: content.Timestamps},
		Account: to_account(result.Account),
	}, nil
}
