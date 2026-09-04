// Package feishu fetches Feishu docx documents and their downloadable assets.
package feishu

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"

	"wx_channel/pkg/cache"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/minib"
)

const (
	PlatformID              = "feishu"
	default_request_timeout = 30 * time.Second
	max_cursor_count        = 1000
	image_batch_size        = 100
	file_info_workers       = 6
)

var (
	document_path_pattern = regexp.MustCompile(`^/docx/([A-Za-z0-9]+)$`)
	bootstrap_pattern     = regexp.MustCompile(`\bwindow\.(?:DATA|SERVER_DATA)\s*=`)
)

// Asset is one image or attached file discovered in a Feishu document.
type Asset struct {
	Token        string `json:"token"`
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	MIMEType     string `json:"mime_type,omitempty"`
	Size         int64  `json:"size,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	URL          string `json:"url,omitempty"`
	LocalPath    string `json:"local_path,omitempty"`
	RelativePath string `json:"relative_path,omitempty"`
	Cookies      string `json:"cookies,omitempty"`
}

// Document is the normalized result of one Feishu docx fetch.
type Document struct {
	Token      string  `json:"token"`
	URL        string  `json:"url"`
	Tenant     string  `json:"tenant"`
	Title      string  `json:"title"`
	Text       string  `json:"text"`
	HTML       string  `json:"html"`
	WordCount  int     `json:"word_count"`
	BlockCount int     `json:"block_count"`
	Assets     []Asset `json:"assets"`
	root_id    string
	blocks     map[string]block_data
}

// Client fetches documents with persisted Feishu cookies and caches decrypted images.
type Client struct {
	cookie_provider *cookies.Reader
	file_cache      *cache.CacheProvider
	timeout         time.Duration
}

type client_vars_response struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Msg     string           `json:"msg"`
	Data    client_vars_data `json:"data"`
}

type client_vars_data struct {
	ID            string                     `json:"id"`
	BlockMap      map[string]json.RawMessage `json:"block_map"`
	BlockSequence []string                   `json:"block_sequence"`
	NextCursors   []string                   `json:"next_cursors"`
}

type bootstrap_export struct {
	WindowData struct {
		ClientVars client_vars_response `json:"clientVars"`
		Meta       struct {
			Title string `json:"title"`
		} `json:"meta"`
	} `json:"window_data"`
}

type block_data struct {
	Type         string                     `json:"type"`
	Children     []string                   `json:"children"`
	Done         bool                       `json:"done"`
	Hidden       bool                       `json:"hidden"`
	Folded       bool                       `json:"folded"`
	Align        string                     `json:"align"`
	Language     string                     `json:"language"`
	EmojiID      string                     `json:"emoji_id"`
	WidthRatio   float64                    `json:"width_ratio"`
	RowsID       []string                   `json:"rows_id"`
	ColumnsID    []string                   `json:"columns_id"`
	CellSet      map[string]table_cell_data `json:"cell_set"`
	ColumnSet    map[string]table_column    `json:"column_set"`
	HeaderRow    bool                       `json:"header_row"`
	HeaderColumn bool                       `json:"header_column"`
	Text         rich_text_data             `json:"text"`
	Image        asset_data                 `json:"image"`
	File         asset_data                 `json:"file"`
	Bookmark     bookmark_data              `json:"bookmark"`
}

type asset_data struct {
	Token    string  `json:"token"`
	MIMEType string  `json:"mimeType"`
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
}

type table_cell_data struct {
	BlockID   string `json:"block_id"`
	MergeInfo struct {
		RowSpan int `json:"row_span"`
		ColSpan int `json:"col_span"`
	} `json:"merge_info"`
}

type table_column struct {
	ColumnWidth float64 `json:"column_width"`
}

type bookmark_data struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

type image_request struct {
	FileToken string `json:"file_token"`
	Policy    string `json:"policy"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

type image_response struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Msg     string       `json:"msg"`
	Data    []image_item `json:"data"`
}

type image_item struct {
	FileToken  string `json:"file_token"`
	Permitted  bool   `json:"permitted"`
	URL        string `json:"url"`
	CipherType any    `json:"cipher_type"`
	Secret     string `json:"secret"`
	Nonce      string `json:"nonce"`
}

