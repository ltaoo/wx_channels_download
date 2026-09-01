package ucdrive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"wx_channel/pkg/clawreq"
)

const (
	detail_api_url          = "https://pc-api.uc.cn/1/clouddrive/share/sharepage/v2/detail?pr=UCBrowser&fr=pc"
	download_api_url        = "https://pc-api.uc.cn/1/clouddrive/file/download?entry=ft&fr=pc&pr=UCBrowser"
	default_page_size       = 50
	max_tree_depth          = 64
	max_tree_entries        = 10000
	max_download_batch_size = 15
)

// Client fetches UC Drive share metadata and signed download URLs.
type Client struct {
	claw_client       *clawreq.Client
	claw_client_err   error
	transient_mu      sync.Mutex
	transient_cookies map[string]string
}

// NewClient creates a browser-like UC Drive client.
func NewClient() *Client {
	claw_client, claw_client_err := clawreq.New(clawreq.Config{
		Profile:         clawreq.ProfileChrome,
		Timeout:         30 * time.Second,
		FollowRedirects: true,
	})
	return &Client{
		claw_client:       claw_client,
		claw_client_err:   claw_client_err,
		transient_cookies: make(map[string]string),
	}
}

// Fetch fetches a public share, including its complete file tree and download URLs.
func (c *Client) Fetch(raw_url string) (*Share, error) {
	return c.FetchContext(context.Background(), raw_url)
}

// FetchContext is Fetch with cancellation support.
func (c *Client) FetchContext(fetch_context context.Context, raw_url string) (*Share, error) {
	if fetch_context == nil {
		fetch_context = context.Background()
	}
	if c == nil || c.claw_client == nil {
		if c != nil && c.claw_client_err != nil {
			return nil, fmt.Errorf("初始化 UC 网盘客户端失败: %w", c.claw_client_err)
		}
		return nil, errors.New("UC 网盘客户端未初始化")
	}
	pwd_id, err := ParseShareURL(raw_url)
	if err != nil {
		return nil, err
	}

	share := &Share{URL: strings.TrimSpace(raw_url), PwdID: pwd_id}
	root, err := c.fetch_directory(fetch_context, pwd_id, "0")
	if err != nil {
		return nil, err
	}
	share.Title = root.title
	share.Author = root.author
	share.AuthorAvatarURL = root.author_avatar_url
	share.ExpiresAt = root.expires_at
	share.Stoken = root.stoken

	entry_count := 0
	visited := make(map[string]bool)
	share.Files, err = c.build_tree(fetch_context, pwd_id, "0", "", root.files, 0, visited, &entry_count, share)
	if err != nil {
		return nil, err
	}
	if err := c.FetchDownloadLinks(fetch_context, share); err != nil {
		return nil, err
	}
	return share, nil
}

type detail_page struct {
	title             string
	author            string
	author_avatar_url string
	expires_at        int64
	stoken            string
	files             []api_file
}

type detail_response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Stoken    string `json:"stoken"`
		TokenInfo struct {
			Stoken string `json:"stoken"`
			Author struct {
				NickName  string `json:"nick_name"`
				Avatar    string `json:"avatar"`
				AvatarURL string `json:"avatar_url"`
			} `json:"author"`
		} `json:"token_info"`
		DetailInfo struct {
			Stoken string `json:"stoken"`
			Share  struct {
				Title     string `json:"title"`
				ExpiredAt int64  `json:"expired_at"`
				Stoken    string `json:"stoken"`
			} `json:"share"`
			List []api_file `json:"list"`
		} `json:"detail_info"`
		List []api_file `json:"list"`
	} `json:"data"`
}

