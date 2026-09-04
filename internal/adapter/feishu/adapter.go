package feishuadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rs/zerolog"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/pkg/cache"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/scraper/feishu"
	"wx_channel/pkg/util"
)

const PlatformID = feishu.PlatformID

func init() {
	adapter.Register(NewFeishuAdapter())
}

// FeishuAdapter connects the Feishu document scraper to the shared registry.
type FeishuAdapter struct {
	runtime_mu      sync.RWMutex
	logger          *zerolog.Logger
	cookie_provider *cookies.Reader
	file_cache      *cache.CacheProvider
}

var (
	_ adapter.PlatformAdapter             = (*FeishuAdapter)(nil)
	_ adapter.ContextProgressFetchAdapter = (*FeishuAdapter)(nil)
	_ adapter.FetchDownloadTaskBuilder    = (*FeishuAdapter)(nil)
	_ adapter.RuntimeAdapter              = (*FeishuAdapter)(nil)
	_ adapter.RuntimeHandle               = (*FeishuAdapter)(nil)
	_ adapter.PlatformStatusDescriber     = (*FeishuAdapter)(nil)
)

// NewFeishuAdapter creates a Feishu document adapter.
func NewFeishuAdapter() *FeishuAdapter { return &FeishuAdapter{} }

func (a *FeishuAdapter) PlatformID() string { return PlatformID }

// PlatformStatuses describes the Feishu document scraper.
func (a *FeishuAdapter) PlatformStatuses() []adapter.PlatformStatusDescriptor {
	return []adapter.PlatformStatusDescriptor{{Platform: PlatformID, Key: PlatformID, Name: "飞书文档"}}
}

// RegisterRuntime attaches cookie, cache, config, and logging dependencies.
func (a *FeishuAdapter) RegisterRuntime(adapter_options *adapter.AdapterOptions) (adapter.RuntimeHandle, error) {
	if a == nil {
		return nil, errors.New("feishu adapter is nil")
	}
	if adapter_options == nil {
		return nil, errors.New("feishu runtime dependencies are nil")
	}
	a.runtime_mu.Lock()
	a.logger = adapter_options.Logger
	a.cookie_provider = adapter_options.Cookies
	a.file_cache = adapter_options.Cache
	a.runtime_mu.Unlock()
	if adapter_options.Bus != nil {
		adapter_options.Bus.Publish(events.PlatformStatusChanged{Platform: PlatformID, Status: "available", Available: true})
	}
	return a, nil
}

// Stop releases runtime references owned by the adapter.
func (a *FeishuAdapter) Stop() {
	if a == nil {
		return
	}
	a.runtime_mu.Lock()
	a.logger = nil
	a.cookie_provider = nil
	a.file_cache = nil
	a.runtime_mu.Unlock()
}

// Fetch retrieves a Feishu document.
func (a *FeishuAdapter) Fetch(raw_url string) (any, error) {
	return a.FetchWithProgressContext(context.Background(), raw_url, adapter.FetchOptions{})
}

// FetchWithProgressContext retrieves a Feishu document with cancellation.
func (a *FeishuAdapter) FetchWithProgressContext(fetch_context context.Context, raw_url string, options adapter.FetchOptions) (any, error) {
	cookie_provider, file_cache, logger := a.runtime_values()
	if logger != nil {
		log_event := logger.Info().Str("component", "feishu_adapter").Str("url", strings.TrimSpace(raw_url))
		if request_id := strings.TrimSpace(options.RequestID); request_id != "" {
			log_event = log_event.Str("job_id", request_id)
		}
		log_event.Msg("feishu fetch started")
	}
	document, err := feishu.NewClient(cookie_provider, file_cache).FetchContext(fetch_context, raw_url)
	if err != nil {
		return nil, err
	}
	return document, nil
}

func (a *FeishuAdapter) runtime_values() (*cookies.Reader, *cache.CacheProvider, *zerolog.Logger) {
	if a == nil {
		return nil, nil, nil
	}
	a.runtime_mu.RLock()
	defer a.runtime_mu.RUnlock()
	return a.cookie_provider, a.file_cache, a.logger
}

func (a *FeishuAdapter) ToContent(data any) (*model.Content, error) {
	document, err := document_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return document_content(document), nil
}

func (a *FeishuAdapter) ToAccount(data any) (*model.Account, error) {
	document, err := document_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return document_account(document), nil
}