type file_info_response struct {
	Code int `json:"code"`
	Data struct {
		DataVersion string `json:"data_version"`
		PreviewMeta struct {
			Data map[string]file_preview `json:"data"`
		} `json:"preview_meta"`
	} `json:"data"`
}

type file_preview struct {
	Status  int `json:"status"`
	Content struct {
		TranscodeURLs map[string]string `json:"transcode_urls"`
	} `json:"content"`
}

// NewClient creates a Feishu document client.
func NewClient(cookie_provider *cookies.Reader, file_cache *cache.CacheProvider) *Client {
	return &Client{
		cookie_provider: cookie_provider,
		file_cache:      file_cache,
		timeout:         default_request_timeout,
	}
}

// Fetch retrieves one Feishu document.
func (c *Client) Fetch(raw_url string) (*Document, error) {
	return c.FetchContext(context.Background(), raw_url)
}

// FetchContext retrieves one Feishu document with cancellation support.
func (c *Client) FetchContext(fetch_context context.Context, raw_url string) (*Document, error) {
	if c == nil {
		return nil, errors.New("feishu client is not initialized")
	}
	if fetch_context == nil {
		fetch_context = context.Background()
	}
	document_url, token, tenant, err := parse_document_url(raw_url)
	if err != nil {
		return nil, err
	}
	browser, err := minib.NewMiniBrowser(c.timeout, c.cookie_provider)
	if err != nil {
		return nil, fmt.Errorf("initialize Feishu browser: %w", err)
	}
	defer browser.Close()

	page, err := browser.Navigate(fetch_context, document_url, http.Header{
		"Accept-Language": []string{"zh-CN,zh;q=0.9,en;q=0.8"},
	}, minib.NavigateOptions{
		DisableSubresources: true,
		DisableCSS:          true,
		DisableImages:       true,
		DisableMedia:        true,
		DisableJavaScript:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("fetch Feishu document page: %w", err)
	}
	if page.StatusCode < http.StatusOK || page.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Feishu document page returned HTTP %d", page.StatusCode)
	}
	if page_url, parse_err := url.Parse(page.URL); parse_err == nil && page_url.Hostname() != tenant {
		return nil, fmt.Errorf("Feishu document redirected to %s and is not publicly readable", page_url.Hostname())
	}
	bootstrap, err := parse_bootstrap(fetch_context, browser, page.HTML)
	if err != nil {
		return nil, err
	}
	if err := validate_client_vars(bootstrap.WindowData.ClientVars, "initial clientVars"); err != nil {
		return nil, err
	}
	pages, err := c.fetch_client_vars_pages(fetch_context, browser, document_url, token, bootstrap.WindowData.ClientVars.Data)
	if err != nil {
		return nil, err
	}
	document, err := build_document(document_url, token, tenant, bootstrap, pages)
	if err != nil {
		return nil, err
	}
	c.resolve_document_files(fetch_context, browser, document)
	if err := c.cache_document_images(fetch_context, browser, document); err != nil {
		return nil, err
	}
	document.HTML = render_document_html(document)
	if strings.TrimSpace(document.HTML) == "" {
		return nil, errors.New("render Feishu document HTML")
	}
	return document, nil
}

func parse_document_url(raw_url string) (string, string, string, error) {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil || parsed_url.Scheme != "https" || parsed_url.Hostname() == "" {
		return "", "", "", fmt.Errorf("Feishu URL must be https://<tenant>.feishu.cn/docx/<token>")
	}
	hostname := strings.ToLower(strings.TrimSuffix(parsed_url.Hostname(), "."))
	if hostname != "feishu.cn" && !strings.HasSuffix(hostname, ".feishu.cn") {
		return "", "", "", fmt.Errorf("Feishu URL must be https://<tenant>.feishu.cn/docx/<token>")
	}
	match := document_path_pattern.FindStringSubmatch(strings.TrimSuffix(parsed_url.EscapedPath(), "/"))
	if len(match) != 2 {
		return "", "", "", fmt.Errorf("Feishu URL must be https://<tenant>.feishu.cn/docx/<token>")
	}
	parsed_url.RawQuery = ""
	parsed_url.Fragment = ""
	parsed_url.Path = strings.TrimSuffix(parsed_url.Path, "/")
	parsed_url.RawPath = ""
	return parsed_url.String(), match[1], hostname, nil
}

