package api

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
	gopeedhttp "github.com/GopeedLab/gopeed/pkg/protocol/http"
	gopeedstream "github.com/GopeedLab/gopeed/pkg/protocol/stream"
	"github.com/gin-gonic/gin"

	"wx_channel/internal/api/types"
	"wx_channel/internal/channels"
	"wx_channel/internal/interceptor"
	result "wx_channel/internal/util"
	"wx_channel/pkg/system"
	"wx_channel/pkg/util"
)

func (c *APIClient) handleSearchChannelsContact(ctx *gin.Context) {
	keyword := ctx.Query("keyword")
	resp, err := c.channels.SearchChannelsContact(keyword)
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
			mediaURL = "http://" + c.cfg.Addr + "/play?url=" + url.QueryEscape(video_url) + "&key=" + m.DecodeKey
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
	url := ctx.Query("url")
	resp, err := c.channels.FetchChannelsFeedProfile(oid, uid, url)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

type FeedDownloadTaskBody struct {
	Id       string `json:"id"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Filename string `json:"filename"`
	Key      int    `json:"key"`
	Spec     string `json:"spec"`
	Suffix   string `json:"suffix"`
}

func (c *APIClient) handleCreateTask(ctx *gin.Context) {
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
	tasks := c.downloader.GetTasks()
	existing := c.check_existing_feed(tasks, &body)
	if existing {
		result.Err(ctx, 409, "已存在该下载内容")
		// ctx.JSON(http.StatusOK, Response{Code: 409, Msg: , Data: body})
		return
	}
	filename, dir, err := c.formatter.ProcessFilename(body.Filename)
	if err != nil {
		result.Err(ctx, 409, "不合法的文件名")
		return
	}
	connections := c.resolve_connections(body.URL)
	id, err := c.downloader.CreateDirect(
		&base.Request{
			URL: body.URL,
			Labels: map[string]string{
				"id":     body.Id,
				"title":  body.Title,
				"key":    strconv.Itoa(body.Key),
				"spec":   body.Spec,
				"suffix": body.Suffix,
			},
		},
		&base.Options{
			Name: filename + body.Suffix,
			Path: filepath.Join(c.cfg.DownloadDir, dir),
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
	c.channels.Broadcast(APIClientWSMessage{
		Type: "tasks",
		Data: c.downloader.GetTasks(),
	})
	result.Ok(ctx, gin.H{"id": id})
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
	c.channels.Broadcast(APIClientWSMessage{
		Type: "tasks",
		Data: c.downloader.GetTasks(),
	})
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
	existing_task_map := make(map[string]struct{})
	for _, t := range tasks {
		if t == nil || t.Meta == nil || t.Meta.Req == nil || t.Meta.Req.Labels == nil {
			continue
		}
		key := fmt.Sprintf("%s|%s|%s", t.Meta.Req.Labels["id"], t.Meta.Req.Labels["spec"], t.Meta.Req.Labels["suffix"])
		existing_task_map[key] = struct{}{}
	}

	var items []map[string]string
	for _, req := range body.Feeds {
		key := fmt.Sprintf("%s|%s|%s", req.Id, req.Spec, req.Suffix)
		if _, exists := existing_task_map[key]; exists {
			continue
		}
		items = append(items, map[string]string{
			"id":     req.Id,
			"title":  req.Title,
			"key":    strconv.Itoa(req.Key),
			"spec":   req.Spec,
			"suffix": req.Suffix,
			"url":    req.URL,
			"name":   req.Filename,
		})
	}
	if len(items) == 0 {
		result.Ok(ctx, gin.H{"ids": []string{}})
		return
	}
	processed_reqs, err := util.ProcessFilenames(items, c.cfg.DownloadDir)
	if err != nil {
		result.Err(ctx, 500, "文件名处理失败: "+err.Error())
		return
	}
	task := base.CreateTaskBatch{}
	for _, item := range processed_reqs {
		url := item["url"]
		full_path := item["full_path"]
		// 从 full_path 中提取目录
		rel_dir := filepath.Dir(full_path)
		// connections := c.resolve_connections(url)
		task.Reqs = append(task.Reqs, &base.CreateTaskBatchItem{
			Req: &base.Request{
				URL: url,
				Labels: map[string]string{
					"id":     item["id"],
					"title":  item["title"],
					"key":    item["key"],
					"spec":   item["spec"],
					"suffix": item["suffix"],
				},
			},
			Opts: &base.Options{
				Name: item["name"] + item["suffix"],
				Path: filepath.Join(c.cfg.DownloadDir, rel_dir),
			},
		})
	}
	ids, err := c.downloader.CreateDirectBatch(&task)
	if err != nil {
		c.logger.Error().Interface("body", body).Err(err).Msg("创建任务失败")
		result.Err(ctx, 500, "创建任务失败: "+err.Error())
		return
	}
	c.channels.Broadcast(APIClientWSMessage{
		Type: "tasks",
		Data: c.downloader.GetTasks(),
	})
	result.Ok(ctx, gin.H{"ids": ids})
}

func (c *APIClient) handleCreateChannelsTask(ctx *gin.Context) {
	var body struct {
		Oid   string `json:"oid"`
		Nid   string `json:"Nid"`
		URL   string `json:"url"`
		MP3   bool   `json:"mp3"`   // 是否下载为 mp3
		Cover bool   `json:"cover"` // 是否下载封面
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.Oid == "" && body.Nid == "" && body.URL == "" {
		result.Err(ctx, 400, "缺少参数")
		return
	}
	r, err := c.channels.FetchChannelsFeedProfile(body.Oid, body.Nid, body.URL)
	if err != nil {
		result.Err(ctx, 500, "获取详情失败: "+err.Error())
		return
	}
	if r.ErrCode != 0 {
		result.Err(ctx, 500, "获取详情失败: "+r.ErrMsg)
		return
	}
	if len(r.Data.Object.ObjectDesc.Media) == 0 {
		result.Err(ctx, 500, "缺少可下载的视频内容")
		return
	}
	media := r.Data.Object.ObjectDesc.Media[0]
	key := 0
	if media.DecodeKey != "" {
		k, err := strconv.Atoi(media.DecodeKey)
		if err != nil {
			result.Err(ctx, 500, "获取详情失败: "+err.Error())
			return
		}
		key = k
	}
	spec := "original"
	if !c.cfg.Original.GetBool("download.defaultHighest") {
		if len(media.Spec) > 0 {
			spec = media.Spec[0].FileFormat
		}
	}
	build_filename := func(feed types.ChannelsObject, spec string) string {
		default_name := feed.ObjectDesc.Description
		if default_name == "" {
			if feed.ID != "" {
				default_name = feed.ID
			} else {
				default_name = util.NowSecondsStr()
			}
		}
		template := c.cfg.Original.GetString("download.filenameTemplate")
		if template == "" {
			return default_name
		}
		params := map[string]string{
			"filename":    default_name,
			"id":          feed.ID,
			"title":       feed.ObjectDesc.Description,
			"spec":        spec,
			"created_at":  strconv.Itoa(feed.CreateTime),
			"download_at": util.NowSecondsStr(),
			"author":      feed.Contact.Nickname,
		}

		result := template
		for k, v := range params {
			result = strings.ReplaceAll(result, "{{"+k+"}}", v)
		}
		return result
	}
	filename := build_filename(r.Data.Object, spec)
	payload := FeedDownloadTaskBody{
		Id:       r.Data.Object.ID,
		Title:    r.Data.Object.ObjectDesc.Description,
		Key:      key,
		Spec:     spec,
		Suffix:   ".mp4",
		URL:      media.URL + media.URLToken,
		Filename: filename,
	}
	if body.MP3 {
		payload.Suffix = ".mp3"
	}
	if body.Cover {
		payload.Suffix += ".jpg"
		cover_url := media.CoverUrl
		payload.URL = cover_url
	}

	if r.Data.Object.ObjectDesc.MediaType == 2 {
		payload.Suffix = ".zip"
		var files []map[string]string
		for i, m := range r.Data.Object.ObjectDesc.Media {
			files = append(files, map[string]string{
				"url":      m.URL + m.URLToken,
				"filename": fmt.Sprintf("%d.jpg", i+1),
			})
		}
		data, _ := json.Marshal(files)
		payload.URL = fmt.Sprintf("zip://weixin.qq.com?files=%s", url.QueryEscape(string(data)))
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
	existing := c.check_existing_feed(tasks, &payload)
	if existing {
		result.Err(ctx, 409, "已存在该下载内容")
		// ctx.JSON(http.StatusOK, Response{Code: 409, Msg: , Data: body})
		return
	}
	filename, dir, err := c.formatter.ProcessFilename(payload.Filename)
	if err != nil {
		result.Err(ctx, 409, "不合法的文件名")
		return
	}
	connections := c.resolve_connections(payload.URL)
	id, err := c.downloader.CreateDirect(
		&base.Request{
			URL: payload.URL,
			Labels: map[string]string{
				"id":     payload.Id,
				"title":  payload.Title,
				"key":    strconv.Itoa(payload.Key),
				"spec":   payload.Spec,
				"suffix": payload.Suffix,
			},
		},
		&base.Options{
			Name: filename + payload.Suffix,
			Path: filepath.Join(c.cfg.DownloadDir, dir),
			Extra: &gopeedhttp.OptsExtra{
				Connections: connections,
			},
		},
	)
	if err != nil {
		result.Err(ctx, 500, "下载失败")
		return
	}
	c.channels.Broadcast(APIClientWSMessage{
		Type: "tasks",
		Data: c.downloader.GetTasks(),
	})
	result.Ok(ctx, gin.H{"id": id})
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
	task := c.downloader.GetTask(body.Id)
	c.downloader.Delete(&downloadpkg.TaskFilter{
		IDs: []string{body.Id},
	}, true)
	c.channels.Broadcast(APIClientWSMessage{
		Type: "event",
		Data: map[string]any{
			"Type": "delete",
			"Task": task,
		},
	})
	result.Ok(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handleClearTasks(ctx *gin.Context) {
	c.downloader.Delete(nil, true)
	c.channels.Broadcast(APIClientWSMessage{
		Type: "clear",
		Data: c.downloader.GetTasks(),
	})
	result.Ok(ctx, nil)
}

func (c *APIClient) handleIndex(ctx *gin.Context) {
	read_asset := func(path string, defaultData []byte) string {
		fullPath := filepath.Join("internal", "interceptor", path)
		data, err := os.ReadFile(fullPath)
		if err == nil {
			return string(data)
		}
		return string(defaultData)
	}
	// html := read_asset("inject/index.html", files.HTMLHome)
	files := interceptor.Assets
	// css := read_asset("inject/lib/weui.min.css", files.CSSWeui)
	// html = strings.Replace(html, "/* INJECT_CSS */", css, 1)
	var inserted_scripts string
	cfg_byte, _ := json.Marshal(c.cfg)
	inserted_scripts += fmt.Sprintf(`<script>var __wx_channels_config__ = %s; var __wx_channels_version__ = "local";</script>`, string(cfg_byte))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, read_asset("inject/lib/mitt.umd.js", files.JSMitt))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, read_asset("inject/src/eventbus.js", files.JSEventBus))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, read_asset("inject/src/utils.js", files.JSUtils))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, read_asset("inject/lib/floating-ui.core.1.7.4.min.js", files.JSFloatingUICore))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, read_asset("inject/lib/floating-ui.dom.1.7.4.min.js", files.JSFloatingUIDOM))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, read_asset("inject/lib/weui.min.js", files.JSWeui))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, read_asset("inject/lib/wui.umd.js", files.JSWui))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, read_asset("inject/src/components.js", files.JSComponents))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, read_asset("inject/src/downloader.js", files.JSDownloader))

	// html = strings.Replace(html, "<!-- INJECT_JS -->", inserted_scripts, 1)

	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, "<html><body><div id=\"app\"></div></body></html>")
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
	Path string `json:"path"`
	Name string `json:"name"`
}

// 在打开文件夹并选中指定文件
func (c *APIClient) handleHighlightFileInFolder(ctx *gin.Context) {
	var body OpenFolderAndHighlightFileBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	if body.Path == "" || body.Name == "" {
		result.Err(ctx, 400, "Missing the `path` or `name`")
		return
	}
	full_filepath := filepath.Join(body.Path, body.Name)
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

func (c *APIClient) handleTest(ctx *gin.Context) {
	dir := c.cfg.DownloadDir
	if err := system.Open(dir); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, nil)
}
