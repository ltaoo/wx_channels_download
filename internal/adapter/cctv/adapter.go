package cctvadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/pkg/scraper/cctv"
)

func init() {
	adapter.Register(NewCCTVAdapter())
}

// CCTVAdapter connects the CCTV scraper to the shared adapter registry.
type CCTVAdapter struct{}

var (
	_ adapter.PlatformAdapter             = (*CCTVAdapter)(nil)
	_ adapter.ProgressFetchAdapter        = (*CCTVAdapter)(nil)
	_ adapter.ContextProgressFetchAdapter = (*CCTVAdapter)(nil)
	_ adapter.FetchDownloadTaskBuilder    = (*CCTVAdapter)(nil)
	_ adapter.RuntimeAdapter              = (*CCTVAdapter)(nil)
	_ adapter.RuntimeHandle               = (*CCTVAdapter)(nil)
)

// NewCCTVAdapter creates a CCTV platform adapter.
func NewCCTVAdapter() *CCTVAdapter {
	return &CCTVAdapter{}
}

func (a *CCTVAdapter) PlatformID() string { return PlatformID }

// RegisterRuntime marks the stateless CCTV scraper as available.
func (a *CCTVAdapter) RegisterRuntime(adapter_options *adapter.AdapterOptions) (adapter.RuntimeHandle, error) {
	if adapter_options == nil {
		return nil, fmt.Errorf("cctv runtime dependencies are nil")
	}
	if adapter_options.Bus != nil {
		adapter_options.Bus.Publish(events.PlatformStatusChanged{
			Platform:  PlatformID,
			Status:    "available",
			Available: true,
		})
	}
	return a, nil
}

// Stop releases the stateless CCTV runtime.
func (a *CCTVAdapter) Stop() {}

// Fetch retrieves one CCTV video page and its VDN metadata.
func (a *CCTVAdapter) Fetch(raw_url string) (any, error) {
	return a.FetchWithProgressContext(context.Background(), raw_url, adapter.FetchOptions{})
}

// FetchWithProgress retrieves CCTV metadata. CCTV currently completes in one
// request stage, so request_id is accepted for interface compatibility.
func (a *CCTVAdapter) FetchWithProgress(raw_url string, request_id string) (any, error) {
	return a.FetchWithProgressContext(context.Background(), raw_url, adapter.FetchOptions{
		RequestID: strings.TrimSpace(request_id),
	})
}

// FetchWithProgressContext retrieves CCTV metadata with cancellation support.
func (a *CCTVAdapter) FetchWithProgressContext(fetch_context context.Context, raw_url string, _ adapter.FetchOptions) (any, error) {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return nil, fmt.Errorf("CCTV URL 不能为空")
	}
	return cctv.NewClient().FetchContext(fetch_context, raw_url)
}

func (a *CCTVAdapter) ToContent(data any) (*model.Content, error) {
	result, err := cctv_result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return ToContent(result)
}

func (a *CCTVAdapter) ToAccount(data any) (*model.Account, error) {
	result, err := cctv_result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return ToAccount(result)
}

func (a *CCTVAdapter) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	result, err := cctv_result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return ToContentDetails(result)
}

// BuildDownloadTask builds a CCTV video task from either a FetchResult or a
// small JSON object containing url/page_url.
func (a *CCTVAdapter) BuildDownloadTask(content_json json.RawMessage, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	result, err := cctv_result_from_json(content_json)
	if err == nil {
		return BuildDownloadTask(result, config_json)
	}

	var input cctv_content_json
	if decode_err := json.Unmarshal(content_json, &input); decode_err != nil {
		return nil, fmt.Errorf("解析 CCTV 下载数据失败: %w", decode_err)
	}
	raw_url := first_non_empty(input.URL, input.PageURL)
	if raw_url == "" {
		return nil, err
	}
	result, err = cctv.GetVideoInfo(raw_url)
	if err != nil {
		return nil, err
	}
	return BuildDownloadTask(result, config_json)
}

// BuildDownloadTaskFromFetch builds a download preview without another CCTV
// request.
func (a *CCTVAdapter) BuildDownloadTaskFromFetch(data any, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	result, err := cctv_result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return BuildDownloadTask(result, config_json)
}

func (a *CCTVAdapter) BuildBrowseHistory(_ json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	return nil, adapter.ErrBrowseHistoryNotSupported
}

type cctv_content_json struct {
	URL     string `json:"url"`
	PageURL string `json:"page_url"`
}

func cctv_result_from_fetch(data any) (*cctv.FetchResult, error) {
	switch value := data.(type) {
	case *cctv.FetchResult:
		return validate_result(value)
	case cctv.FetchResult:
		return validate_result(&value)
	case json.RawMessage:
		return cctv_result_from_json(value)
	case []byte:
		return cctv_result_from_json(value)
	case string:
		return cctv_result_from_json([]byte(value))
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("编码 CCTV 抓取数据失败: %w", err)
	}
	return cctv_result_from_json(encoded)
}

func cctv_result_from_json(content_json []byte) (*cctv.FetchResult, error) {
	if len(strings.TrimSpace(string(content_json))) == 0 {
		return nil, fmt.Errorf("CCTV 抓取数据为空")
	}
	var result cctv.FetchResult
	if err := json.Unmarshal(content_json, &result); err != nil {
		return nil, fmt.Errorf("解析 CCTV 抓取数据失败: %w", err)
	}
	return validate_result(&result)
}