func parse_bootstrap(fetch_context context.Context, browser *minib.MiniBrowser, html_text string) (*bootstrap_export, error) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(html_text))
	if err != nil {
		return nil, fmt.Errorf("parse Feishu document HTML: %w", err)
	}
	scripts := make([]string, 0, 2)
	document.Find("script").Each(func(_ int, script *goquery.Selection) {
		source := script.Text()
		if bootstrap_pattern.MatchString(source) {
			scripts = append(scripts, source)
		}
	})
	if len(scripts) == 0 {
		return nil, errors.New("Feishu document page contains no readable bootstrap data")
	}
	expression := "(function(){var window={globalConfig:{}};\n" + strings.Join(scripts, "\n") + "\n;return JSON.stringify({window_data:window.DATA,server_data:window.SERVER_DATA})})()"
	value, err := browser.ExecuteJS(fetch_context, expression)
	if err != nil {
		return nil, fmt.Errorf("evaluate Feishu bootstrap data: %w", err)
	}
	var bootstrap bootstrap_export
	if err := json.Unmarshal([]byte(value.String()), &bootstrap); err != nil {
		return nil, fmt.Errorf("decode Feishu bootstrap data: %w", err)
	}
	if bootstrap.WindowData.ClientVars.Data.ID == "" && bootstrap.WindowData.ClientVars.Code == 0 {
		return nil, errors.New("Feishu bootstrap data is missing clientVars")
	}
	return &bootstrap, nil
}

func (c *Client) fetch_client_vars_pages(fetch_context context.Context, browser *minib.MiniBrowser, document_url string, token string, initial client_vars_data) ([]client_vars_response, error) {
	parsed_url, _ := url.Parse(document_url)
	page_url := &url.URL{Scheme: parsed_url.Scheme, Host: parsed_url.Host, Path: "/space/api/docx/pages/client_vars"}
	limit := len(initial.BlockSequence)
	if limit == 0 {
		limit = 100
	}
	queue := append([]string(nil), initial.NextCursors...)
	seen := make(map[string]bool, len(queue))
	pages := make([]client_vars_response, 0, len(queue))
	for len(queue) > 0 {
		if len(seen) >= max_cursor_count {
			return nil, fmt.Errorf("Feishu document exceeded %d pagination cursors", max_cursor_count)
		}
		cursor := strings.TrimSpace(queue[0])
		queue = queue[1:]
		if cursor == "" || seen[cursor] {
			continue
		}
		seen[cursor] = true
		query := page_url.Query()
		query.Set("id", token)
		query.Set("mode", "7")
		query.Set("limit", strconv.Itoa(limit))
		query.Set("cursor", cursor)
		page_url.RawQuery = query.Encode()
		headers := http.Header{
			"Accept":  []string{"application/json, text/plain, */*"},
			"Referer": []string{document_url},
		}
		response, err := browser.Get(fetch_context, page_url.String(), headers)
		if err != nil {
			return nil, fmt.Errorf("fetch Feishu clientVars page: %w", err)
		}
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("Feishu clientVars page returned HTTP %d", response.StatusCode)
		}
		var page client_vars_response
		if err := json.Unmarshal(response.Body, &page); err != nil {
			return nil, fmt.Errorf("decode Feishu clientVars page: %w", err)
		}
		if err := validate_client_vars(page, "paginated clientVars"); err != nil {
			return nil, err
		}
		pages = append(pages, page)
		queue = append(queue, page.Data.NextCursors...)
	}
	return pages, nil
}

func validate_client_vars(response client_vars_response, name string) error {
	if response.Code == 0 {
		return nil
	}
	message := strings.TrimSpace(response.Message)
	if message == "" {
		message = strings.TrimSpace(response.Msg)
	}
	if message == "" {
		message = "unknown error"
	}
	return fmt.Errorf("Feishu %s failed with code %d: %s", name, response.Code, message)
}

