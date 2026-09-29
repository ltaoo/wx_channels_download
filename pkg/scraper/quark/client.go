package quark

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
	"wx_channel/pkg/minib"
)

const (
	share_page_url          = "https://pan.quark.cn/s/"
	detail_api_url          = "https://drive-pc.quark.cn/1/clouddrive/share/sharepage/v2/detail?pr=ucpro&fr=pc"
	download_api_url        = "https://drive-pc.quark.cn/1/clouddrive/file/download?pr=ucpro&fr=pc&sys=win32&ve=6.9.7.761"
	social_token_api_url    = "https://drive-social-api.quark.cn/1/clouddrive/chat/conv/file/acquire_dl_token?pr=ucpro&fr=pc&sys=win32&ve=6.9.7.761&fr=win&la=zh-CN&ch=pckk%40product_guanwan"
	api_user_agent          = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36"
	download_user_agent     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 QuarkPC/6.9.7.761 QuarkCloudDrivePC/6.9.7.761 quark-cloud-drive/2.5.40"
	default_page_size       = 50
	max_tree_depth          = 64
	max_tree_entries        = 10000
	max_download_batch_size = 15
)

// Client fetches Quark Drive share metadata and signed download URLs.
type Client struct {
	claw_client       *clawreq.Client
	claw_client_err   error
	transient_mu      sync.Mutex
	transient_cookies map[string]string
}

// NewClient creates a browser-like Quark Drive client.
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

func (c *Client) SetCookieHeader(header string) {
	if c == nil {
		return
	}
	c.transient_mu.Lock()
	defer c.transient_mu.Unlock()
	for _, part := range strings.Split(header, ";") {
		name, value, found := strings.Cut(strings.TrimSpace(part), "=")
		if !found || strings.TrimSpace(name) == "" {
			continue
		}
		c.transient_cookies[strings.TrimSpace(name)] = value
	}
}

func QuarkUserAgent() string { return download_user_agent }