type api_file struct {
	Fid           string `json:"fid"`
	FileName      string `json:"file_name"`
	ParentFid     string `json:"pdir_fid"`
	FileType      int    `json:"file_type"`
	Size          int64  `json:"size"`
	FormatType    string `json:"format_type"`
	ShareFidToken string `json:"share_fid_token"`
	Dir           bool   `json:"dir"`
	File          bool   `json:"file"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

type download_response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    []struct {
		Fid         string `json:"fid"`
		DownloadURL string `json:"download_url"`
		URL         string `json:"url"`
	} `json:"data"`
}

func (c *Client) fetch_directory(fetch_context context.Context, pwd_id string, parent_fid string) (*detail_page, error) {
	page := 1
	result := &detail_page{}
	for {
		request_body, _ := json.Marshal(map[string]any{
			"pwd_id":                 pwd_id,
			"passcode":               "",
			"force":                  0,
			"page":                   page,
			"size":                   default_page_size,
			"fetch_banner":           1,
			"fetch_share":            1,
			"fetch_total":            1,
			"sort":                   "file_type:asc,file_name:asc",
			"banner_platform":        "other",
			"pdir_fid":               parent_fid,
			"web_platform":           "mac",
			"fetch_error_background": 1,
		})
		request_options, err := c.request_options(detail_api_url, map[string]string{
			"Accept":       "application/json, text/plain, */*",
			"Content-Type": "application/json;charset=utf-8",
			"Origin":       "https://drive.uc.cn",
			"Referer":      "https://drive.uc.cn/",
		})
		if err != nil {
			return nil, err
		}
		response, err := c.claw_client.Do(fetch_context, http.MethodPost, detail_api_url, bytes.NewReader(request_body), request_options...)
		if err != nil {
			return nil, fmt.Errorf("请求 UC 网盘文件列表失败: %w", err)
		}
		c.remember_response_cookies(response)
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, fmt.Errorf("请求 UC 网盘文件列表失败: HTTP %d", response.StatusCode)
		}
		var payload detail_response
		if err := json.Unmarshal(response.Body, &payload); err != nil {
			return nil, fmt.Errorf("解析 UC 网盘文件列表失败: %w", err)
		}
		if payload.Code != 0 {
			return nil, fmt.Errorf("UC 网盘文件列表失败: %s", first_non_empty(payload.Message, fmt.Sprintf("code=%d", payload.Code)))
		}
		if page == 1 {
			result.title = first_non_empty(payload.Data.DetailInfo.Share.Title, pwd_id)
			result.author = payload.Data.TokenInfo.Author.NickName
			result.author_avatar_url = first_non_empty(payload.Data.TokenInfo.Author.Avatar, payload.Data.TokenInfo.Author.AvatarURL)
			result.expires_at = payload.Data.DetailInfo.Share.ExpiredAt
			result.stoken = first_non_empty(payload.Data.TokenInfo.Stoken, payload.Data.Stoken, payload.Data.DetailInfo.Stoken, payload.Data.DetailInfo.Share.Stoken)
		}
		page_files := payload.Data.DetailInfo.List
		if len(page_files) == 0 {
			page_files = payload.Data.List
		}
		result.files = append(result.files, page_files...)
		if len(page_files) < default_page_size {
			return result, nil
		}
		page++
	}
}

func (c *Client) build_tree(fetch_context context.Context, pwd_id string, parent_fid string, parent_path string, entries []api_file, depth int, visited map[string]bool, entry_count *int, share *Share) ([]File, error) {
	if depth > max_tree_depth {
		return nil, fmt.Errorf("UC 网盘文件树超过最大深度 %d", max_tree_depth)
	}
	files := make([]File, 0, len(entries))
	for _, entry := range entries {
		*entry_count = *entry_count + 1
		if *entry_count > max_tree_entries {
			return nil, fmt.Errorf("UC 网盘文件数超过最大限制 %d", max_tree_entries)
		}
		is_dir := entry.Dir || !entry.File && entry.FileType == 0
		file := File{
			Fid:           entry.Fid,
			Name:          strings.TrimSpace(entry.FileName),
			Path:          file_path(parent_path, entry.FileName, entry.Fid),
			ParentFid:     first_non_empty(entry.ParentFid, parent_fid),
			IsDir:         is_dir,
			Size:          entry.Size,
			FormatType:    strings.TrimSpace(entry.FormatType),
			FileType:      entry.FileType,
			ShareFidToken: strings.TrimSpace(entry.ShareFidToken),
			CreatedAt:     entry.CreatedAt,
			UpdatedAt:     entry.UpdatedAt,
		}
		if file.Name == "" {
			file.Name = file.Fid
		}
		if is_dir && file.Fid != "" {
			if visited[file.Fid] {
				return nil, fmt.Errorf("UC 网盘文件树存在循环目录: %s", file.Fid)
			}
			visited[file.Fid] = true
			directory, err := c.fetch_directory(fetch_context, pwd_id, file.Fid)
			if err != nil {
				return nil, err
			}
			file.Children, err = c.build_tree(fetch_context, pwd_id, file.Fid, file.Path, directory.files, depth+1, visited, entry_count, share)
			if err != nil {
				return nil, err
			}
		}
		if !file.IsDir {
			share.FileCount++
			share.TotalSize += file.Size
		}
		files = append(files, file)
	}
	return files, nil
}

// FetchDownloadLinks refreshes the signed URLs of every file in a share.
func (c *Client) FetchDownloadLinks(fetch_context context.Context, share *Share) error {
	if share == nil {
		return errors.New("UC 网盘分享为空")
	}
	if c == nil || c.claw_client == nil {
		if c != nil && c.claw_client_err != nil {
			return fmt.Errorf("初始化 UC 网盘客户端失败: %w", c.claw_client_err)
		}
		return errors.New("UC 网盘客户端未初始化")
	}
	if strings.TrimSpace(share.PwdID) == "" || strings.TrimSpace(share.Stoken) == "" {
		return errors.New("UC 网盘分享缺少下载令牌")
	}
	files := make([]*File, 0)
	for index := range share.Files {
		collect_downloadable_files(&share.Files[index], &files)
	}
	for start := 0; start < len(files); start += max_download_batch_size {
		end := start + max_download_batch_size
		if end > len(files) {
			end = len(files)
		}
		fids := make([]string, 0, end-start)
		fids_token := make([]string, 0, end-start)
		for _, file := range files[start:end] {
			fids = append(fids, file.Fid)
			fids_token = append(fids_token, file.ShareFidToken)
		}
		request_body, _ := json.Marshal(map[string]any{
			"fids":       fids,
			"fids_token": fids_token,
			"pwd_id":     share.PwdID,
			"stoken":     share.Stoken,
		})
		request_options, err := c.request_options(download_api_url, map[string]string{
			"Accept":       "application/json, text/plain, */*",
			"Content-Type": "application/json;charset=utf-8",
			"Origin":       "https://drive.uc.cn",
			"Referer":      "https://drive.uc.cn/",
		})
		if err != nil {
			return err
		}
		response, err := c.claw_client.Do(fetch_context, http.MethodPost, download_api_url, bytes.NewReader(request_body), request_options...)
		if err != nil {
			return fmt.Errorf("请求 UC 网盘下载地址失败: %w", err)
		}
		c.remember_response_cookies(response)
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return fmt.Errorf("请求 UC 网盘下载地址失败: HTTP %d", response.StatusCode)
		}
		var payload download_response
		if err := json.Unmarshal(response.Body, &payload); err != nil {
			return fmt.Errorf("解析 UC 网盘下载地址失败: %w", err)
		}
		if payload.Code != 0 {
			return fmt.Errorf("UC 网盘下载地址失败: %s", first_non_empty(payload.Message, fmt.Sprintf("code=%d", payload.Code)))
		}
		links := make(map[string]string, len(payload.Data))
		for _, item := range payload.Data {
			links[item.Fid] = first_non_empty(item.DownloadURL, item.URL)
		}
		for _, file := range files[start:end] {
			file.DownloadURL = strings.TrimSpace(links[file.Fid])
			if file.DownloadURL == "" {
				return fmt.Errorf("UC 网盘未返回文件下载地址: %s", file.Name)
			}
		}
	}
	share.DownloadCookies = c.transient_cookie_header()
	return nil
}

func collect_downloadable_files(file *File, files *[]*File) {
	if file == nil {
		return
	}
	if file.IsDir {
		for index := range file.Children {
			collect_downloadable_files(&file.Children[index], files)
		}
		return
	}
	*files = append(*files, file)
}

func (c *Client) request_options(raw_url string, headers map[string]string) ([]clawreq.RequestOption, error) {
	request_options := []clawreq.RequestOption{clawreq.WithHeaders(headers)}
	if cookie_header := c.transient_cookie_header(); cookie_header != "" {
		request_options = append(request_options, clawreq.WithCookie(cookie_header))
	}
	return request_options, nil
}

func (c *Client) remember_response_cookies(response *clawreq.Response) {
	if c == nil || response == nil {
		return
	}
	cookies := (&http.Response{Header: response.Header}).Cookies()
	if len(cookies) == 0 {
		return
	}
	c.transient_mu.Lock()
	defer c.transient_mu.Unlock()
	if c.transient_cookies == nil {
		c.transient_cookies = make(map[string]string)
	}
	for _, cookie := range cookies {
		if cookie == nil || cookie.Name == "" {
			continue
		}
		if cookie.MaxAge < 0 || (!cookie.Expires.IsZero() && cookie.Expires.Before(time.Now())) {
			delete(c.transient_cookies, cookie.Name)
			continue
		}
		c.transient_cookies[cookie.Name] = cookie.Value
	}
}

func (c *Client) transient_cookie_header() string {
	if c == nil {
		return ""
	}
	c.transient_mu.Lock()
	defer c.transient_mu.Unlock()
	names := make([]string, 0, len(c.transient_cookies))
	for name := range c.transient_cookies {
		names = append(names, name)
	}
	sort.Strings(names)
	values := make([]string, 0, len(names))
	for _, name := range names {
		values = append(values, name+"="+c.transient_cookies[name])
	}
	return strings.Join(values, "; ")
}

func first_non_empty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