func build_document(document_url string, token string, tenant string, bootstrap *bootstrap_export, pages []client_vars_response) (*Document, error) {
	data_pages := []client_vars_data{bootstrap.WindowData.ClientVars.Data}
	for _, page := range pages {
		data_pages = append(data_pages, page.Data)
	}
	blocks := make(map[string]block_data)
	order := make([]string, 0)
	ordered := make(map[string]bool)
	for _, data := range data_pages {
		for block_id, raw_block := range data.BlockMap {
			block, err := decode_block(raw_block)
			if err != nil {
				return nil, fmt.Errorf("decode Feishu block %s: %w", block_id, err)
			}
			blocks[block_id] = block
		}
		for _, block_id := range data.BlockSequence {
			if !ordered[block_id] {
				order = append(order, block_id)
				ordered[block_id] = true
			}
		}
	}
	missing_ids := make([]string, 0)
	for block_id := range blocks {
		if !ordered[block_id] {
			missing_ids = append(missing_ids, block_id)
		}
	}
	sort.Strings(missing_ids)
	order = append(order, missing_ids...)

	title := strings.TrimSpace(bootstrap.WindowData.Meta.Title)
	root_id := bootstrap.WindowData.ClientVars.Data.ID
	if root, exists := blocks[root_id]; exists {
		if root_title := block_text(root); root_title != "" {
			title = root_title
		}
	}
	if title == "" {
		title = "Feishu document"
	}

	plain_lines := make([]string, 0)
	assets := make([]Asset, 0)
	seen_assets := make(map[string]bool)
	for _, block_id := range order {
		block, exists := blocks[block_id]
		if !exists {
			continue
		}
		text := block_text(block)
		if block.Type != "page" && text != "" {
			plain_lines = append(plain_lines, text)
		}
		asset, exists := block_asset(block)
		if exists && !seen_assets[asset.Token] {
			assets = append(assets, asset)
			seen_assets[asset.Token] = true
		}
	}
	plain_text := strings.TrimSpace(strings.Join(plain_lines, "\n"))
	return &Document{
		Token:      token,
		URL:        document_url,
		Tenant:     tenant,
		Title:      title,
		Text:       plain_text,
		WordCount:  word_count(plain_text),
		BlockCount: len(blocks),
		Assets:     assets,
		root_id:    root_id,
		blocks:     blocks,
	}, nil
}

func decode_block(raw_block json.RawMessage) (block_data, error) {
	var wrapper struct {
		Data     json.RawMessage `json:"data"`
		Snapshot json.RawMessage `json:"snapshot"`
	}
	if err := json.Unmarshal(raw_block, &wrapper); err != nil {
		return block_data{}, err
	}
	target := raw_block
	if len(wrapper.Data) > 0 && string(wrapper.Data) != "null" {
		target = wrapper.Data
	} else if len(wrapper.Snapshot) > 0 && string(wrapper.Snapshot) != "null" {
		target = wrapper.Snapshot
	}
	var block block_data
	if err := json.Unmarshal(target, &block); err != nil {
		return block_data{}, err
	}
	return block, nil
}

func block_text(block block_data) string {
	texts := block.Text.InitialAttributedTexts.Text
	if len(texts) == 0 {
		return ""
	}
	keys := ordered_text_keys(texts)
	var text_builder strings.Builder
	for _, key := range keys {
		text_builder.WriteString(texts[key])
	}
	return strings.TrimSpace(text_builder.String())
}

func block_asset(block block_data) (Asset, bool) {
	asset := block.File
	kind := "file"
	if block.Type == "image" {
		asset = block.Image
		kind = "image"
	} else if block.Type != "file" {
		return Asset{}, false
	}
	asset.Token = strings.TrimSpace(asset.Token)
	if !valid_asset_token(asset.Token) {
		return Asset{}, false
	}
	name := strings.TrimSpace(asset.Name)
	if name == "" {
		name = kind + "_" + asset.Token
	}
	result := Asset{
		Token:    asset.Token,
		Kind:     kind,
		Name:     name,
		MIMEType: strings.ToLower(strings.TrimSpace(asset.MIMEType)),
		Size:     asset.Size,
		Width:    int(asset.Width),
		Height:   int(asset.Height),
	}
	if kind == "file" {
		result.URL = "https://internal-api-drive-stream.feishu.cn/space/api/box/stream/download/all/" + url.PathEscape(asset.Token) + "/?mount_point=docx_file&mount_node_token=" + url.QueryEscape(asset.Token)
		result.RelativePath = asset_relative_path(kind, asset.Token, name, result.MIMEType)
	}
	return result, true
}

