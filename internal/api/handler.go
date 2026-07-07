//go:build legacy_api_handler

package api

import (
	"archive/zip"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
	gopeedhttp "github.com/GopeedLab/gopeed/pkg/protocol/http"
	gopeedstream "github.com/GopeedLab/gopeed/pkg/protocol/stream"
	"github.com/gin-gonic/gin"
	officialaccountdownload "wx_channel/pkg/scraper/officialaccount"

	"wx_channel/frontend"
	result "wx_channel/internal/util"
	channels "wx_channel/pkg/scraper/wxchannels"
	"wx_channel/pkg/system"
	pkgutil "wx_channel/pkg/util"
)

func (c *APIClient) handleSearchChannelsContact(ctx *gin.Context) {
	keyword := ctx.Query("keyword")
	next_marker := ctx.Query("next_marker")
	resp, err := c.channels.SearchChannelsContact(keyword, next_marker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}
func (c *APIClient) handleFetchFeedListOfContact(ctx *gin.Context) {
	username := ctx.Query("username")
	next_marker := ctx.Query("next_marker")
	resp, err := c.channels.FetchChannelsFeedListOfContact(username, next_marker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

func (c *APIClient) handleFetchLiveReplayList(ctx *gin.Context) {
	username := ctx.Query("username")
	next_marker := ctx.Query("next_marker")
	resp, err := c.channels.FetchChannelsLiveReplayList(username, next_marker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

func (c *APIClient) handleFetchInteractionedFeedList(ctx *gin.Context) {
	flag := ctx.Query("flag")
	next_marker := ctx.Query("next_marker")
	resp, err := c.channels.FetchChannelsInteractionedFeedList(flag, next_marker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

func (c *APIClient) handleFetchFollowList(ctx *gin.Context) {
	next_marker := ctx.Query("next_marker")
	resp, err := c.channels.FetchChannelsFollowList(next_marker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

func (c *APIClient) handleFetchFeedCommentList(ctx *gin.Context) {
	oid := ctx.Query("oid")
	nid := ctx.Query("nid")
	comment_id := ctx.Query("comment_id")
	next_marker := ctx.Query("next_marker")
	resp, err := c.channels.FetchChannelsFeedCommentList(oid, nid, comment_id, next_marker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

type AtomAuthor struct {
	Name string `xml:"name"`
}

type AtomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}

type AtomContent struct {
	Type string `xml:"type,attr"`
	Body string `xml:",chardata"`
}

type AtomEntry struct {
	Title     string      `xml:"title"`
	ID        string      `xml:"id"`
	Updated   string      `xml:"updated"`
	Published string      `xml:"published"`
	Link      []AtomLink  `xml:"link"`
	Content   AtomContent `xml:"content"`
	Author    AtomAuthor  `xml:"author"`
}

type AtomFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Title   string      `xml:"title"`
	ID      string      `xml:"id"`
	Updated string      `xml:"updated"`
	Link    []AtomLink  `xml:"link"`
	Author  AtomAuthor  `xml:"author"`
	Entry   []AtomEntry `xml:"entry"`
}

func (c *APIClient) handleFetchFeedListOfContactRSS(ctx *gin.Context) {
	username := ctx.Query("username")
	next_marker := ctx.Query("next_marker")
	resp, err := c.channels.FetchChannelsFeedListOfContact(username, next_marker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	entries := make([]AtomEntry, 0, len(resp.Data.Object))
	for _, obj := range resp.Data.Object {
		var mediaURL, coverURL string
		if len(obj.ObjectDesc.Media) > 0 {
			m := obj.ObjectDesc.Media[0]
			video_url := m.URL + m.URLToken
			addr := c.cfg.Protocol + "://" + c.cfg.Hostname
			if c.cfg.Port != 80 {
				addr += ":" + strconv.Itoa(c.cfg.Port)
			}
			mediaURL = addr + "/play?url=" + url.QueryEscape(video_url) + "&key=" + m.DecodeKey
			coverURL = m.CoverUrl
		}

		desc := obj.ObjectDesc.Description
		if coverURL != "" && mediaURL != "" {
			desc = fmt.Sprintf(`<img src="%s" style="display: none;" /><video controls poster="%s"><source src="%s" type="video/mp4"></video><br/>%s`, coverURL, coverURL, mediaURL, desc)
		} else if coverURL != "" {
			desc = fmt.Sprintf(`<img src="%s" /><br/>%s`, coverURL, desc)
		}

		pubDate := time.Unix(int64(obj.CreateTime), 0).Format(time.RFC3339)

		entries = append(entries, AtomEntry{
			Title:     obj.ObjectDesc.Description,
			ID:        obj.ID,
			Updated:   pubDate,
			Published: pubDate,
			Link: []AtomLink{
				{Rel: "alternate", Href: mediaURL},
			},
			Content: AtomContent{
				Type: "html",
				Body: desc,
			},
			Author: AtomAuthor{
				Name: obj.Contact.Nickname,
			},
		})
	}

	// feedLink := "https://channels.weixin.qq.com"
	if len(resp.Data.Object) > 0 {
		// Use the first object's contact info for the feed (assuming all are from same contact if username was provided)
		// Or just use the response contact info
	}

	links := []AtomLink{
		{Rel: "self", Href: "http://" + ctx.Request.Host + ctx.Request.RequestURI},
		{Rel: "alternate", Href: "https://channels.weixin.qq.com"},
	}

	if resp.Data.ContinueFlag != 0 && resp.Data.LastBuffer != "" {
		u := ctx.Request.URL
		q := u.Query()
		q.Set("next_marker", resp.Data.LastBuffer)
		u.RawQuery = q.Encode()
		nextLink := "http://" + ctx.Request.Host + u.String()
		links = append(links, AtomLink{Rel: "next", Href: nextLink})
	}

	atom := AtomFeed{
		Title:   resp.Data.Contact.Nickname,
		ID:      resp.Data.Contact.Username, // Using username as ID
		Updated: time.Now().Format(time.RFC3339),
		Link:    links,
		Author: AtomAuthor{
			Name: resp.Data.Contact.Nickname,
		},
		Entry: entries,
	}

	ctx.Header("Content-Type", "application/atom+xml; charset=utf-8")
	ctx.XML(http.StatusOK, atom)
}

func (c *APIClient) handleFetchFeedProfile(ctx *gin.Context) {
	oid := ctx.Query("oid")
	uid := ctx.Query("nid")
	_url := ctx.Query("url")
	eid := ctx.Query("eid")
	// 提前解析 URL，如果包含 eid 则提取出来
	if eid == "" && _url != "" {
		if parsedURL, err := url.Parse(_url); err == nil {
			if _eid := parsedURL.Query().Get("eid"); _eid != "" {
				eid = _eid
				_url = ""
			}
		}
	}
	resp, err := c.channels.FetchChannelsFeedProfile(oid, uid, _url, eid)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

func (c *APIClient) handleFetchSharedFeedProfile(ctx *gin.Context) {
	_url := ctx.Query("url")
	if _url == "" {
		result.Err(ctx, 400, "missing url")
		return
	}
	resp, err := c.channels.FetchChannelsSharedFeedProfile(_url)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

func (c *APIClient) handleFetchFeedShareUrl(ctx *gin.Context) {
	oid := ctx.Query("oid")
	if oid == "" {
		result.Err(ctx, 400, "missing oid")
		return
	}
	resp, err := c.channels.FetchChannelsFeedShareUrl(oid)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

type FeedDownloadTaskBody struct {
	Id        string `json:"id"`
	NonceId   string `json:"nonce_id"`
	URL       string `json:"url"`
	Title     string `json:"title"`
	Filename  string `json:"filename"`
	Key       int    `json:"key"`
	Spec      string `json:"spec"`
	Suffix    string `json:"suffix"`
	SourceURL string `json:"source_url"`
	Overwrite bool   `json:"overwrite"`
	Duplicate bool   `json:"duplicate"`
}

func (c *APIClient) handleCreateFeedDownloadTask(ctx *gin.Context) {
	var body FeedDownloadTaskBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		result.Err(ctx, 400, "缺少 feed id 参数")
		return
	}
	if body.Suffix == ".mp3" {
		has_ffmpeg := system.ExistingCommand("ffmpeg")
		if !has_ffmpeg {
			result.Err(ctx, 3001, "下载 mp3 需要支持 ffmpeg 命令")
			return
		}
	}
	if c.downloader == nil {
		result.Err(ctx, 500, "请先初始化 downloader")
		return
	}
	// 先用 NormalizeFilename（不做去重）计算初始路径，用于冲突检测
	filename, dir, err := c.formatter.NormalizeFilename(body.Filename + body.Suffix)
	if err != nil {
		result.Err(ctx, 410, "不合法的文件名，"+err.Error())
		return
	}
	taskName := filename
	taskPath := filepath.Join(c.cfg.DownloadDir, dir)
	taskFilePath := filepath.Join(taskPath, taskName)
	tasks := c.downloader.GetTasks()
	existingTasks := mergeTasks(
		c.find_existing_feed_tasks(tasks, &body),
		findTasksByDownloadFile(tasks, taskPath, taskName, taskFilePath),
	)
	_, statErr := os.Stat(taskFilePath)
	fileExists := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		result.Err(ctx, 500, "检查文件失败："+statErr.Error())
		return
	}
	// 是否需要 ProcessFilename 做并发去重：
	// - duplicate 模式：EnsureFilename 已保证唯一，不需要
	// - 无冲突：NormalizeFilename 已给出正确名称，不需要
	// - overwrite 模式：需要（处理并发 overwrite 请求）
	needDedup := len(existingTasks) > 0 || fileExists
	if len(existingTasks) > 0 || fileExists {
		if body.Duplicate || (len(existingTasks) == 0 && fileExists) {
			// 重复下载：跳过冲突检查，用 EnsureFilename 找到不重名的文件名
			// 本地已有同名文件时也默认走重复下载模式，自动追加 (n)
			uniqueName, err := pkgutil.EnsureFilename(taskName, dir, c.cfg.DownloadDir)
			if err != nil {
				result.Err(ctx, 500, "生成唯一文件名失败："+err.Error())
				return
			}
			taskName = uniqueName
			taskFilePath = filepath.Join(taskPath, taskName)
			needDedup = false
		} else if !body.Overwrite {
			result.Err(ctx, 409, "已存在该下载内容")
			return
		} else {
			if err := c.deleteTasks(existingTasks, true); err != nil {
				result.Err(ctx, 500, "删除已存在任务失败："+err.Error())
				return
			}
			if fileExists {
				if err := removeExistingDownloadFile(taskFilePath); err != nil {
					result.Err(ctx, 500, "覆盖已存在文件失败："+err.Error())
					return
				}
			}
			// overwrite 后清除 in-memory 去重残留，后续 ProcessFilename 会重新登记
			c.formatter.RemoveFilename(taskName, dir)
		}
	}
	if needDedup {
		// 并发去重：防止两个请求同时创建同名任务时文件名冲突
		filename, dir, err = c.processTaskFilename(body.Filename, body.Suffix)
		if err != nil {
			result.Err(ctx, 410, "不合法的文件名，"+err.Error())
			return
		}
		taskName = filename
		taskPath = filepath.Join(c.cfg.DownloadDir, dir)
		taskFilePath = filepath.Join(taskPath, taskName)
	}
	connections := c.resolve_connections(body.URL)
	id, err := c.downloader.CreateDirect(
		&base.Request{
			URL:            body.URL,
			SkipVerifyCert: true,
			Labels: map[string]string{
				"id":         body.Id,
				"nonce_id":   body.NonceId,
				"title":      body.Title,
				"key":        strconv.Itoa(body.Key),
				"spec":       body.Spec,
				"suffix":     body.Suffix,
				"source_url": body.SourceURL,
			},
		},
		&base.Options{
			Name: taskName,
			Path: taskPath,
			Extra: &gopeedhttp.OptsExtra{
				Connections: connections,
			},
		},
	)
	if err != nil {
		c.logger.Error().Interface("body", body).Err(err).Msg("创建任务失败")
		result.Err(ctx, 500, "创建任务失败："+err.Error())
		return
	}
	task := c.downloader.GetTask(id)
	if task != nil {
		c.downloader_ws.Broadcast(APIClientWSMessage{
			Type: "event",
			Data: map[string]interface{}{
				"task":          task,
				"status_counts": c.downloadTaskStatusCounts(),
			},
		})
	}
	result.Ok(ctx, newCreateTaskResp(id, taskPath, taskName))
}

type DownloadTaskPayload struct {
	URL      string            `json:"url"`
	Filename string            `json:"filename"`
	Dir      string            `json:"dir"`
	Extra    map[string]string `json:"extra"`
}

type DownloadTaskBatchPayload struct {
	Text  string                `json:"text"`
	URLs  []string              `json:"urls"`
	Tasks []DownloadTaskPayload `json:"tasks"`
	Dir   string                `json:"dir"`
	Extra map[string]string     `json:"extra"`
}

type DownloadTaskCreateFailure struct {
	URL     string `json:"url"`
	Code    int    `json:"code,omitempty"`
	Message string `json:"message"`
}

func cloneDownloadTaskExtra(extra map[string]string) map[string]string {
	if len(extra) == 0 {
		return nil
	}
	clone := make(map[string]string, len(extra))
	for k, v := range extra {
		clone[k] = v
	}
	return clone
}

func applyDownloadTaskDefaults(task DownloadTaskPayload, dir string, extra map[string]string) DownloadTaskPayload {
	task.URL = strings.TrimSpace(task.URL)
	if task.Dir == "" {
		task.Dir = dir
	}
	labels := cloneDownloadTaskExtra(extra)
	for k, v := range task.Extra {
		if labels == nil {
			labels = make(map[string]string)
		}
		labels[k] = v
	}
	task.Extra = labels
	return task
}

func parseDownloadTaskText(text string, dir string, extra map[string]string) []DownloadTaskPayload {
	var tasks []DownloadTaskPayload
	normalizedText := strings.ReplaceAll(text, "\r\n", "\n")
	for _, line := range strings.Split(normalizedText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "{") {
			var task DownloadTaskPayload
			if err := json.Unmarshal([]byte(line), &task); err == nil && strings.TrimSpace(task.URL) != "" {
				tasks = append(tasks, applyDownloadTaskDefaults(task, dir, extra))
				continue
			}
		}
		tasks = append(tasks, DownloadTaskPayload{
			URL:   line,
			Dir:   dir,
			Extra: cloneDownloadTaskExtra(extra),
		})
	}
	return tasks
}

func appendDownloadTaskPayload(tasks []DownloadTaskPayload, task DownloadTaskPayload, dir string, extra map[string]string) []DownloadTaskPayload {
	task = applyDownloadTaskDefaults(task, dir, extra)
	if task.URL == "" {
		return tasks
	}
	return append(tasks, task)
}

func (c *APIClient) broadcastCreatedDownloadTask(id string) {
	task := c.downloader.GetTask(id)
	if task == nil {
		return
	}
	c.downloader_ws.Broadcast(APIClientWSMessage{
		Type: "event",
		Data: map[string]interface{}{
			"task":          task,
			"status_counts": c.downloadTaskStatusCounts(),
		},
	})
}

func (c *APIClient) createDownloadTask(body DownloadTaskPayload) (string, int, string) {
	body.URL = strings.TrimSpace(body.URL)
	if body.URL == "" {
		return "", 400, "缺少 url 参数"
	}
	if c.downloader == nil {
		return "", 500, "请先初始化 downloader"
	}

	// Extract article_id for officialaccount URLs
	articleID := officialaccountdownload.ExtractArticleID(body.URL)

	tasks := c.downloader.GetTasks()
	for _, t := range tasks {
		if t == nil || t.Meta == nil || t.Meta.Req == nil {
			continue
		}
		// For officialaccount URLs, compare by article_id label
		if articleID != "" && t.Meta.Req.Labels != nil && t.Meta.Req.Labels["article_id"] == articleID {
			return "", 409, "已存在该下载内容"
		}
		// For other URLs, compare by URL directly
		if articleID == "" && t.Meta.Req.URL == body.URL {
			return "", 409, "已存在该下载内容"
		}
	}

	labels := cloneDownloadTaskExtra(body.Extra)
	if labels == nil {
		labels = make(map[string]string)
	}
	if articleID != "" {
		labels["article_id"] = articleID
		// Pass filename template to the officialaccount fetcher
		filenameTemplate := c.cfg.Original.GetString("download.filenameTemplate")
		if filenameTemplate != "" {
			labels["filename_template"] = filenameTemplate
		}
	}

	id, err := c.downloader.CreateDirect(
		&base.Request{
			URL:    body.URL,
			Labels: labels,
		},
		&base.Options{
			Name: body.Filename,
			Path: filepath.Join(c.cfg.DownloadDir, body.Dir),
			Extra: &gopeedhttp.OptsExtra{
				Connections: 1,
			},
		},
	)
	if err != nil {
		return "", 500, "创建任务失败：" + err.Error()
	}
	c.broadcastCreatedDownloadTask(id)
	return id, 0, ""
}

// 创建常规下载任务
func (c *APIClient) handleCreateDownloadTask(ctx *gin.Context) {
	var body DownloadTaskPayload
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	id, code, msg := c.createDownloadTask(body)
	if code != 0 {
		result.Err(ctx, code, msg)
		return
	}
	result.Ok(ctx, gin.H{"id": id})
}

// 批量创建常规下载任务
func (c *APIClient) handleBatchCreateDownloadTask(ctx *gin.Context) {
	var body DownloadTaskBatchPayload
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}

	var tasks []DownloadTaskPayload
	for _, task := range body.Tasks {
		tasks = appendDownloadTaskPayload(tasks, task, body.Dir, body.Extra)
	}
	for _, rawURL := range body.URLs {
		tasks = appendDownloadTaskPayload(tasks, DownloadTaskPayload{URL: rawURL}, body.Dir, body.Extra)
	}
	tasks = append(tasks, parseDownloadTaskText(body.Text, body.Dir, body.Extra)...)
	if len(tasks) == 0 {
		result.Err(ctx, 400, "请提供下载任务")
		return
	}

	seen := make(map[string]bool)
	ids := make([]string, 0, len(tasks))
	skipped := make([]DownloadTaskCreateFailure, 0)
	failed := make([]DownloadTaskCreateFailure, 0)
	for _, task := range tasks {
		if seen[task.URL] {
			skipped = append(skipped, DownloadTaskCreateFailure{
				URL:     task.URL,
				Code:    409,
				Message: "重复的下载地址",
			})
			continue
		}
		seen[task.URL] = true
		id, code, msg := c.createDownloadTask(task)
		if code == 0 {
			ids = append(ids, id)
			continue
		}
		item := DownloadTaskCreateFailure{
			URL:     task.URL,
			Code:    code,
			Message: msg,
		}
		if code == 409 {
			skipped = append(skipped, item)
			continue
		}
		failed = append(failed, item)
	}
	if len(ids) == 0 && len(skipped) == 0 && len(failed) > 0 {
		result.Err(ctx, failed[0].Code, failed[0].Message)
		return
	}
	result.Ok(ctx, gin.H{
		"ids":     ids,
		"skipped": skipped,
		"failed":  failed,
	})
}

func (c *APIClient) handleFetchTaskList(ctx *gin.Context) {
	status := ctx.Query("status")
	page_str := ctx.Query("page")
	page_size_str := ctx.Query("page_size")

	filter := &downloadpkg.TaskFilter{}
	if status != "" && status != "all" {
		filter.Statuses = []base.Status{base.Status(status)}
	}
	list := c.downloader.GetTasksByFilter(filter)
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	total := len(list)
	page_num, err := strconv.Atoi(page_str)
	if err != nil {
		page_num = 1
	}
	page_size_num, err := strconv.Atoi(page_size_str)
	if err != nil {
		page_size_num = 20
	}
	start := (page_num - 1) * page_size_num
	if start > total {
		start = total
	}
	end := start + page_size_num
	if end > total {
		end = total
	}
	result.Ok(ctx, gin.H{
		"list":          list[start:end],
		"total":         total,
		"page":          page_num,
		"page_size":     page_size_num,
		"status_counts": c.downloadTaskStatusCounts(),
	})
}

func normalizeDownloadTaskStatus(status base.Status) string {
	value := strings.ToLower(strings.TrimSpace(string(status)))
	switch value {
	case "paused":
		return "pause"
	case "failed", "fail", "failure", "errored":
		return "error"
	case "pending", "waiting", "queued":
		return "wait"
	case "completed", "success", "finished":
		return "done"
	default:
		return value
	}
}

func downloadTaskStatusFilter(status string) *downloadpkg.TaskFilter {
	value := normalizeDownloadTaskStatus(base.Status(status))
	if value == "" || value == "all" {
		return nil
	}
	statuses := []base.Status{base.Status(value)}
	if value == "wait" {
		statuses = []base.Status{
			base.DownloadStatusReady,
			base.DownloadStatusWait,
		}
	}
	return &downloadpkg.TaskFilter{Statuses: statuses}
}

func downloadTaskPauseAllFilter(status string) *downloadpkg.TaskFilter {
	filter := downloadTaskStatusFilter(status)
	if filter != nil {
		return filter
	}
	return &downloadpkg.TaskFilter{
		Statuses: []base.Status{
			base.DownloadStatusReady,
			base.DownloadStatusRunning,
			base.DownloadStatusWait,
		},
	}
}

func countDownloadTaskStatuses(tasks []*downloadpkg.Task) map[string]int {
	counts := map[string]int{
		"total":   0,
		"ready":   0,
		"running": 0,
		"wait":    0,
		"pause":   0,
		"error":   0,
		"done":    0,
	}
	for _, task := range tasks {
		if task == nil {
			continue
		}
		counts["total"]++
		status := normalizeDownloadTaskStatus(task.Status)
		if status == "" {
			continue
		}
		counts[status]++
	}
	return counts
}

func (c *APIClient) downloadTaskStatusCounts() map[string]int {
	if c == nil || c.downloader == nil {
		return countDownloadTaskStatuses(nil)
	}
	return countDownloadTaskStatuses(c.downloader.GetTasks())
}

type LiveDownloadTaskBody struct {
	Url       string            `json:"url"`
	Name      string            `json:"name"`
	UserAgent string            `json:"userAgent"`
	Headers   map[string]string `json:"headers"`
}

func (c *APIClient) handleCreateLiveTask(ctx *gin.Context) {
	var body LiveDownloadTaskBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.Url == "" {
		result.Err(ctx, 400, "缺少 url 参数")
		return
	}

	name := body.Name
	if name == "" {
		// Try to parse from URL or use timestamp
		u, _ := url.Parse(body.Url)
		if u != nil {
			name = filepath.Base(u.Path)
		}
		if name == "" || name == "." || name == "/" {
			name = fmt.Sprintf("live_%d.mp4", time.Now().Unix())
		}
	}
	if !strings.HasSuffix(name, ".mp4") && !strings.HasSuffix(name, ".ts") && !strings.HasSuffix(name, ".flv") && !strings.HasSuffix(name, ".mkv") {
		name += ".mp4"
	}

	reqExtra := &gopeedstream.ReqExtra{
		Header: make(map[string]string),
	}
	if body.UserAgent != "" {
		reqExtra.Header["User-Agent"] = body.UserAgent
	}
	for k, v := range body.Headers {
		reqExtra.Header[k] = v
	}

	id, err := c.downloader.CreateDirect(
		&base.Request{
			URL:   body.Url,
			Extra: reqExtra,
			Labels: map[string]string{
				"type": "live",
			},
		},
		&base.Options{
			Name: name,
			Path: c.cfg.DownloadDir,
		},
	)
	if err != nil {
		result.Err(ctx, 500, "创建任务失败: "+err.Error())
		return
	}
	task := c.downloader.GetTask(id)
	if task != nil {
		c.downloader_ws.Broadcast(APIClientWSMessage{
			Type: "event",
			Data: map[string]interface{}{
				"task":          task,
				"status_counts": c.downloadTaskStatusCounts(),
			},
		})
	}
	result.Ok(ctx, gin.H{"id": id})
}

// 批量创建下载任务
func (c *APIClient) handleBatchCreateTask(ctx *gin.Context) {
	var body struct {
		Feeds []FeedDownloadTaskBody `json:"feeds"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	tasks := c.downloader.GetTasks()
	existing_task_map := make(map[string]int)
	for _, t := range tasks {
		if t == nil || t.Meta == nil || t.Meta.Req == nil || t.Meta.Req.Labels == nil {
			continue
		}
		key := fmt.Sprintf("%s|%s|%s", t.Meta.Req.Labels["id"], t.Meta.Req.Labels["spec"], t.Meta.Req.Labels["suffix"])
		existing_task_map[key] = 1
	}
	task, err := buildBatchCreateTask(c, existing_task_map, body.Feeds, c.cfg.DownloadDir)
	if err != nil {
		result.Err(ctx, 500, "文件名处理失败: "+err.Error())
		return
	}
	if len(task.Reqs) == 0 {
		result.Ok(ctx, gin.H{"ids": []string{}})
		return
	}
	// start := time.Now()
	ids, err := c.downloader.CreateDirectBatch(task)
	if err != nil {
		c.logger.Error().Interface("body", body).Err(err).Msg("创建任务失败")
		result.Err(ctx, 500, "创建任务失败: "+err.Error())
		return
	}
	var batchTasks []interface{}
	for _, id := range ids {
		task := c.downloader.GetTask(id)
		if task != nil {
			batchTasks = append(batchTasks, task)
		}
	}
	if len(batchTasks) > 0 {
		c.downloader_ws.Broadcast(APIClientWSMessage{
			Type: "batch_tasks",
			Data: batchTasks,
		})
	}
	result.Ok(ctx, gin.H{"ids": ids})
}

func buildBatchCreateTask(c *APIClient, existing_task_map map[string]int, feeds []FeedDownloadTaskBody, download_dir string) (*base.CreateTaskBatch, error) {
	var items []map[string]string
	for _, req := range feeds {
		key := fmt.Sprintf("%s|%s|%s", req.Id, req.Spec, req.Suffix)
		_, exists := existing_task_map[key]
		if exists {
			continue
		}
		items = append(items, map[string]string{
			"id":       req.Id,
			"nonce_id": req.NonceId,
			"title":    req.Title,
			"key":      strconv.Itoa(req.Key),
			"spec":     req.Spec,
			"suffix":   req.Suffix,
			"url":      req.URL,
			"name":     req.Filename,
		})
	}
	if len(items) == 0 {
		return &base.CreateTaskBatch{}, nil
	}
	task := base.CreateTaskBatch{}
	for _, item := range items {
		filename, dir, err := c.processTaskFilename(item["name"], item["suffix"])
		if err != nil {
			continue
		}
		url := item["url"]
		task.Reqs = append(task.Reqs, &base.CreateTaskBatchItem{
			Req: &base.Request{
				URL:            url,
				SkipVerifyCert: true,
				Labels: map[string]string{
					"id":       item["id"],
					"nonce_id": item["nonce_id"],
					"title":    item["title"],
					"key":      item["key"],
					"spec":     item["spec"],
					"suffix":   item["suffix"],
				},
			},
			Opts: &base.Options{
				Name: filename,
				Path: filepath.Join(download_dir, dir),
			},
		})
	}
	return &task, nil
}

type ChannelsDownloadPayload struct {
	Oid   string `json:"oid"`
	Nid   string `json:"nid"`
	Eid   string `json:"eid"`
	URL   string `json:"url"`
	Spec  string `json:"spec"`  // 自定义规格，为空时下载原始视频
	MP3   bool   `json:"mp3"`   // 是否下载为 mp3
	Cover bool   `json:"cover"` // 是否下载封面
}

func (c *APIClient) handleCreateChannelsTask(ctx *gin.Context) {
	var body ChannelsDownloadPayload
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.Oid == "" && body.Nid == "" && body.URL == "" && body.Eid == "" {
		result.Err(ctx, 400, "缺少参数")
		return
	}
	// 提前解析 URL，如果包含 eid 则提取出来
	if body.Eid == "" && body.URL != "" {
		if parsedURL, err := url.Parse(body.URL); err == nil {
			if eid := parsedURL.Query().Get("eid"); eid != "" {
				body = ChannelsDownloadPayload{
					Eid: eid,
				}
			}
		}
	}
	payload, _, err := c.createFeedTaskBody(body.Oid, body.Nid, body.URL, body.Eid, body.MP3, body.Cover, body.Spec)
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}

	if payload.Id == "" {
		result.Err(ctx, 400, "缺少 feed id 参数")
		return
	}
	if payload.Suffix == ".mp3" {
		has_ffmpeg := system.ExistingCommand("ffmpeg")
		if !has_ffmpeg {
			result.Err(ctx, 3001, "下载 mp3 需要支持 ffmpeg 命令")
			return
		}
	}
	tasks := c.downloader.GetTasks()
	existing := c.check_existing_feed(tasks, payload)
	if existing {
		result.Err(ctx, 409, "已存在该下载内容")
		// ctx.JSON(http.StatusOK, Response{Code: 409, Msg: , Data: body})
		return
	}
	filename, dir, err := c.processTaskFilename(payload.Filename, payload.Suffix)
	if err != nil {
		result.Err(ctx, 410, "不合法的文件名，"+err.Error())
		return
	}
	taskName := filename
	taskPath := filepath.Join(c.cfg.DownloadDir, dir)
	connections := c.resolve_connections(payload.URL)
	id, err := c.downloader.CreateDirect(
		&base.Request{
			URL:            payload.URL,
			SkipVerifyCert: true,
			Labels: map[string]string{
				"id":     payload.Id,
				"title":  payload.Title,
				"key":    strconv.Itoa(payload.Key),
				"spec":   payload.Spec,
				"suffix": payload.Suffix,
			},
		},
		&base.Options{
			Name: taskName,
			Path: taskPath,
			Extra: &gopeedhttp.OptsExtra{
				Connections: connections,
			},
		},
	)
	if err != nil {
		result.Err(ctx, 500, "下载失败")
		return
	}
	task := c.downloader.GetTask(id)
	if task != nil {
		c.downloader_ws.Broadcast(APIClientWSMessage{
			Type: "event",
			Data: map[string]interface{}{
				"task":          task,
				"status_counts": c.downloadTaskStatusCounts(),
			},
		})
	}
	result.Ok(ctx, newCreateTaskResp(id, taskPath, taskName))
}

func (c *APIClient) handleStartTask(ctx *gin.Context) {
	var body struct {
		Id string `json:"id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		result.Err(ctx, 400, "缺少 feed id 参数")
		return
	}
	c.downloader.Continue(&downloadpkg.TaskFilter{
		IDs: []string{body.Id},
	})
	result.Ok(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handleStartAllTasks(ctx *gin.Context) {
	var body struct {
		Status string `json:"status"`
	}
	if ctx.Request.Body != nil && ctx.Request.ContentLength != 0 {
		if err := ctx.ShouldBindJSON(&body); err != nil {
			result.Err(ctx, 400, "不合法的参数")
			return
		}
	}
	if err := c.downloader.Continue(downloadTaskStatusFilter(body.Status)); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, nil)
}

func (c *APIClient) handlePauseTask(ctx *gin.Context) {
	var body struct {
		Id string `json:"id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		result.Err(ctx, 400, "缺少 feed id 参数")
		return
	}
	c.downloader.Pause(&downloadpkg.TaskFilter{
		IDs: []string{body.Id},
	})
	result.Ok(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handlePauseAllTasks(ctx *gin.Context) {
	var body struct {
		Status string `json:"status"`
	}
	if ctx.Request.Body != nil && ctx.Request.ContentLength != 0 {
		if err := ctx.ShouldBindJSON(&body); err != nil {
			result.Err(ctx, 400, "不合法的参数")
			return
		}
	}
	if err := c.downloader.Pause(downloadTaskPauseAllFilter(body.Status)); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, nil)
}

func (c *APIClient) handleResumeTask(ctx *gin.Context) {
	var body struct {
		Id string `json:"id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		result.Err(ctx, 400, "缺少 feed id 参数")
		return
	}
	c.downloader.Continue(&downloadpkg.TaskFilter{
		IDs: []string{body.Id},
	})
	result.Ok(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handleDeleteTask(ctx *gin.Context) {
	var body struct {
		Id          string `json:"id"`
		DeleteFiles bool   `json:"delete_files"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		result.Err(ctx, 400, "缺少 feed id 参数")
		return
	}
	if err := c.downloader.Delete(&downloadpkg.TaskFilter{
		IDs: []string{body.Id},
	}, body.DeleteFiles); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handleClearTasks(ctx *gin.Context) {
	var body struct {
		DeleteFiles bool `json:"delete_files"`
	}
	if ctx.Request.Body != nil && ctx.Request.ContentLength != 0 {
		if err := ctx.ShouldBindJSON(&body); err != nil && err != io.EOF {
			result.Err(ctx, 400, "不合法的参数")
			return
		}
	}
	if err := c.downloader.Delete(nil, body.DeleteFiles); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	c.downloader_ws.Broadcast(APIClientWSMessage{
		Type: "clear",
		Data: c.downloader.GetTasks(),
	})
	result.Ok(ctx, nil)
}

func (c *APIClient) handleIndex(ctx *gin.Context) {
	c.handleDownloadPage(ctx)
}

func (c *APIClient) handleDownloadPage(ctx *gin.Context) {
	c.renderInjectedRootHTML(ctx, "index.html")
}

func (c *APIClient) handleChannelsPage(ctx *gin.Context) {
	c.renderInjectedRootHTML(ctx, "channels.html")
}

func (c *APIClient) renderInjectedRootHTML(ctx *gin.Context, name string) {
	data, err := interceptor.Assets.ReadRoot(name)
	if err != nil {
		result.Err(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	cfgByte, _ := json.Marshal(c.cfg)
	html := string(data)
	html = strings.ReplaceAll(html, "__WX_DOWNLOAD_CONFIG_JSON__", string(cfgByte))
	html = strings.ReplaceAll(html, "__WX_DOWNLOAD_VERSION__", "local")

	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, html)
}

func (c *APIClient) handlePlay(ctx *gin.Context) {
	target_url := ctx.Query("url")
	if target_url == "" {
		result.Err(ctx, 400, "missing targetURL")
		return
	}
	if !strings.HasPrefix(target_url, "http") {
		target_url = "https://" + target_url
	}
	if _, err := url.Parse(target_url); err != nil {
		result.Err(ctx, 400, "Invalid URL")
		return
	}
	decrypt_key_str := ctx.Query("key")
	decryptor := channels.NewChannelsVideoDecryptor()
	if decrypt_key_str != "" {
		decryptKey, err := strconv.ParseUint(decrypt_key_str, 0, 64)
		if err != nil {
			result.Err(ctx, 400, "invalid decryptKey")
			return
		}
		decryptor.DecryptOnlyInline(ctx.Writer, ctx.Request, target_url, decryptKey, 131072)
		return
	}
	decryptor.SimpleProxy(target_url, ctx.Writer, ctx.Request)
}

func (c *APIClient) handleOpenDownloadDir(ctx *gin.Context) {
	dir := c.cfg.DownloadDir
	if err := system.Open(dir); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, nil)
}

type OpenFolderAndHighlightFileBody struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	FilePath string `json:"file_path"`
}

// 在打开文件夹并选中指定文件
func (c *APIClient) handleHighlightFileInFolder(ctx *gin.Context) {
	var body OpenFolderAndHighlightFileBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	full_filepath := strings.TrimSpace(body.FilePath)
	if full_filepath == "" && body.Path != "" && body.Name != "" {
		full_filepath = filepath.Join(body.Path, body.Name)
	}
	if full_filepath == "" {
		result.Err(ctx, 400, "Missing the `file_path` or `path` and `name`")
		return
	}
	_, err := os.Stat(full_filepath)
	if err != nil {
		result.Err(ctx, 500, "找不到文件")
		return
	}
	if err := system.ShowInExplorer(full_filepath); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, nil)
}

// 根据任务ID流式返回视频
func (c *APIClient) handleStreamVideo(ctx *gin.Context) {
	path := ctx.Query("path")
	if path == "" {
		task_id := ctx.Query("id")
		if task_id != "" {
			task := c.downloader.GetTask(task_id)
			if task != nil && task.Meta != nil && task.Meta.Opts != nil {
				path = filepath.Join(task.Meta.Opts.Path, task.Meta.Opts.Name)
			}
		}
	}

	if path == "" {
		result.Err(ctx, 400, "missing path or id")
		return
	}

	_, err := os.Stat(path)
	if err != nil {
		result.Err(ctx, 404, "file not found")
		return
	}
	ctx.File(path)
}

func (c *APIClient) handleStreamImage(ctx *gin.Context) {
	c.handleStreamVideo(ctx)
}

func (c *APIClient) handlePreviewFile(ctx *gin.Context) {
	content := files.HTMLPreview
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(200, string(content))
}

func (c *APIClient) handleFetchTaskProfile(ctx *gin.Context) {
	id := ctx.Query("id")
	if id == "" {
		result.Err(ctx, 400, "missing task id")
		return
	}
	task := c.downloader.GetTask(id)
	if task == nil {
		result.Err(ctx, 404, "task not found")
		return
	}
	if task.Meta == nil || task.Meta.Req == nil {
		result.Err(ctx, 400, "invalid task meta")
		return
	}
	result.Ok(ctx, gin.H{
		"path": task.Meta.Opts.Path,
		"name": task.Meta.Opts.Name,
	})
}

type ImageFileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	URL     string `json:"url"`
	Size    int64  `json:"size"`
	Width   int    `json:"width,omitempty"`
	Height  int    `json:"height,omitempty"`
	ModTime string `json:"mod_time,omitempty"`
}

func (c *APIClient) imageFileInfo(root string, path string, info fs.FileInfo) ImageFileInfo {
	name, err := filepath.Rel(root, path)
	if err != nil {
		name = filepath.Base(path)
	}
	name = filepath.ToSlash(name)
	width, height := imageDimensions(path)
	return ImageFileInfo{
		Name:    name,
		Path:    path,
		URL:     "/file?path=" + url.QueryEscape(path),
		Size:    info.Size(),
		Width:   width,
		Height:  height,
		ModTime: info.ModTime().Format(time.RFC3339),
	}
}

func imageDimensions(path string) (int, int) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer file.Close()
	cfg, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

func (c *APIClient) listImageFiles(root string) ([]ImageFileInfo, error) {
	root = filepath.Clean(root)
	images := make([]ImageFileInfo, 0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !c.isImage(strings.ToLower(filepath.Ext(d.Name()))) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		images = append(images, c.imageFileInfo(root, path, info))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(images, func(i, j int) bool {
		return images[i].Name < images[j].Name
	})
	return images, nil
}

func (c *APIClient) handleFetchFile(ctx *gin.Context) {
	path := strings.TrimSpace(ctx.Query("path"))
	if path == "" {
		result.Err(ctx, 400, "missing path")
		return
	}
	// Check if file exists
	fi, err := os.Stat(path)
	if err != nil {
		result.Err(ctx, 404, "file not found")
		return
	}
	if fi.IsDir() {
		if !filepath.IsAbs(path) {
			result.Err(ctx, 400, "path must be absolute")
			return
		}
		images, err := c.listImageFiles(path)
		if err != nil {
			result.Err(ctx, 500, err.Error())
			return
		}
		result.Ok(ctx, gin.H{
			"type":   "directory",
			"path":   filepath.Clean(path),
			"images": images,
		})
		return
	}

	ext := strings.ToLower(filepath.Ext(path))
	if c.isImage(ext) {
		result.Ok(ctx, gin.H{
			"type": "image",
			"url":  "/file?path=" + url.QueryEscape(path),
		})
		return
	}

	if ext == ".mp3" || (c.isVideoOrImage(ext) && !c.isImage(ext)) {
		result.Ok(ctx, gin.H{
			"type": "video",
			"url":  "/file?path=" + url.QueryEscape(path),
		})
		return
	}

	if ext == ".html" || ext == ".htm" {
		result.Ok(ctx, gin.H{
			"type": "html",
			"url":  "/file?path=" + url.QueryEscape(path),
		})
		return
	}

	if ext == ".zip" {
		r, err := zip.OpenReader(path)
		if err != nil {
			result.Err(ctx, 500, fmt.Sprintf("failed to open zip: %v", err))
			return
		}
		defer r.Close()

		var images []map[string]string
		for _, f := range r.File {
			fExt := strings.ToLower(filepath.Ext(f.Name))
			if c.isImage(fExt) {
				rc, err := f.Open()
				if err != nil {
					continue
				}
				if f.FileInfo().Size() > 10*1024*1024 { // 10MB limit
					rc.Close()
					continue
				}
				data, err := io.ReadAll(rc)
				rc.Close()
				if err != nil {
					continue
				}

				base64Str := base64.StdEncoding.EncodeToString(data)
				mimeType := c.getMimeType(fExt)
				imgSrc := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Str)
				images = append(images, map[string]string{
					"name": f.Name,
					"url":  imgSrc,
				})
			}
		}
		result.Ok(ctx, gin.H{
			"type":   "zip",
			"images": images,
		})
		return
	}

	result.Err(ctx, 400, "unsupported file type")
}

func (c *APIClient) isVideoOrImage(ext string) bool {
	if c.isImage(ext) {
		return true
	}
	switch ext {
	case ".mp4", ".mkv", ".avi", ".mov", ".webm":
		return true
	}
	return false
}

func (c *APIClient) isImage(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
		return true
	}
	return false
}

func (c *APIClient) getMimeType(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	}
	return "image/jpeg"
}

func (c *APIClient) handleGetFileURL(ctx *gin.Context) {
	id := ctx.Query("id")
	if id == "" {
		result.Err(ctx, 400, "missing id")
		return
	}
	url := c.cfg.Protocol + "://" + c.cfg.Hostname
	if c.cfg.Port != 80 {
		url += ":" + strconv.Itoa(c.cfg.Port)
	}
	url += "/video?id=" + id
	result.Ok(ctx, gin.H{
		"url": url,
	})
}

func (c *APIClient) handleTest(ctx *gin.Context) {
	dir := c.cfg.DownloadDir
	if err := system.Open(dir); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, nil)
}

func (c *APIClient) handleDownloadAllOfficialAccountMsgs(ctx *gin.Context) {
	var body struct {
		Biz        string `json:"biz"`
		Uin        string `json:"uin"`
		Key        string `json:"key"`
		PassTicket string `json:"pass_ticket"`
		Token      string `json:"token"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "参数错误")
		return
	}
	if valid := c.official.ValidateToken(body.Token); !valid {
		result.Err(ctx, 401, "token 无效")
		return
	}
	if body.Biz == "" {
		result.Err(ctx, 400, "缺少 biz 参数")
		return
	}
	acct := &officialaccount.OfficialAccount{
		Biz:        body.Biz,
		Uin:        body.Uin,
		Key:        body.Key,
		PassTicket: body.PassTicket,
	}
	urls, err := c.official.FetchAllMsgURLs(acct)
	if err != nil && len(urls) == 0 {
		result.Err(ctx, 500, err.Error())
		return
	}
	count := 0
	for _, u := range urls {
		taskURL := "officialaccount://" + u
		_, code, _ := c.createDownloadTask(DownloadTaskPayload{URL: taskURL})
		if code == 0 {
			count++
		}
	}
	result.Ok(ctx, gin.H{"count": count, "total": len(urls)})
}
