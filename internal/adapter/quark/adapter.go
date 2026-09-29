package quarkadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/scraper/quark"
)

const PlatformID = quark.PlatformID

func init() {
	adapter.Register(NewQuarkAdapter())
}

// QuarkAdapter connects the Quark Drive scraper to the shared adapter registry.
type QuarkAdapter struct {
	cookie_reader *cookies.Reader
}

var (
	_ adapter.PlatformAdapter                   = (*QuarkAdapter)(nil)
	_ adapter.ContextProgressFetchAdapter       = (*QuarkAdapter)(nil)
	_ adapter.FetchDownloadTaskBuilder          = (*QuarkAdapter)(nil)
	_ adapter.FetchDownloadTaskResourcePreparer = (*QuarkAdapter)(nil)
	_ adapter.Postprocessor                     = (*QuarkAdapter)(nil)
	_ adapter.RuntimeAdapter                    = (*QuarkAdapter)(nil)
	_ adapter.RuntimeHandle                     = (*QuarkAdapter)(nil)
	_ adapter.PlatformStatusDescriber           = (*QuarkAdapter)(nil)
)

// NewQuarkAdapter creates the stateless Quark Drive adapter.
func NewQuarkAdapter() *QuarkAdapter { return &QuarkAdapter{} }

func (a *QuarkAdapter) PlatformID() string { return PlatformID }

func (a *QuarkAdapter) PlatformStatuses() []adapter.PlatformStatusDescriptor {
	return []adapter.PlatformStatusDescriptor{{
		Platform: PlatformID,
		Key:      PlatformID,
		Name:     "夸克网盘",
	}}
}

func (a *QuarkAdapter) RegisterRuntime(adapter_options *adapter.AdapterOptions) (adapter.RuntimeHandle, error) {
	if adapter_options == nil {
		return nil, fmt.Errorf("quark runtime dependencies are nil")
	}
	a.cookie_reader = adapter_options.Cookies
	if adapter_options.Bus != nil {
		adapter_options.Bus.Publish(events.PlatformStatusChanged{
			Platform:  PlatformID,
			Key:       PlatformID,
			Name:      "夸克网盘",
			Status:    "available",
			Available: true,
		})
	}
	return a, nil
}

func (a *QuarkAdapter) Stop() {
}

func (a *QuarkAdapter) Fetch(raw_url string) (any, error) {
	return a.FetchWithProgressContext(context.Background(), raw_url, adapter.FetchOptions{})
}

func (a *QuarkAdapter) FetchWithProgressContext(fetch_context context.Context, raw_url string, _ adapter.FetchOptions) (any, error) {
	if strings.TrimSpace(raw_url) == "" {
		return nil, fmt.Errorf("夸克网盘 URL 不能为空")
	}
	return quark.NewClient().FetchContext(fetch_context, raw_url)
}

func (a *QuarkAdapter) ToContent(data any) (*model.Content, error) {
	share, err := share_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return to_content(share), nil
}

func (a *QuarkAdapter) ToAccount(data any) (*model.Account, error) {
	share, err := share_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return to_account(share), nil
}

func (a *QuarkAdapter) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	share, err := share_from_fetch(data)
	if err != nil {
		return nil, err
	}
	content := to_content(share)
	return []adapter.ContentDetail{{
		Type:    model.ContentTypeCollection,
		Key:     content.Id,
		Content: content,
	}}, nil
}

func (a *QuarkAdapter) BuildDownloadTask(content_json json.RawMessage, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	share, err := share_from_json(content_json)
	if err == nil {
		return build_download_task(share, config_json)
	}
	var input struct {
		URL          string `json:"url"`
		SourceURL    string `json:"source_url"`
		RequestedURL string `json:"requested_url"`
	}
	if decode_err := json.Unmarshal(content_json, &input); decode_err != nil {
		return nil, err
	}
	raw_url := first_non_empty(input.RequestedURL, input.SourceURL, input.URL)
	if raw_url == "" {
		return nil, err
	}
	client := quark.NewClient()
	fetched_share, fetch_err := client.Fetch(raw_url)
	if fetch_err != nil {
		return nil, fetch_err
	}
	if err := client.FetchDownloadLinks(context.Background(), fetched_share); err != nil {
		return nil, err
	}
	return build_download_task(fetched_share, config_json)
}