func (c *Client) resolve_document_files(fetch_context context.Context, browser *minib.MiniBrowser, document *Document) {
	if document == nil || browser == nil {
		return
	}
	parsed_url, err := url.Parse(document.URL)
	if err != nil {
		return
	}
	origin := parsed_url.Scheme + "://" + parsed_url.Host
	headers := http.Header{
		"Accept":       []string{"application/json, text/plain, */*"},
		"Content-Type": []string{"application/json;charset=UTF-8"},
		"Origin":       []string{origin},
		"Referer":      []string{document.URL},
	}
	if csrf_token := browser_cookie(browser, document.URL, "swp_csrf_token"); csrf_token != "" {
		headers.Set("x-csrftoken", csrf_token)
	}

	indexes := make(chan int)
	worker_count := file_info_workers
	if len(document.Assets) < worker_count {
		worker_count = len(document.Assets)
	}
	var workers sync.WaitGroup
	workers.Add(worker_count)
	for worker_index := 0; worker_index < worker_count; worker_index++ {
		go func() {
			defer workers.Done()
			for asset_index := range indexes {
				asset := &document.Assets[asset_index]
				if preview_url := resolve_file_preview(fetch_context, browser, origin, headers, *asset); preview_url != "" {
					asset.URL = preview_url
					if strings.HasPrefix(asset.MIMEType, "video/") {
						asset.MIMEType = "video/mp4"
						asset.RelativePath = asset_relative_path("file", asset.Token, "", asset.MIMEType)
					}
				}
				asset.Cookies = browser_cookie_header(browser, asset.URL)
			}
		}()
	}
	for asset_index := range document.Assets {
		asset := &document.Assets[asset_index]
		if asset.Kind != "file" {
			continue
		}
		asset.Cookies = browser_cookie_header(browser, asset.URL)
		if asset.MIMEType == "application/pdf" || strings.HasPrefix(asset.MIMEType, "video/") || strings.HasPrefix(asset.MIMEType, "audio/") {
			select {
			case indexes <- asset_index:
			case <-fetch_context.Done():
				close(indexes)
				workers.Wait()
				return
			}
		}
	}
	close(indexes)
	workers.Wait()
}

func resolve_file_preview(fetch_context context.Context, browser *minib.MiniBrowser, origin string, headers http.Header, asset Asset) string {
	body, _ := json.Marshal(map[string]any{
		"file_token":       asset.Token,
		"mount_point":      "docx_file",
		"mount_node_token": asset.Token,
		"option_params":    []string{"preview_meta", "check_cipher"},
	})
	response, err := browser.Request(fetch_context, http.MethodPost, origin+"/space/api/box/file/info/", bytes.NewReader(body), headers)
	if err != nil || response.StatusCode != http.StatusOK {
		return ""
	}
	var info file_info_response
	if json.Unmarshal(response.Body, &info) != nil || info.Code != 0 {
		return ""
	}
	return file_preview_url(asset, info)
}

func file_preview_url(asset Asset, info file_info_response) string {
	if asset.MIMEType == "application/pdf" {
		if preview, exists := info.Data.PreviewMeta.Data["9"]; !exists || preview.Status != 0 || info.Data.DataVersion == "" {
			return ""
		}
		preview_url := &url.URL{Scheme: "https", Host: "internal-api-drive-stream.feishu.cn", Path: "/space/api/box/stream/download/preview/" + asset.Token}
		query := preview_url.Query()
		query.Set("preview_type", "9")
		query.Set("version", info.Data.DataVersion)
		query.Set("mount_point", "docx_file")
		preview_url.RawQuery = query.Encode()
		return preview_url.String()
	}
	preview, exists := info.Data.PreviewMeta.Data["3"]
	if !exists || len(preview.Content.TranscodeURLs) == 0 {
		return ""
	}
	for _, quality := range []string{"720p", "360p"} {
		if transcode_url := strings.TrimSpace(preview.Content.TranscodeURLs[quality]); transcode_url != "" {
			return transcode_url
		}
	}
	qualities := make([]string, 0, len(preview.Content.TranscodeURLs))
	for quality := range preview.Content.TranscodeURLs {
		qualities = append(qualities, quality)
	}
	sort.Strings(qualities)
	for _, quality := range qualities {
		if transcode_url := strings.TrimSpace(preview.Content.TranscodeURLs[quality]); transcode_url != "" {
			return transcode_url
		}
	}
	return ""
}

