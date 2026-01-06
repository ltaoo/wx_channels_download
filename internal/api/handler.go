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

	"wx_channel/internal/interceptor"
	"wx_channel/pkg/system"
	"wx_channel/pkg/util"
)

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

func (c *APIClient) handleSearchChannelsContact(ctx *gin.Context) {
	keyword := ctx.Query("keyword")
	resp, err := c.SearchChannelsContact(keyword)
	if err != nil {
		c.jsonError(ctx, 400, err.Error())
		return
	}
	c.jsonSuccess(ctx, resp)
}
func (c *APIClient) handleFetchFeedListOfContact(ctx *gin.Context) {
	username := ctx.Query("username")
	nextMarker := ctx.Query("next_marker")
	resp, err := c.FetchChannelsFeedListOfContact(username, nextMarker)
	if err != nil {
		c.jsonError(ctx, 400, err.Error())
		return
	}
	c.jsonSuccess(ctx, resp)
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
	resp, err := c.FetchChannelsFeedListOfContact(username, next_marker)
	if err != nil {
		c.jsonError(ctx, 400, err.Error())
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
	resp, err := c.FetchChannelsFeedProfile(oid, uid, url)
	if err != nil {
		c.jsonError(ctx, 400, err.Error())
		return
	}
	c.jsonSuccess(ctx, resp)
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
		c.jsonError(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		c.jsonError(ctx, 400, "缺少 feed id 参数")
		return
	}
	if body.Suffix == ".mp3" {
		has_ffmpeg := system.ExistingCommand("ffmpeg")
		if !has_ffmpeg {
			c.jsonError(ctx, 400, "下载 mp3 需要支持 ffmpeg 命令")
			return
		}
	}
	tasks := c.downloader.GetTasks()
	existing := c.check_existing_feed(tasks, &body)
	if existing {
		ctx.JSON(http.StatusOK, Response{Code: 409, Msg: "已存在该下载内容", Data: body})
		return
	}
	filename, dir, err := c.formatter.ProcessFilename(body.Filename)
	if err != nil {
		c.jsonError(ctx, 409, "不合法的文件名")
		return
	}
	connections := c.resolveConnections(body.URL)
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
		c.jsonError(ctx, 500, "下载失败")
		return
	}
	c.broadcast(APIClientWSMessage{
		Type: "tasks",
		Data: c.downloader.GetTasks(),
	})
	c.jsonSuccess(ctx, gin.H{"id": id})
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
		c.jsonError(ctx, 400, "不合法的参数")
		return
	}
	if body.Url == "" {
		c.jsonError(ctx, 400, "缺少 url 参数")
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
		c.jsonError(ctx, 500, "创建任务失败: "+err.Error())
		return
	}

	c.broadcast(APIClientWSMessage{
		Type: "tasks",
		Data: c.downloader.GetTasks(),
	})
	c.jsonSuccess(ctx, gin.H{"id": id})
}