func (a *FeishuAdapter) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	document, err := document_from_fetch(data)
	if err != nil {
		return nil, err
	}
	content_id := PlatformID + ":" + document.Token
	return []adapter.ContentDetail{{
		Type: model.ContentTypeDocument,
		Key:  content_id,
		Data: &model.ContentDocument{Id: content_id, FileFormat: "html", WordCount: document.WordCount},
	}}, nil
}

func (a *FeishuAdapter) BuildBrowseHistory(json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	return nil, adapter.ErrBrowseHistoryNotSupported
}

func (a *FeishuAdapter) BuildDownloadTask(content_json json.RawMessage, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	document, err := document_from_json(content_json)
	if err != nil {
		return nil, err
	}
	return a.build_download_task(document, config_json)
}

// BuildDownloadTaskFromFetch builds a task without issuing another request.
func (a *FeishuAdapter) BuildDownloadTaskFromFetch(data any, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	document, err := document_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return a.build_download_task(document, config_json)
}

func (a *FeishuAdapter) build_download_task(document *feishu.Document, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	config_data := make(map[string]any)
	if config_text := strings.TrimSpace(string(config_json)); config_text != "" && config_text != "null" {
		if err := json.Unmarshal(config_json, &config_data); err != nil {
			return nil, fmt.Errorf("decode Feishu download config: %w", err)
		}
	}
	content := document_content(document)
	account := document_account(document)
	details, _ := a.ToContentDetails(document)
	task_name, _ := config_data["filename"].(string)
	if task_name = strings.TrimSpace(task_name); task_name == "" {
		task_name = document.Title
	}
	config_bytes, _ := json.Marshal(config_data)
	metadata_bytes, _ := json.Marshal(map[string]any{
		"platform":    PlatformID,
		"token":       document.Token,
		"tenant":      document.Tenant,
		"block_count": document.BlockCount,
		"asset_count": len(document.Assets),
	})
	content_id := content.Id
	result := &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &content_id,
			Name:         task_name,
			UniqueID:     document.Token + "_document",
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			SourceURL:    document.URL,
			ConfigJSON:   string(config_bytes),
			MetadataJSON: string(metadata_bytes),
		},
		Content:        content,
		Account:        account,
		ContentDetail:  details[0].Data,
		ContentDetails: details,
	}
	body := strings.TrimSpace(document.HTML)
	if body == "" {
		return nil, errors.New("Feishu document HTML is empty")
	}
	body += "\n"
	result.Resources = append(result.Resources, &adapter.ResourceInfo{
		Resource:      model.DownloadResource{ContentId: &content_id, Name: "document", Kind: "text/html", UniqueID: document.Token + "_body", Size: int64(len(body))},
		Endpoints:     []model.DownloadEndpoint{{Protocol: "inline", URL: body, Enabled: 1}},
		ContentAssets: []adapter.ContentAssetReference{{Kind: model.ContentAssetKindText, Role: model.ContentAssetRoleArticleBody, AssetKey: "body", Relation: model.DownloadResourceAssetRelationSource}},
	})
	for asset_index, asset := range document.Assets {
		resource_info, err := a.asset_resource(document, content_id, asset, asset_index)
		if err != nil {
			return nil, err
		}
		result.Resources = append(result.Resources, resource_info)
	}
	return result, nil
}