func (c *Client) fetch_social_token(fetch_context context.Context) string {
	timestamp := time.Now().Unix()
	request_body, _ := json.Marshal(map[string]any{
		"conversation_id":   fmt.Sprintf("300000%d", timestamp),
		"conversation_type": 3,
		"msg_id":            fmt.Sprintf("%d000", timestamp),
	})
	request_options, err := c.request_options(social_token_api_url, map[string]string{
		"Content-Type": "application/json",
		"User-Agent":   api_user_agent,
	})
	if err != nil {
		return ""
	}
	response, err := c.claw_client.Do(
		fetch_context,
		http.MethodPost,
		social_token_api_url,
		bytes.NewReader(request_body),
		request_options...,
	)
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return ""
	}
	var payload struct {
		Code int `json:"code"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if json.Unmarshal(response.Body, &payload) != nil || payload.Code != 0 {
		return ""
	}
	return strings.TrimSpace(payload.Data.Token)
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
			return nil, fmt.Errorf("初始化夸克网盘客户端失败: %w", c.claw_client_err)
		}
		return nil, errors.New("夸克网盘客户端未初始化")
	}
	pwd_id, err := ParseShareURL(raw_url)
	if err != nil {
		return nil, err
	}

	// Navigate the share page with minib to collect JS-set session cookies
	// (ctoken, web-grey-id, etc.) required by the download API.
	if err := c.warm_cookies(fetch_context, pwd_id); err != nil {
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
	share.Files, err = c.build_tree(fetch_context, pwd_id, "0", "", root.stoken, root.files, 0, visited, &entry_count, share)
	if err != nil {
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

// warm_cookies navigates the Quark share page with minib to execute JavaScript
// and collect session cookies (ctoken, web-grey-id, tfstk, isg, etc.) that the
// download API requires. A simple HTTP GET is not enough because these cookies
// are set by client-side scripts.
func (c *Client) warm_cookies(fetch_context context.Context, pwd_id string) error {
	browser, err := minib.NewMiniBrowser(30 * time.Second)
	if err != nil {
		return fmt.Errorf("初始化夸克网盘浏览器失败: %w", err)
	}
	defer browser.Close()

	page_url := share_page_url + pwd_id
	page, err := browser.Navigate(fetch_context, page_url, http.Header{
		"Accept":          {"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"},
		"Accept-Language": {"zh-CN,zh;q=0.9,en;q=0.8"},
	}, minib.NavigateOptions{
		DisableImages: true,
		DisableMedia:  true,
		DisableCSS:    true,
	})
	if err != nil {
		return fmt.Errorf("访问夸克网盘分享页面失败: %w", err)
	}
	_ = page

	// Collect cookies from the browser session for both domains.
	for _, domain := range []string{"https://pan.quark.cn", "https://drive-pc.quark.cn"} {
		cookies, err := browser.Cookies(domain)
		if err != nil {
			continue
		}
		c.transient_mu.Lock()
		for _, cookie := range cookies {
			if cookie != nil && cookie.Name != "" {
				c.transient_cookies[cookie.Name] = cookie.Value
			}
		}
		c.transient_mu.Unlock()
	}
	return nil
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
			"Origin":       "https://pan.quark.cn",
			"Referer":      "https://pan.quark.cn/",
		})
		if err != nil {
			return nil, err
		}
		response, err := c.claw_client.Do(fetch_context, http.MethodPost, detail_api_url, bytes.NewReader(request_body), request_options...)
		if err != nil {
			return nil, fmt.Errorf("请求夸克网盘文件列表失败: %w", err)
		}
		c.remember_response_cookies(response)
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, fmt.Errorf("请求夸克网盘文件列表失败: HTTP %d", response.StatusCode)
		}
		var payload detail_response
		if err := json.Unmarshal(response.Body, &payload); err != nil {
			return nil, fmt.Errorf("解析夸克网盘文件列表失败: %w", err)
		}
		if payload.Code != 0 {
			return nil, fmt.Errorf("夸克网盘文件列表失败: %s", first_non_empty(payload.Message, fmt.Sprintf("code=%d", payload.Code)))
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

func (c *Client) build_tree(fetch_context context.Context, pwd_id string, parent_fid string, parent_path string, parent_stoken string, entries []api_file, depth int, visited map[string]bool, entry_count *int, share *Share) ([]File, error) {
	if depth > max_tree_depth {
		return nil, fmt.Errorf("夸克网盘文件树超过最大深度 %d", max_tree_depth)
	}
	files := make([]File, 0, len(entries))
	for _, entry := range entries {
		*entry_count = *entry_count + 1
		if *entry_count > max_tree_entries {
			return nil, fmt.Errorf("夸克网盘文件数超过最大限制 %d", max_tree_entries)
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
			Stoken:        parent_stoken,
			ShareFidToken: strings.TrimSpace(entry.ShareFidToken),
			CreatedAt:     entry.CreatedAt,
			UpdatedAt:     entry.UpdatedAt,
		}
		if file.Name == "" {
			file.Name = file.Fid
		}
		if is_dir && file.Fid != "" {
			if visited[file.Fid] {
				return nil, fmt.Errorf("夸克网盘文件树存在循环目录: %s", file.Fid)
			}
			visited[file.Fid] = true
			directory, err := c.fetch_directory(fetch_context, pwd_id, file.Fid)
			if err != nil {
				return nil, err
			}
			file.Children, err = c.build_tree(fetch_context, pwd_id, file.Fid, file.Path, directory.stoken, directory.files, depth+1, visited, entry_count, share)
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
		return errors.New("夸克网盘分享为空")
	}
	return c.FetchDownloadLinksForFiles(fetch_context, share, share.DownloadableFiles())
}

// FetchDownloadLinksForFiles refreshes signed URLs for only the requested files.
func (c *Client) FetchDownloadLinksForFiles(fetch_context context.Context, share *Share, files []*File) error {
	if share == nil {
		return errors.New("夸克网盘分享为空")
	}
	if c == nil || c.claw_client == nil {
		if c != nil && c.claw_client_err != nil {
			return fmt.Errorf("初始化夸克网盘客户端失败: %w", c.claw_client_err)
		}
		return errors.New("夸克网盘客户端未初始化")
	}
	if strings.TrimSpace(share.PwdID) == "" || strings.TrimSpace(share.Stoken) == "" {
		return errors.New("夸克网盘分享缺少下载令牌")
	}
	if c.transient_cookie_header() == "" {
		if err := c.warm_cookies(fetch_context, share.PwdID); err != nil {
			return err
		}
	}
	token := c.fetch_social_token(fetch_context)
	for start := 0; start < len(files); {
		end := start + 1
		for end < len(files) && end-start < max_download_batch_size && files[end].Stoken == files[start].Stoken {
			end++
		}
		fids := make([]string, 0, end-start)
		fids_token := make([]string, 0, end-start)
		for _, file := range files[start:end] {
			fids = append(fids, file.Fid)
			fids_token = append(fids_token, file.ShareFidToken)
		}
		stoken := first_non_empty(files[start].Stoken, share.Stoken)
		request_body, _ := json.Marshal(map[string]any{
			"fids":            fids,
			"fids_token":      fids_token,
			"pwd_id":          share.PwdID,
			"stoken":          stoken,
			"speedup_session": "",
			"token":           token,
		})
		request_options, err := c.download_request_options(map[string]string{
			"Accept":       "application/json, text/plain, */*",
			"Content-Type": "application/json;charset=utf-8",
			"Origin":       "https://pan.quark.cn",
			"Referer":      "https://pan.quark.cn/",
			"User-Agent":   api_user_agent,
		})
		if err != nil {
			return err
		}
		response, err := c.claw_client.Do(fetch_context, http.MethodPost, download_api_url, bytes.NewReader(request_body), request_options...)
		if err != nil {
			return fmt.Errorf("请求夸克网盘下载地址失败: %w", err)
		}
		c.remember_response_cookies(response)
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			var payload download_response
			if json.Unmarshal(response.Body, &payload) == nil && payload.Code != 0 {
				return fmt.Errorf(
					"请求夸克网盘下载地址失败: %s",
					first_non_empty(payload.Message, fmt.Sprintf("code=%d", payload.Code)),
				)
			}
			if response.StatusCode == http.StatusForbidden {
				return fmt.Errorf("请求夸克网盘下载地址失败: HTTP 403，请导入有效的 pan.quark.cn Cookie 或确认分享允许下载")
			}
			return fmt.Errorf("请求夸克网盘下载地址失败: HTTP %d", response.StatusCode)
		}
		var payload download_response
		if err := json.Unmarshal(response.Body, &payload); err != nil {
			return fmt.Errorf("解析夸克网盘下载地址失败: %w", err)
		}
		if payload.Code != 0 {
			return fmt.Errorf("夸克网盘下载地址失败: %s", first_non_empty(payload.Message, fmt.Sprintf("code=%d", payload.Code)))
		}
		links := make(map[string]string, len(payload.Data))
		for _, item := range payload.Data {
			links[item.Fid] = first_non_empty(item.DownloadURL, item.URL)
		}
		for _, file := range files[start:end] {
			file.DownloadURL = strings.TrimSpace(links[file.Fid])
			if file.DownloadURL == "" {
				return fmt.Errorf("夸克网盘未返回文件下载地址: %s", file.Name)
			}
		}
		start = end
	}
	share.DownloadCookies = c.download_file_cookie_header()
	return nil
}

func (c *Client) request_options(raw_url string, headers map[string]string) ([]clawreq.RequestOption, error) {
	request_options := []clawreq.RequestOption{clawreq.WithHeaders(headers)}
	cookie_header := c.transient_cookie_header()
	if cookie_header != "" {
		request_options = append(request_options, clawreq.WithCookie(cookie_header))
	}
	return request_options, nil
}

func (c *Client) download_request_options(headers map[string]string) ([]clawreq.RequestOption, error) {
	request_options := []clawreq.RequestOption{clawreq.WithOnlyHeaders(headers)}
	cookie_header := c.download_api_cookie_header()
	if cookie_header != "" {
		request_options = append(request_options, clawreq.WithCookie(cookie_header))
	}
	return request_options, nil
}

func (c *Client) download_api_cookie_header() string {
	if c == nil {
		return ""
	}
	c.transient_mu.Lock()
	defer c.transient_mu.Unlock()
	return "__pugs=" + strings.TrimSpace(c.transient_cookies["__pugs"])
}

func (c *Client) download_file_cookie_header() string {
	if c == nil {
		return ""
	}
	c.transient_mu.Lock()
	defer c.transient_mu.Unlock()
	if pugs := strings.TrimSpace(c.transient_cookies["__pugs"]); pugs != "" {
		return "__pugs=" + pugs
	}
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