func (a *QuarkAdapter) BuildDownloadTaskFromFetch(data any, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	share, err := share_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return build_download_task(share, config_json)
}

func (a *QuarkAdapter) PrepareDownloadTaskResources(data any, resource_indexes []int) (any, error) {
	share, err := share_from_fetch(data)
	if err != nil {
		return nil, err
	}
	files := share.DownloadableFiles()
	selected_files := files
	if len(resource_indexes) > 0 {
		selected_files = make([]*quark.File, 0, len(resource_indexes))
		for _, resource_index := range resource_indexes {
			if resource_index < 0 || resource_index >= len(files) {
				return nil, fmt.Errorf("夸克网盘下载资源序号 %d 超出范围", resource_index)
			}
			selected_files = append(selected_files, files[resource_index])
		}
	}
	client := quark.NewClient()
	client.SetCookieHeader(a.quark_cookie())
	return share, client.FetchDownloadLinksForFiles(
		context.Background(),
		share,
		selected_files,
	)
}

func (a *QuarkAdapter) quark_cookie() string {
	if a == nil || a.cookie_reader == nil {
		return ""
	}
	cookie_header, err := a.cookie_reader.HeaderForDomain("quark.cn")
	if err != nil {
		return ""
	}
	return cookie_header
}

func (a *QuarkAdapter) BuildBrowseHistory(_ json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	return nil, adapter.ErrBrowseHistoryNotSupported
}

func share_from_fetch(data any) (*quark.Share, error) {
	switch value := data.(type) {
	case *quark.Share:
		return validate_share(value)
	case quark.Share:
		return validate_share(&value)
	case json.RawMessage:
		return share_from_json(value)
	case []byte:
		return share_from_json(value)
	case string:
		return share_from_json([]byte(value))
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("编码夸克网盘抓取数据失败: %w", err)
	}
	return share_from_json(encoded)
}

func share_from_json(content_json []byte) (*quark.Share, error) {
	if len(strings.TrimSpace(string(content_json))) == 0 {
		return nil, fmt.Errorf("夸克网盘抓取数据为空")
	}
	var share quark.Share
	if err := json.Unmarshal(content_json, &share); err != nil {
		return nil, fmt.Errorf("解析夸克网盘抓取数据失败: %w", err)
	}
	return validate_share(&share)
}

func validate_share(share *quark.Share) (*quark.Share, error) {
	if share == nil {
		return nil, fmt.Errorf("夸克网盘分享为空")
	}
	if strings.TrimSpace(share.PwdID) == "" {
		return nil, fmt.Errorf("夸克网盘分享缺少分享 ID")
	}
	return share, nil
}

