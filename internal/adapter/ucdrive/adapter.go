package ucdriveadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"mime"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/pkg/scraper/ucdrive"
)

const PlatformID = ucdrive.PlatformID

func init() {
	adapter.Register(NewUCDriveAdapter())
}

// UCDriveAdapter connects the UC Drive scraper to the shared adapter registry.
type UCDriveAdapter struct{}

var (
	_ adapter.PlatformAdapter             = (*UCDriveAdapter)(nil)
	_ adapter.ContextProgressFetchAdapter = (*UCDriveAdapter)(nil)
	_ adapter.FetchDownloadTaskBuilder    = (*UCDriveAdapter)(nil)
	_ adapter.Postprocessor               = (*UCDriveAdapter)(nil)
	_ adapter.RuntimeAdapter              = (*UCDriveAdapter)(nil)
	_ adapter.RuntimeHandle               = (*UCDriveAdapter)(nil)
	_ adapter.PlatformStatusDescriber     = (*UCDriveAdapter)(nil)
)

// NewUCDriveAdapter creates the stateless UC Drive adapter.
func NewUCDriveAdapter() *UCDriveAdapter { return &UCDriveAdapter{} }

func (a *UCDriveAdapter) PlatformID() string { return PlatformID }

func (a *UCDriveAdapter) PlatformStatuses() []adapter.PlatformStatusDescriptor {
	return []adapter.PlatformStatusDescriptor{{
		Platform: PlatformID,
		Key:      PlatformID,
		Name:     "UC网盘",
	}}
}

func (a *UCDriveAdapter) RegisterRuntime(adapter_options *adapter.AdapterOptions) (adapter.RuntimeHandle, error) {
	if adapter_options == nil {
		return nil, fmt.Errorf("ucdrive runtime dependencies are nil")
	}
	if adapter_options.Bus != nil {
		adapter_options.Bus.Publish(events.PlatformStatusChanged{
			Platform:  PlatformID,
			Key:       PlatformID,
			Name:      "UC网盘",
			Status:    "available",
			Available: true,
		})
	}
	return a, nil
}

func (a *UCDriveAdapter) Stop() {
}

func (a *UCDriveAdapter) Fetch(raw_url string) (any, error) {
	return a.FetchWithProgressContext(context.Background(), raw_url, adapter.FetchOptions{})
}

func (a *UCDriveAdapter) FetchWithProgressContext(fetch_context context.Context, raw_url string, _ adapter.FetchOptions) (any, error) {
	if strings.TrimSpace(raw_url) == "" {
		return nil, fmt.Errorf("UC 网盘 URL 不能为空")
	}
	return ucdrive.NewClient().FetchContext(fetch_context, raw_url)
}

func (a *UCDriveAdapter) ToContent(data any) (*model.Content, error) {
	share, err := share_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return to_content(share), nil
}

func (a *UCDriveAdapter) ToAccount(data any) (*model.Account, error) {
	share, err := share_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return to_account(share), nil
}

func (a *UCDriveAdapter) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	share, err := share_from_fetch(data)
	if err != nil {
		return nil, err
	}
	content := to_content(share)
	article := to_article(share, content.Id)
	return []adapter.ContentDetail{{
		Type:    model.ContentTypeWebpage,
		Key:     content.Id,
		Content: content,
		Data:    article,
	}}, nil
}

func (a *UCDriveAdapter) BuildDownloadTask(content_json json.RawMessage, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
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
	fetched_share, fetch_err := ucdrive.NewClient().Fetch(raw_url)
	if fetch_err != nil {
		return nil, fetch_err
	}
	return build_download_task(fetched_share, config_json)
}

func (a *UCDriveAdapter) BuildDownloadTaskFromFetch(data any, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	share, err := share_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return build_download_task(share, config_json)
}

func (a *UCDriveAdapter) BuildBrowseHistory(_ json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	return nil, adapter.ErrBrowseHistoryNotSupported
}

func share_from_fetch(data any) (*ucdrive.Share, error) {
	switch value := data.(type) {
	case *ucdrive.Share:
		return validate_share(value)
	case ucdrive.Share:
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
		return nil, fmt.Errorf("编码 UC 网盘抓取数据失败: %w", err)
	}
	return share_from_json(encoded)
}

func share_from_json(content_json []byte) (*ucdrive.Share, error) {
	if len(strings.TrimSpace(string(content_json))) == 0 {
		return nil, fmt.Errorf("UC 网盘抓取数据为空")
	}
	var share ucdrive.Share
	if err := json.Unmarshal(content_json, &share); err != nil {
		return nil, fmt.Errorf("解析 UC 网盘抓取数据失败: %w", err)
	}
	return validate_share(&share)
}

func validate_share(share *ucdrive.Share) (*ucdrive.Share, error) {
	if share == nil {
		return nil, fmt.Errorf("UC 网盘分享为空")
	}
	if strings.TrimSpace(share.PwdID) == "" {
		return nil, fmt.Errorf("UC 网盘分享缺少分享 ID")
	}
	return share, nil
}