func (a *FeishuAdapter) asset_resource(document *feishu.Document, content_id string, asset feishu.Asset, asset_index int) (*adapter.ResourceInfo, error) {
	resource_name := strings.TrimSpace(asset.RelativePath)
	resource_name = strings.TrimSuffix(resource_name, filepath.Ext(resource_name))
	if resource_name == "" {
		resource_name = "files/" + asset.Token
		if strings.HasPrefix(asset.MIMEType, "image/") {
			resource_name = "images/" + asset.Token
		}
	}
	resource := model.DownloadResource{
		ContentId:  &content_id,
		Name:       resource_name,
		Kind:       first_non_empty(asset.MIMEType, "application/octet-stream"),
		UniqueID:   document.Token + "_asset_" + asset.Token,
		Size:       asset.Size,
		MergeOrder: asset_index + 1,
	}
	endpoint := model.DownloadEndpoint{Enabled: 1}
	if asset.LocalPath != "" {
		endpoint.Protocol = "file"
		endpoint.URL = asset.LocalPath
	} else {
		asset_url, err := url.Parse(strings.TrimSpace(asset.URL))
		if err != nil || asset_url.Scheme != "https" || asset_url.Hostname() == "" {
			return nil, fmt.Errorf("Feishu asset %s has no downloadable URL", asset.Token)
		}
		endpoint.Protocol = asset_url.Scheme
		endpoint.URL = asset_url.String()
		headers, _ := json.Marshal(map[string]string{"Referer": document.URL, "x-command": "stream.download"})
		endpoint.Headers = string(headers)
		endpoint.Cookies = strings.TrimSpace(asset.Cookies)
		cookie_provider, _, _ := a.runtime_values()
		if endpoint.Cookies == "" && cookie_provider != nil {
			cookie_header, cookie_err := cookie_provider.HeaderForURL(endpoint.URL)
			if cookie_err != nil && !errors.Is(cookie_err, cookies.ErrCookieNotFound) {
				return nil, fmt.Errorf("read Feishu asset cookies: %w", cookie_err)
			}
			endpoint.Cookies = cookie_header
		}
	}
	return &adapter.ResourceInfo{
		Resource:  resource,
		Endpoints: []model.DownloadEndpoint{endpoint},
		ContentAssets: []adapter.ContentAssetReference{{
			Kind:     asset_kind(asset.MIMEType),
			Role:     model.ContentAssetRoleAttachment,
			AssetKey: asset.Token,
			Relation: model.DownloadResourceAssetRelationSource,
		}},
	}, nil
}

func document_from_fetch(data any) (*feishu.Document, error) {
	switch value := data.(type) {
	case *feishu.Document:
		return validate_document(value)
	case feishu.Document:
		return validate_document(&value)
	case json.RawMessage:
		return document_from_json(value)
	case []byte:
		return document_from_json(value)
	case string:
		return document_from_json([]byte(value))
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("encode Feishu document: %w", err)
	}
	return document_from_json(encoded)
}

func document_from_json(content_json []byte) (*feishu.Document, error) {
	if len(strings.TrimSpace(string(content_json))) == 0 {
		return nil, errors.New("Feishu document is empty")
	}
	var document feishu.Document
	if err := json.Unmarshal(content_json, &document); err != nil {
		return nil, fmt.Errorf("decode Feishu document: %w", err)
	}
	return validate_document(&document)
}

func validate_document(document *feishu.Document) (*feishu.Document, error) {
	if document == nil || strings.TrimSpace(document.Token) == "" || strings.TrimSpace(document.URL) == "" {
		return nil, errors.New("Feishu document is missing its token or URL")
	}
	return document, nil
}

func document_content(document *feishu.Document) *model.Content {
	metadata, _ := json.Marshal(map[string]any{"tenant": document.Tenant, "block_count": document.BlockCount, "asset_count": len(document.Assets)})
	now := util.NowMillis()
	return &model.Content{
		Id:          PlatformID + ":" + document.Token,
		PlatformId:  PlatformID,
		Type:        model.ContentTypeDocument,
		ExternalId:  document.Token,
		Title:       document.Title,
		Description: truncate_text(document.Text, 280),
		URL:         document.URL,
		SourceURL:   document.URL,
		Metadata:    string(metadata),
		Timestamps:  model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
}

func document_account(document *feishu.Document) *model.Account {
	now := util.NowMillis()
	origin := "https://" + document.Tenant + "/"
	return &model.Account{
		Id:         PlatformID + ":" + document.Tenant,
		PlatformId: PlatformID,
		ExternalId: document.Tenant,
		Nickname:   document.Tenant,
		AvatarURL:  origin + "favicon.ico",
		ProfileURL: origin,
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
}

func asset_kind(mime_type string) string {
	switch {
	case strings.HasPrefix(mime_type, "image/"):
		return model.ContentAssetKindImage
	case strings.HasPrefix(mime_type, "audio/"):
		return model.ContentAssetKindAudio
	case strings.HasPrefix(mime_type, "video/"):
		return model.ContentAssetKindVideo
	case mime_type == "application/pdf", strings.Contains(mime_type, "document"), strings.Contains(mime_type, "sheet"):
		return model.ContentAssetKindDocument
	default:
		return model.ContentAssetKindBinary
	}
}

func truncate_text(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}

func first_non_empty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