func to_content(share *quark.Share) *model.Content {
	content_id := PlatformID + ":" + share.PwdID
	title := first_non_empty(share.Title, share.PwdID)
	file_count := share.FileCount
	if file_count == 0 {
		file_count = count_files(share.Files)
	}
	metadata_data, _ := json.Marshal(map[string]any{
		"platform":   PlatformID,
		"pwd_id":     share.PwdID,
		"author":     share.Author,
		"expires_at": share.ExpiresAt,
		"file_count": file_count,
		"total_size": share.TotalSize,
		"source_url": share.URL,
	})
	now := time.Now().UnixMilli()
	return &model.Content{
		Id:          content_id,
		PlatformId:  PlatformID,
		Type:        model.ContentTypeCollection,
		Subtype:     model.ContentSubtypeFeed,
		ExternalId:  share.PwdID,
		Title:       title,
		Description: fmt.Sprintf("夸克网盘分享，包含 %d 个文件", file_count),
		URL:         share.URL,
		SourceURL:   share.URL,
		Metadata:    string(metadata_data),
		Timestamps:  model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
}

func to_account(share *quark.Share) *model.Account {
	external_id := first_non_empty(share.Author, share.PwdID)
	now := time.Now().UnixMilli()
	return &model.Account{
		Id:         PlatformID + ":" + external_id,
		PlatformId: PlatformID,
		ExternalId: external_id,
		Nickname:   first_non_empty(share.Author, "夸克网盘分享"),
		AvatarURL:  share.AuthorAvatarURL,
		ProfileURL: "https://pan.quark.cn/",
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
}

func build_download_task(share *quark.Share, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	if share == nil {
		return nil, fmt.Errorf("夸克网盘分享为空")
	}
	files := make([]quark.File, 0)
	for _, file := range share.FlattenFiles() {
		if !file.IsDir {
			files = append(files, file)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("夸克网盘分享没有可下载文件")
	}
	config := make(map[string]any)
	if text := strings.TrimSpace(string(config_json)); text != "" && text != "null" {
		if err := json.Unmarshal(config_json, &config); err != nil {
			return nil, fmt.Errorf("解析夸克网盘下载配置失败: %w", err)
		}
	}
	task_name := config_string(config, "filename")
	if task_name == "" {
		task_name = first_non_empty(share.Title, share.PwdID)
	}
	content := to_content(share)
	account := to_account(share)
	config_data, _ := json.Marshal(config)
	metadata_data, _ := json.Marshal(map[string]any{
		"platform":   PlatformID,
		"pwd_id":     share.PwdID,
		"source_url": share.URL,
		"file_count": len(files),
	})
	now := time.Now().UnixMilli()
	result := &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &content.Id,
			Name:         task_name,
			UniqueID:     content.ExternalId,
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			SourceURL:    share.URL,
			ConfigJSON:   string(config_data),
			MetadataJSON: string(metadata_data),
			Timestamps:   model.Timestamps{CreatedAt: now, UpdatedAt: now},
		},
		Content:        content,
		Account:        account,
		ContentDetails: []adapter.ContentDetail{{Type: model.ContentTypeCollection, Key: content.Id, Content: content}},
	}
	for _, file := range files {
		kind := file_kind(file)
		resource_name := extensionless_name(file.Path, kind)
		extra_data, _ := json.Marshal(map[string]string{
			"fid":        file.Fid,
			"path":       file.Path,
			"source_url": share.URL,
		})
		result.Resources = append(result.Resources, &adapter.ResourceInfo{
			Resource: model.DownloadResource{
				ContentId: &content.Id,
				Name:      resource_name,
				Kind:      kind,
				UniqueID:  content.ExternalId + ":" + file.Fid,
				Type:      model.ResourceTypeFile,
				Size:      file.Size,
				Extra:     string(extra_data),
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: "https",
				URL:      file.DownloadURL,
				Enabled:  1,
				Headers:  endpoint_headers_json(),
				Cookies:  share.DownloadCookies,
			}},
		})
	}
	return result, nil
}

func endpoint_headers_json() string {
	data, _ := json.Marshal(map[string]string{
		"Referer":    "https://pan.quark.cn/",
		"User-Agent": quark.QuarkUserAgent(),
	})
	return string(data)
}

func file_kind(file quark.File) string {
	kind := strings.TrimSpace(file.FormatType)
	if media_type, _, err := mime.ParseMediaType(kind); err == nil {
		kind = media_type
	}
	if kind == "" {
		kind = mime.TypeByExtension(filepath.Ext(file.Name))
	}
	if kind == "" {
		kind = "application/octet-stream"
	}
	return kind
}

func extensionless_name(name string, kind string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "file"
	}
	extension := filepath.Ext(name)
	if extension == "" {
		return name
	}
	extensions, _ := mime.ExtensionsByType(kind)
	for _, candidate := range extensions {
		if strings.EqualFold(extension, candidate) {
			return strings.TrimSuffix(name, extension)
		}
	}
	return name
}

func count_files(files []quark.File) int {
	count := 0
	for _, file := range files {
		if !file.IsDir {
			count++
		}
		count += count_files(file.Children)
	}
	return count
}

func format_size(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	value := float64(size)
	for _, unit := range units {
		value /= 1024
		if value < 1024 || unit == units[len(units)-1] {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}
	return fmt.Sprintf("%d B", size)
}

func config_string(config map[string]any, key string) string {
	value, _ := config[key].(string)
	return strings.TrimSpace(value)
}

func first_non_empty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