func to_content(share *ucdrive.Share) *model.Content {
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
		Description: fmt.Sprintf("UC 网盘分享，包含 %d 个文件", file_count),
		URL:         share.URL,
		SourceURL:   share.URL,
		Metadata:    string(metadata_data),
		Timestamps:  model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
}

func to_account(share *ucdrive.Share) *model.Account {
	external_id := first_non_empty(share.Author, share.PwdID)
	now := time.Now().UnixMilli()
	return &model.Account{
		Id:         PlatformID + ":" + external_id,
		PlatformId: PlatformID,
		ExternalId: external_id,
		Nickname:   first_non_empty(share.Author, "UC网盘分享"),
		AvatarURL:  share.AuthorAvatarURL,
		ProfileURL: "https://drive.uc.cn/",
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
}

func to_article(share *ucdrive.Share, content_id string) *model.ContentArticle {
	html_text := render_file_tree(share.Files)
	plain_text := render_file_tree_text(share.Files)
	return &model.ContentArticle{
		Id:        content_id,
		Type:      model.ContentArticleTypeHTML,
		WordCount: utf8.RuneCountInString(plain_text),
		Text:      plain_text,
		HTML:      html_text,
		Markdown:  plain_text,
	}
}

func build_download_task(share *ucdrive.Share, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	if share == nil {
		return nil, fmt.Errorf("UC 网盘分享为空")
	}
	files := make([]ucdrive.File, 0)
	for _, file := range share.FlattenFiles() {
		if !file.IsDir {
			files = append(files, file)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("UC 网盘分享没有可下载文件")
	}
	for _, file := range files {
		if strings.TrimSpace(share.DownloadCookies) == "" || strings.TrimSpace(file.DownloadURL) == "" {
			if err := ucdrive.NewClient().FetchDownloadLinks(context.Background(), share); err != nil {
				return nil, err
			}
			files = files[:0]
			for _, refreshed_file := range share.FlattenFiles() {
				if !refreshed_file.IsDir {
					files = append(files, refreshed_file)
				}
			}
			break
		}
	}
	for _, file := range files {
		if strings.TrimSpace(file.DownloadURL) == "" {
			return nil, fmt.Errorf("UC 网盘文件缺少下载地址: %s", file.Name)
		}
	}
	config := make(map[string]any)
	if text := strings.TrimSpace(string(config_json)); text != "" && text != "null" {
		if err := json.Unmarshal(config_json, &config); err != nil {
			return nil, fmt.Errorf("解析 UC 网盘下载配置失败: %w", err)
		}
	}
	task_name := config_string(config, "filename")
	if task_name == "" {
		task_name = first_non_empty(share.Title, share.PwdID)
	}
	content := to_content(share)
	account := to_account(share)
	article := to_article(share, content.Id)
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
		ContentDetail:  article,
		ContentDetails: []adapter.ContentDetail{{Type: model.ContentTypeWebpage, Key: content.Id, Content: content, Data: article}},
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
		"Referer": "https://drive.uc.cn/",
	})
	return string(data)
}

func file_kind(file ucdrive.File) string {
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

func render_file_tree(files []ucdrive.File) string {
	var builder strings.Builder
	builder.WriteString(`<section class="ucdrive-file-tree"><h3>文件树</h3><ul>`)
	for _, file := range files {
		render_file_tree_item(&builder, file)
	}
	builder.WriteString(`</ul></section>`)
	return builder.String()
}

func render_file_tree_item(builder *strings.Builder, file ucdrive.File) {
	icon := "📄"
	class_name := "ucdrive-file"
	if file.IsDir {
		icon = "📁"
		class_name = "ucdrive-directory"
	}
	builder.WriteString(`<li><span class="`)
	builder.WriteString(class_name)
	builder.WriteString(`">`)
	builder.WriteString(icon)
	builder.WriteString(` `)
	builder.WriteString(html.EscapeString(first_non_empty(file.Name, file.Fid)))
	if !file.IsDir && file.Size > 0 {
		builder.WriteString(` <small>`)
		builder.WriteString(html.EscapeString(format_size(file.Size)))
		builder.WriteString(`</small>`)
	}
	builder.WriteString(`</span>`)
	if file.IsDir && len(file.Children) > 0 {
		builder.WriteString(`<ul>`)
		for _, child := range file.Children {
			render_file_tree_item(builder, child)
		}
		builder.WriteString(`</ul>`)
	}
	builder.WriteString(`</li>`)
}

func render_file_tree_text(files []ucdrive.File) string {
	var builder strings.Builder
	var walk func([]ucdrive.File, int)
	walk = func(entries []ucdrive.File, depth int) {
		for _, file := range entries {
			builder.WriteString(strings.Repeat("  ", depth))
			if file.IsDir {
				builder.WriteString("📁 ")
			} else {
				builder.WriteString("📄 ")
			}
			builder.WriteString(first_non_empty(file.Name, file.Fid))
			if !file.IsDir && file.Size > 0 {
				builder.WriteString(" (" + format_size(file.Size) + ")")
			}
			builder.WriteByte('\n')
			walk(file.Children, depth+1)
		}
	}
	walk(files, 0)
	return strings.TrimSpace(builder.String())
}

func count_files(files []ucdrive.File) int {
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