func valid_asset_token(token string) bool {
	if token == "" {
		return false
	}
	for _, character := range token {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func (c *Client) cache_document_images(fetch_context context.Context, browser *minib.MiniBrowser, document *Document) error {
	image_indexes := make(map[string][]int)
	requests := make([]image_request, 0)
	for asset_index := range document.Assets {
		asset := &document.Assets[asset_index]
		if asset.Kind != "image" {
			continue
		}
		image_indexes[asset.Token] = append(image_indexes[asset.Token], asset_index)
		if cached_path, cached_mime, cached_size := c.cached_image(document.Token, asset.Token); cached_path != "" {
			asset.LocalPath = cached_path
			asset.MIMEType = cached_mime
			asset.Size = cached_size
			asset.RelativePath = asset_relative_path("image", asset.Token, "", cached_mime)
			continue
		}
		if c.file_cache == nil || !c.file_cache.Enabled() {
			return errors.New("Feishu image download requires an enabled persistent cache")
		}
		width, height := image_request_size(*asset)
		requests = append(requests, image_request{FileToken: asset.Token, Policy: "near", Width: width, Height: height})
	}
	if len(requests) == 0 {
		return nil
	}
	parsed_url, _ := url.Parse(document.URL)
	origin := parsed_url.Scheme + "://" + parsed_url.Host
	headers := http.Header{
		"Accept":       []string{"application/json, text/plain, */*"},
		"Content-Type": []string{"application/json;charset=UTF-8"},
		"Origin":       []string{origin},
		"Referer":      []string{document.URL},
	}
	if csrf_token := browser_cookie(browser, document.URL, "swp_csrf_token"); csrf_token != "" {
		headers.Set("x-csrftoken", csrf_token)
	}
	for offset := 0; offset < len(requests); offset += image_batch_size {
		end := offset + image_batch_size
		if end > len(requests) {
			end = len(requests)
		}
		body, _ := json.Marshal(requests[offset:end])
		response, err := browser.Request(fetch_context, http.MethodPost, origin+"/space/api/box/file/cdn_url/", bytes.NewReader(body), headers)
		if err != nil {
			return fmt.Errorf("resolve Feishu image URLs: %w", err)
		}
		var payload image_response
		if err := json.Unmarshal(response.Body, &payload); err != nil {
			return fmt.Errorf("decode Feishu image URL response: %w", err)
		}
		if response.StatusCode != http.StatusOK || payload.Code != 0 {
			return fmt.Errorf("Feishu image URL lookup returned HTTP %d code %d: %s", response.StatusCode, payload.Code, first_non_empty(payload.Message, payload.Msg, "unknown error"))
		}
		returned := make(map[string]bool, len(payload.Data))
		for _, item := range payload.Data {
			indexes := image_indexes[item.FileToken]
			if len(indexes) == 0 || !item.Permitted || strings.TrimSpace(item.URL) == "" {
				continue
			}
			local_path, mime_type, size, err := c.download_image(fetch_context, browser, document.Token, origin, item)
			if err != nil {
				return fmt.Errorf("download Feishu image %s: %w", item.FileToken, err)
			}
			for _, asset_index := range indexes {
				document.Assets[asset_index].LocalPath = local_path
				document.Assets[asset_index].MIMEType = mime_type
				document.Assets[asset_index].Size = size
				document.Assets[asset_index].RelativePath = asset_relative_path("image", item.FileToken, "", mime_type)
			}
			returned[item.FileToken] = true
		}
		for _, request := range requests[offset:end] {
			if !returned[request.FileToken] {
				return fmt.Errorf("Feishu image URL lookup denied token %s", request.FileToken)
			}
		}
	}
	return nil
}

func image_request_size(asset Asset) (int, int) {
	if asset.MIMEType == "image/gif" || (asset.Width > 0 && asset.Height > asset.Width*2) {
		if asset.Width > 0 && asset.Height > 0 {
			return asset.Width, asset.Height
		}
	}
	return 1280, 1280
}

func (c *Client) cached_image(document_token string, image_token string) (string, string, int64) {
	if c.file_cache == nil || !c.file_cache.Enabled() {
		return "", "", 0
	}
	for _, image_type := range []struct {
		extension string
		mime_type string
	}{
		{".png", "image/png"},
		{".jpg", "image/jpeg"},
		{".gif", "image/gif"},
		{".webp", "image/webp"},
	} {
		relative_path := image_cache_path(document_token, image_token, image_type.extension)
		file_info, err := c.file_cache.Stat(relative_path)
		if err == nil && file_info.Mode().IsRegular() && file_info.Size() > 0 {
			absolute_path, path_err := c.file_cache.Path(relative_path)
			if path_err == nil {
				return absolute_path, image_type.mime_type, file_info.Size()
			}
		}
	}
	return "", "", 0
}

func (c *Client) download_image(fetch_context context.Context, browser *minib.MiniBrowser, document_token string, origin string, item image_item) (string, string, int64, error) {
	image_url, err := url.Parse(strings.TrimSpace(item.URL))
	if err != nil || image_url.Scheme != "https" || image_url.Hostname() == "" {
		return "", "", 0, errors.New("invalid CDN URL")
	}
	response, err := browser.Get(fetch_context, image_url.String(), http.Header{
		"Origin":  []string{origin},
		"Referer": []string{origin + "/"},
	})
	if err != nil {
		return "", "", 0, err
	}
	if response.StatusCode != http.StatusOK {
		return "", "", 0, fmt.Errorf("CDN returned HTTP %d", response.StatusCode)
	}
	content := response.Body
	if fmt.Sprint(item.CipherType) == "1" {
		content, err = decrypt_image(content, item.Secret, item.Nonce)
		if err != nil {
			return "", "", 0, err
		}
	}
	mime_type, extension := image_type(content)
	if mime_type == "" {
		return "", "", 0, errors.New("CDN response is not a supported image")
	}
	relative_path := image_cache_path(document_token, item.FileToken, extension)
	if err := c.file_cache.Write(relative_path, content); err != nil {
		return "", "", 0, err
	}
	absolute_path, err := c.file_cache.Path(relative_path)
	if err != nil {
		return "", "", 0, err
	}
	return absolute_path, mime_type, int64(len(content)), nil
}

func decrypt_image(content []byte, secret string, nonce string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return nil, fmt.Errorf("decode image key: %w", err)
	}
	nonce_data, err := base64.StdEncoding.DecodeString(nonce)
	if err != nil {
		return nil, fmt.Errorf("decode image nonce: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("initialize image cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("initialize image GCM: %w", err)
	}
	plaintext, err := gcm.Open(nil, nonce_data, content, nil)
	if err != nil {
		return nil, errors.New("decrypt image CDN response")
	}
	return plaintext, nil
}

func image_type(content []byte) (string, string) {
	switch {
	case len(content) >= 8 && bytes.Equal(content[:8], []byte("\x89PNG\r\n\x1a\n")):
		return "image/png", ".png"
	case len(content) >= 3 && bytes.Equal(content[:3], []byte("\xff\xd8\xff")):
		return "image/jpeg", ".jpg"
	case len(content) >= 6 && (bytes.Equal(content[:6], []byte("GIF87a")) || bytes.Equal(content[:6], []byte("GIF89a"))):
		return "image/gif", ".gif"
	case len(content) >= 12 && bytes.Equal(content[:4], []byte("RIFF")) && bytes.Equal(content[8:12], []byte("WEBP")):
		return "image/webp", ".webp"
	default:
		return "", ""
	}
}

func image_cache_path(document_token string, image_token string, extension string) string {
	return filepath.ToSlash(filepath.Join("documents", document_token, "images", image_token+extension))
}

func browser_cookie(browser *minib.MiniBrowser, raw_url string, name string) string {
	stored_cookies, err := browser.Cookies(raw_url)
	if err != nil {
		return ""
	}
	for _, cookie := range stored_cookies {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

func browser_cookie_header(browser *minib.MiniBrowser, raw_url string) string {
	stored_cookies, err := browser.Cookies(raw_url)
	if err != nil || len(stored_cookies) == 0 {
		return ""
	}
	request := &http.Request{Header: make(http.Header)}
	for _, cookie := range stored_cookies {
		request.AddCookie(cookie)
	}
	return request.Header.Get("Cookie")
}

func word_count(value string) int {
	count := 0
	for _, character := range value {
		if unicode.IsLetter(character) || unicode.IsNumber(character) {
			count++
		}
	}
	return count
}

func first_non_empty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
