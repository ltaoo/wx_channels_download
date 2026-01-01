package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
	gopeedhttp "github.com/GopeedLab/gopeed/pkg/protocol/http"
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
	resp, err := c.FetchChannelsFeedListOfContact(username)
	if err != nil {
		c.jsonError(ctx, 400, err.Error())
		return
	}
	c.jsonSuccess(ctx, resp)
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