func (c *APIClient) handleBatchCreateTask(ctx *gin.Context) {
	var body struct {
		Feeds []FeedDownloadTaskBody `json:"feeds"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.jsonError(ctx, 400, "不合法的参数")
		return
	}
	tasks := c.downloader.GetTasks()
	var items []map[string]string
	for _, req := range body.Feeds {
		if c.check_existing_feed(tasks, &req) {
			continue
		}
		items = append(items, map[string]string{
			"name":   req.Filename,
			"id":     req.Id,
			"url":    req.URL,
			"title":  req.Title,
			"key":    strconv.Itoa(req.Key),
			"suffix": req.Suffix,
		})
	}
	if len(items) == 0 {
		c.jsonSuccess(ctx, gin.H{"ids": []string{}})
		return
	}
	processed_reqs, err := util.ProcessFilenames(items, c.cfg.DownloadDir)
	if err != nil {
		c.jsonError(ctx, 500, "文件名处理失败: "+err.Error())
		return
	}
	task := base.CreateTaskBatch{}
	for _, item := range processed_reqs {
		url := item["url"]
		fullPath := item["full_path"]
		// 从 full_path 中提取目录
		relDir := filepath.Dir(fullPath)

		connections := c.resolveConnections(url)

		task.Reqs = append(task.Reqs, &base.CreateTaskBatchItem{
			Req: &base.Request{
				URL: url,
				Labels: map[string]string{
					"id":     item["id"],
					"title":  item["title"],
					"key":    item["key"],
					"suffix": item["suffix"],
				},
			},
			Opts: &base.Options{
				Name: item["name"] + item["suffix"],
				Path: filepath.Join(c.cfg.DownloadDir, relDir),
				Extra: &gopeedhttp.OptsExtra{
					Connections: connections,
				},
			},
		})
	}

	ids, err := c.downloader.CreateDirectBatch(&task)
	if err != nil {
		c.jsonError(ctx, 500, "创建失败")
		return
	}
	c.broadcast(APIClientWSMessage{
		Type: "tasks",
		Data: c.downloader.GetTasks(),
	})
	c.jsonSuccess(ctx, gin.H{"ids": ids})
}

func (c *APIClient) handleStartTask(ctx *gin.Context) {
	var body struct {
		Id string `json:"id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.jsonError(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		c.jsonError(ctx, 400, "缺少 feed id 参数")
		return
	}
	c.downloader.Continue(&downloadpkg.TaskFilter{
		IDs: []string{body.Id},
	})
	c.jsonSuccess(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handlePauseTask(ctx *gin.Context) {
	var body struct {
		Id string `json:"id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.jsonError(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		c.jsonError(ctx, 400, "缺少 feed id 参数")
		return
	}
	c.downloader.Pause(&downloadpkg.TaskFilter{
		IDs: []string{body.Id},
	})
	c.jsonSuccess(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handleResumeTask(ctx *gin.Context) {
	var body struct {
		Id string `json:"id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.jsonError(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		c.jsonError(ctx, 400, "缺少 feed id 参数")
		return
	}
	c.downloader.Continue(&downloadpkg.TaskFilter{
		IDs: []string{body.Id},
	})
	c.jsonSuccess(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handleDeleteTask(ctx *gin.Context) {
	var body struct {
		Id string `json:"id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.jsonError(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		c.jsonError(ctx, 400, "缺少 feed id 参数")
		return
	}
	task := c.downloader.GetTask(body.Id)
	c.downloader.Delete(&downloadpkg.TaskFilter{
		IDs: []string{body.Id},
	}, true)
	c.broadcast(APIClientWSMessage{
		Type: "event",
		Data: map[string]any{
			"Type": "delete",
			"Task": task,
		},
	})
	c.jsonSuccess(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handleClearTasks(ctx *gin.Context) {
	c.downloader.Delete(nil, true)
	c.broadcast(APIClientWSMessage{
		Type: "clear",
		Data: c.downloader.GetTasks(),
	})
	c.jsonSuccess(ctx, nil)
}

func (c *APIClient) handleIndex(ctx *gin.Context) {
	readAsset := func(path string, defaultData []byte) string {
		fullPath := filepath.Join("internal", "interceptor", path)
		data, err := os.ReadFile(fullPath)
		if err == nil {
			return string(data)
		}
		return string(defaultData)
	}
	html := readAsset("inject/index.html", interceptor.Assets.IndexHTML)
	files := interceptor.Assets
	css := readAsset("inject/lib/weui.min.css", files.CSSWeui)
	html = strings.Replace(html, "/* INJECT_CSS */", css, 1)
	var inserted_scripts string
	cfg_byte, _ := json.Marshal(c.cfg)
	inserted_scripts += fmt.Sprintf(`<script>var __wx_channels_config__ = %s; var __wx_channels_version__ = "local";</script>`, string(cfg_byte))

	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, readAsset("inject/lib/mitt.umd.js", files.JSMitt))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, readAsset("inject/src/eventbus.js", files.JSEventBus))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, readAsset("inject/src/utils.js", files.JSUtils))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, readAsset("inject/lib/floating-ui.core.1.7.4.min.js", files.JSFloatingUICore))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, readAsset("inject/lib/floating-ui.dom.1.7.4.min.js", files.JSFloatingUIDOM))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, readAsset("inject/lib/weui.min.js", files.JSWeui))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, readAsset("inject/lib/wui.umd.js", files.JSWui))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, readAsset("inject/src/components.js", files.JSComponents))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, readAsset("inject/src/downloader.js", files.JSDownloader))

	html = strings.Replace(html, "<!-- INJECT_JS -->", inserted_scripts, 1)

	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, html)
}

func (c *APIClient) handleOpenDownloadDir(ctx *gin.Context) {
	dir := c.cfg.DownloadDir
	if err := system.Open(dir); err != nil {
		c.jsonError(ctx, 500, err.Error())
		return
	}
	c.jsonSuccess(ctx, nil)
}
func (c *APIClient) handleTest(ctx *gin.Context) {
	dir := c.cfg.DownloadDir
	if err := system.Open(dir); err != nil {
		c.jsonError(ctx, 500, err.Error())
		return
	}
	c.jsonSuccess(ctx, nil)
}

// func (c *APIClient) handleDownload(ctx *gin.Context) {
// 	targetURL := ctx.Query("url")
// 	if targetURL == "" {
// 		c.jsonError(ctx, 400, "missing targetURL")
// 		return
// 	}
// 	if !strings.HasPrefix(targetURL, "http") {
// 		targetURL = "https://" + targetURL
// 	}
// 	if _, err := url.Parse(targetURL); err != nil {
// 		c.jsonError(ctx, 400, "Invalid URL")
// 		return
// 	}
// 	filename := ctx.Query("filename")
// 	if filename == "" {
// 		if u, err := url.Parse(targetURL); err == nil {
// 			if base := path.Base(u.Path); base != "" && base != "/" {
// 				filename = base
// 			}
// 		}
// 		if filename == "" {
// 			filename = "download.mp4"
// 		}
// 	}
// 	decryptKeyStr := ctx.Query("key")
// 	toMP3 := ctx.Query("mp3")
// 	mp := NewChannelsVideoDecryptor()
// 	if decryptKeyStr != "" {
// 		decryptKey, err := strconv.ParseUint(decryptKeyStr, 0, 64)
// 		if err != nil {
// 			c.jsonError(ctx, 400, "invalid decryptKey")
// 			return
// 		}
// 		if toMP3 == "1" {
// 			mp.convertWithDecrypt(ctx.Writer, targetURL, decryptKey, 131072, filename)
// 			return
// 		}
// 		mp.decryptOnly(ctx.Writer, ctx.Request, targetURL, decryptKey, 131072, filename)
// 		return
// 	}
// 	mp.convertOnly(targetURL, ctx.Writer, filename, "mp3")
// 	c.jsonSuccess(ctx, nil)
// }

func (c *APIClient) handlePlay(ctx *gin.Context) {
	targetURL := ctx.Query("url")
	if targetURL == "" {
		c.jsonError(ctx, 400, "missing targetURL")
		return
	}
	if !strings.HasPrefix(targetURL, "http") {
		targetURL = "https://" + targetURL
	}
	if _, err := url.Parse(targetURL); err != nil {
		c.jsonError(ctx, 400, "Invalid URL")
		return
	}
	decryptKeyStr := ctx.Query("key")
	mp := NewChannelsVideoDecryptor()
	if decryptKeyStr != "" {
		decryptKey, err := strconv.ParseUint(decryptKeyStr, 0, 64)
		if err != nil {
			c.jsonError(ctx, 400, "invalid decryptKey")
			return
		}
		mp.decryptOnlyInline(ctx.Writer, ctx.Request, targetURL, decryptKey, 131072)
		return
	}
	mp.simpleProxy(targetURL, ctx.Writer, ctx.Request)
}

type OpenFolderAndHighlightFileBody struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// 在打开文件夹并选中指定文件
func (c *APIClient) handleHighlightFileInFolder(ctx *gin.Context) {
	var body OpenFolderAndHighlightFileBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.jsonError(ctx, 400, err.Error())
		return
	}
	if body.Path == "" || body.Name == "" {
		c.jsonError(ctx, 400, "Missing the `path` or `name`")
		return
	}
	full_filepath := filepath.Join(body.Path, body.Name)
	_, err := os.Stat(full_filepath)
	if err != nil {
		c.jsonError(ctx, 500, err.Error())
		return
	}
	if err := system.ShowInExplorer(full_filepath); err != nil {
		c.jsonError(ctx, 500, err.Error())
		return
	}
	c.jsonSuccess(ctx, nil)
	return
}
