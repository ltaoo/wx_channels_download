package singlefileadapter

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/scraper/singlefile"
	"wx_channel/pkg/util"
)

const PlatformID = singlefile.PlatformID

func init() { adapter.Register(NewSinglefileAdapter()) }

type SinglefileAdapter struct {
	runtime_mu      sync.RWMutex
	cookie_provider *cookies.Reader
}

var (
	_ adapter.PlatformAdapter             = (*SinglefileAdapter)(nil)
	_ adapter.ContextProgressFetchAdapter = (*SinglefileAdapter)(nil)
	_ adapter.FetchDownloadTaskBuilder    = (*SinglefileAdapter)(nil)
	_ adapter.RuntimeAdapter              = (*SinglefileAdapter)(nil)
	_ adapter.Postprocessor               = (*SinglefileAdapter)(nil)
)

func NewSinglefileAdapter() *SinglefileAdapter { return &SinglefileAdapter{} }

func (a *SinglefileAdapter) PlatformID() string { return PlatformID }

func (a *SinglefileAdapter) PlatformStatuses() []adapter.PlatformStatusDescriptor {
	return []adapter.PlatformStatusDescriptor{{Platform: PlatformID, Key: PlatformID, Name: "网页"}}
}

func (a *SinglefileAdapter) RegisterRuntime(options *adapter.AdapterOptions) (adapter.RuntimeHandle, error) {
	if options == nil {
		return nil, fmt.Errorf("singlefile: runtime dependencies are nil")
	}
	a.runtime_mu.Lock()
	a.cookie_provider = options.Cookies
	a.runtime_mu.Unlock()
	if options.Bus != nil {
		options.Bus.Publish(events.PlatformStatusChanged{Platform: PlatformID, Status: "available", Available: true})
	}
	return a, nil
}

func (a *SinglefileAdapter) Stop() {
	a.runtime_mu.Lock()
	a.cookie_provider = nil
	a.runtime_mu.Unlock()
}

func (a *SinglefileAdapter) runtime_cookie_provider() *cookies.Reader {
	a.runtime_mu.RLock()
	defer a.runtime_mu.RUnlock()
	return a.cookie_provider
}

func (a *SinglefileAdapter) Fetch(raw_url string) (any, error) {
	return a.FetchWithProgressContext(context.Background(), raw_url, adapter.FetchOptions{})
}

func (a *SinglefileAdapter) FetchWithProgressContext(ctx context.Context, raw_url string, _ adapter.FetchOptions) (any, error) {
	return singlefile.NewClient(a.runtime_cookie_provider()).FetchContext(ctx, raw_url)
}

func (a *SinglefileAdapter) ToContent(data any) (*model.Content, error) {
	page, err := page_from_fetch(data)
	if err != nil {
		return nil, err
	}
	page_url, _ := singlefile.ParseURL(page.URL)
	page_url.Fragment = ""
	external_id := fmt.Sprintf("%x", sha256.Sum256([]byte(page_url.String())))
	source_url := page.RequestedURL
	if source_url == "" {
		source_url = page.URL
	}
	title := strings.TrimSpace(page.Title)
	if title == "" {
		title = page_url.Hostname()
	}
	now := util.NowMillis()
	return &model.Content{
		Id: PlatformID + ":" + external_id, PlatformId: PlatformID,
		ExternalId: external_id, Type: model.ContentTypeWebpage, Title: title,
		URL: page.URL, SourceURL: source_url,
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}, nil
}

func (a *SinglefileAdapter) ToAccount(data any) (*model.Account, error) {
	page, err := page_from_fetch(data)
	if err != nil {
		return nil, err
	}
	page_url, _ := singlefile.ParseURL(page.URL)
	domain := strings.ToLower(page_url.Hostname())
	now := util.NowMillis()
	return &model.Account{
		Id: PlatformID + ":" + domain, PlatformId: PlatformID, ExternalId: domain,
		Nickname: domain, ProfileURL: page_url.Scheme + "://" + page_url.Host + "/",
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}, nil
}

func (a *SinglefileAdapter) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	page, err := page_from_fetch(data)
	if err != nil {
		return nil, err
	}
	content, err := a.ToContent(page)
	if err != nil {
		return nil, err
	}
	return []adapter.ContentDetail{{Type: content.Type, Key: content.Id, Content: content,
		Data: &model.ContentArticle{Id: content.Id, Type: model.ContentArticleTypeHTML, HTML: page.HTML},
	}}, nil
}

func (a *SinglefileAdapter) BuildBrowseHistory(json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	return nil, adapter.ErrBrowseHistoryNotSupported
}

func (a *SinglefileAdapter) BuildDownloadTask(content_json json.RawMessage, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	page, err := page_from_fetch(content_json)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(page.HTML) == "" {
		page, err = singlefile.NewClient(a.runtime_cookie_provider()).Fetch(page.URL)
		if err != nil {
			return nil, err
		}
	}
	return a.BuildDownloadTaskFromFetch(page, config_json)
}

func (a *SinglefileAdapter) BuildDownloadTaskFromFetch(data any, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	page, err := page_from_fetch(data)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(page.HTML) == "" {
		return nil, fmt.Errorf("singlefile: HTML is empty")
	}
	config_data := make(map[string]any)
	if strings.TrimSpace(string(config_json)) != "" {
		if err := json.Unmarshal(config_json, &config_data); err != nil {
			return nil, fmt.Errorf("singlefile download config: %w", err)
		}
	}
	content, err := a.ToContent(page)
	if err != nil {
		return nil, err
	}
	account, err := a.ToAccount(page)
	if err != nil {
		return nil, err
	}
	details, err := a.ToContentDetails(page)
	if err != nil {
		return nil, err
	}
	task_name, _ := config_data["filename"].(string)
	if task_name = strings.TrimSpace(task_name); task_name == "" {
		task_name = content.Title
	}
	config_bytes, _ := json.Marshal(config_data)
	extra, _ := json.Marshal(map[string]string{"singlefile_postprocess": "inline", "source_url": page.URL})
	return &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId: &content.Id, Name: task_name, UniqueID: content.ExternalId + "_singlefile",
			PlatformId: PlatformID, Status: model.TaskStatusWaiting, SourceURL: content.SourceURL,
			ConfigJSON: string(config_bytes),
		},
		Content: content, Account: account, ContentDetail: details[0].Data, ContentDetails: details,
		Resources: []*adapter.ResourceInfo{{
			Resource: model.DownloadResource{ContentId: &content.Id, Name: task_name, Kind: "text/html",
				UniqueID: content.ExternalId + "_html", Size: int64(len(page.HTML)), Extra: string(extra)},
			Endpoints: []model.DownloadEndpoint{{Protocol: "inline", URL: page.HTML, Enabled: 1}},
			ContentAssets: []adapter.ContentAssetReference{{Kind: model.ContentAssetKindText,
				Role: model.ContentAssetRoleArticleBody, AssetKey: "body", Relation: model.DownloadResourceAssetRelationSource}},
		}},
	}, nil
}

func page_from_fetch(data any) (*singlefile.Page, error) {
	var page singlefile.Page
	var encoded []byte
	var err error
	switch value := data.(type) {
	case *singlefile.Page:
		if value == nil {
			return nil, fmt.Errorf("singlefile: page is nil")
		}
		page = *value
	case singlefile.Page:
		page = value
	case json.RawMessage:
		encoded = value
	case []byte:
		encoded = value
	case string:
		encoded = []byte(value)
	default:
		encoded, err = json.Marshal(data)
	}
	if err != nil {
		return nil, err
	}
	if encoded != nil {
		if err := json.Unmarshal(encoded, &page); err != nil {
			return nil, fmt.Errorf("singlefile page: %w", err)
		}
	}
	if _, err := singlefile.ParseURL(page.URL); err != nil {
		return nil, err
	}
	return &page, nil
}
