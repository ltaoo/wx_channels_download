package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	// "net/url"
	// "regexp"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/api/services"
	apitypes "wx_channel/internal/api/types"
	"wx_channel/internal/assets"
	"wx_channel/internal/database/model"
	downloaderclient "wx_channel/internal/downloader"
	"wx_channel/frontend"
	// "wx_channel/internal/officialaccount"
	"wx_channel/internal/manager"
	"wx_channel/internal/storage"
	"wx_channel/pkg/browsermgr"
	"wx_channel/pkg/decrypt"
	// "wx_channel/internal/webcontent/officialaccount"
	// wxchannels "wx_channel/internal/webcontent/wxchannels"
	// channels "wx_channel/pkg/scraper/wxchannels"
	"wx_channel/pkg/system"
	"wx_channel/pkg/util"
)

type APIClient struct {
	downloader    *downloadpkg.Downloader
	// official      *officialaccount.OfficialAccountClient
	// channels      *channels.ChannelsClient
	downloader_ws *downloaderclient.DownloaderClient
	status_ws     *downloaderclient.DownloaderClient
	filehelper    *FileHelperHandler
	formatter     *util.FilenameProcessor
	cfg           *APIConfig
	engine        *gin.Engine
	db            *gorm.DB
	logger        *zerolog.Logger
	httpHandler   http.Handler
	serviceMgr    *manager.ServerManager
	browserMgr    *browsermgr.Manager

	// Services
	downloadService       *services.DownloadService
	channelsService       *services.ChannelsService
	accountService        *services.AccountService
	contentService        *services.ContentService
	browseService         *services.BrowseService
	channelsUploadService *services.ChannelsUploadService

	// V1 native downloader (replaces gopeed for V1 tasks)
	v1Nat *v1NativeDownloader
}

func NewAPIClient(cfg *APIConfig, parent_logger *zerolog.Logger, db *gorm.DB) *APIClient {
	data_dir := cfg.WorkDir
	logger := parent_logger.With().Str("Client", "api_client").Logger()
	var st downloadpkg.Storage
	if db != nil {
		st = storage.NewSqliteStorage(db, &logger, cfg.DownloadDir)
	} else {
		st = downloadpkg.NewBoltStorage(data_dir)
	}
	downloader := downloadpkg.NewDownloader(&downloadpkg.DownloaderConfig{
		RefreshInterval: 360,
		Storage:         st,
		StorageDir:      data_dir,
	})
	// var channels_client *channels.ChannelsClient
	// official_cfg := officialaccount.NewOfficialAccountConfig(cfg.Original, cfg.RemoteServerMode)
	// officialaccount_client := officialaccount.NewOfficialAccountClient(official_cfg, parent_logger)
	// channels_client = channels.NewChannelsClient(cfg.ChannelsRefreshInterval)
	// if db != nil {
	// 	channels_client.SetDB(db)
	// }
	downloader_ws := downloaderclient.NewDownloaderClient()
	status_ws := downloaderclient.NewDownloaderClient()

	// get_sorted_tasks := func() []*downloadpkg.Task {
	// 	tasks := downloader.GetTasks()
	// 	sort.Slice(tasks, func(i, j int) bool {
	// 		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	// 	})
	// 	return tasks
	// }

	downloader_ws.OnConnected = func(client *downloaderclient.WSClient) {
		// all_tasks := get_sorted_tasks()
		// limit := 50
		// if limit > len(all_tasks) {
		// 	limit = len(all_tasks)
		// }
		// tasks := all_tasks[:limit]
		// if data, err := json.Marshal(APIClientWSMessage{Type: "tasks", Data: map[string]interface{}{
		// 	"list":  tasks,
		// 	"total": len(all_tasks),
		// }}); err == nil {
		// 	client.Send <- data
		// }
	}

	downloader_ws.OnMessage = func(client *downloaderclient.WSClient, message []byte) {
	}

	// Initialize services
	downloadService := services.NewDownloadService(downloader, db, cfg.DownloadDir, downloader_ws)
	// channelsService := services.NewChannelsService(channels_client)
	accountService := services.NewAccountService(db)
	contentService := services.NewContentService(db)
	browseService := services.NewBrowseService(db)
	channelsUploadService := services.NewChannelsUploadService(db, &logger)
	browserMgr, browserMgrErr := browsermgr.New(browsermgr.Config{
		WorkDir:          cfg.WorkDir,
		DockerImage:      cfg.BrowserDockerImage,
		DockerEntrypoint: cfg.BrowserDockerEntrypoint,
		DockerNetwork:    cfg.BrowserDockerNetwork,
		CDPPortMin:       cfg.BrowserCDPPortMin,
		CDPPortMax:       cfg.BrowserCDPPortMax,
		DesktopPortMin:   cfg.BrowserDesktopPortMin,
		DesktopPortMax:   cfg.BrowserDesktopPortMax,
		Resolution:       cfg.BrowserDesktopResolution,
		ShmSize:          cfg.BrowserDockerShmSize,
		MemoryLimit:      cfg.BrowserDockerMemoryLimit,
		ChromeCommand:    cfg.BrowserDockerChromeCommand,
	}, nil)
	if browserMgrErr != nil {
		logger.Warn().Err(browserMgrErr).Msg("初始化浏览器管理器失败")
	}

	apiClient := &APIClient{
		downloader:            downloader,
		downloader_ws:         downloader_ws,
		// official:              officialaccount_client,
		// channels:              channels_client,
		status_ws:             status_ws,
		// filehelper:            NewFileHelperHandler(),
		formatter:             util.NewFilenameProcessor(cfg.DownloadDir, make(map[string]int)),
		cfg:                   cfg,
		engine:                gin.Default(),
		db:                    db,
		logger:                &logger,
		browserMgr:            browserMgr,
		downloadService:       downloadService,
		// channelsService:       channelsService,
		accountService:        accountService,
		contentService:        contentService,
		browseService:         browseService,
		channelsUploadService: channelsUploadService,
	}

	apiClient.v1Nat = newV1NativeDownloader(&dbTaskStore{db: db},
		func(taskID int, event EventType) {
			fmt.Println("handle event", taskID)
			apiClient.broadcastTaskProgress(taskID)
		}, cfg.MaxRunning)

	status_ws.OnConnected = func(wsClient *downloaderclient.WSClient) {
		data, err := json.Marshal(APIClientWSMessage{
			Type: "channels_status",
			Data: apiClient.channelsStatusData(),
		})
		if err != nil {
			return
		}
		select {
		case wsClient.Send <- data:
		default:
		}
	}
	// channels_client.OnConnected = func(_ *channels.Client) {
	// 	status_ws.Broadcast(APIClientWSMessage{
	// 		Type: "channels_status",
	// 		Data: apiClient.channelsStatusData(),
	// 	})
	// }
	// channels_client.OnDisconnected = func(_ *channels.Client) {
	// 	status_ws.Broadcast(APIClientWSMessage{
	// 		Type: "channels_status",
	// 		Data: apiClient.channelsStatusData(),
	// 	})
	// }

	// // 设置文件传输助手视频号自动下载回调
	// apiClient.filehelper.SetFinderAutoDownloadCallback(apiClient.autoCreateChannelsTask)
	// // 设置文件传输助手 SPH 自动下载回调
	// apiClient.filehelper.SetSphAutoDownloadCallback(apiClient.autoDownloadSphVideo)

	apiClient.SetupRoutes()
	// apiClient.httpHandler = apiClient.buildHTTPHandler()
	return apiClient
}

func (c *APIClient) SetManager(mgr *manager.ServerManager) {
	c.serviceMgr = mgr
}

func (c *APIClient) downloadTaskWSEventData(evt *downloadpkg.Event) any {
	if evt == nil {
		return evt
	}
	errText := ""
	if evt.Err != nil {
		errText = evt.Err.Error()
	}
	data := gin.H{
		"Key":   evt.Key,
		"Task":  evt.Task,
		"Err":   errText,
		"error": errText,
	}
	if c.db != nil && evt.Task != nil && evt.Task.ID != "" {
		var rec model.DownloadTask
		if err := c.db.Where("task_id = ?", evt.Task.ID).First(&rec).Error; err == nil {
			data["download_task_id"] = rec.Id
		}
	}
	return data
}

type APIClientWSMessage struct {
	Type  string      `json:"type"`
	Data  interface{} `json:"data"`
	Error string      `json:"error"`
}

type ClientWSMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}
type ClientWebsocketRequestBody struct {
	ID   string      `json:"id"`
	Key  string      `json:"key"`
	Body interface{} `json:"data"`
}
type ClientWebsocketResponse struct {
	Id string `json:"id"`
	// 调用 wx api 原始响应
	Data json.RawMessage `json:"data"`
}

func (c *APIClient) Start() error {
	if err := c.downloader.Setup(); err != nil {
		return err
	}
	// c.loadPersistedPlatformWorkflowRuns()
	_ = c.downloader.PutConfig(&base.DownloaderStoreConfig{
		DownloadDir: c.cfg.DownloadDir,
		MaxRunning:  c.cfg.MaxRunning,
		ProtocolConfig: map[string]any{
			"http": map[string]any{
				"connections": 4,
			},
		},
		Extra: map[string]any{},
		Proxy: &base.DownloaderProxyConfig{},
	})
	c.downloader.Listener(func(evt *downloadpkg.Event) {
		if evt == nil || evt.Task == nil || evt.Task.ID == "" {
			return
		}
		var errMsg string
		if evt.Err != nil {
			errMsg = evt.Err.Error()
		}
		c.downloader_ws.Broadcast(APIClientWSMessage{
			Type: "event",
			Data: gin.H{
				"Key":           evt.Key,
				"Task":          evt.Task,
				"Err":           errMsg,
				// "status_counts": c.downloadTaskStatusCounts(),
			},
		})
		c.recordDownloadTaskEvent(evt)
		if evt.Key == downloadpkg.EventKeyDone {
			if c.cfg.PlayDoneAudio {
				go assets.PlayDoneAudio()
			}
			task := c.downloader.GetTask(evt.Task.ID)
			file_path := task.Meta.SingleFilepath()
			go func() {
				k := task.Meta.Req.Labels["key"]
				if k != "" {
					key, err := strconv.Atoi(k)
					if err == nil {
						if key != 0 {
							data, err := os.ReadFile(file_path)
							if err == nil {
								length := uint32(131072)
								_key := uint64(key)
								decrypt.DecryptData(data, length, _key)
								_ = os.WriteFile(file_path, data, 0644)
							}
						}
					}
				}
				suffix := task.Meta.Req.Labels["suffix"]
				if suffix == ".mp3" {
					temp_path := file_path + ".temp"
					if err := os.Rename(file_path, temp_path); err == nil {
						if err := system.RunCommand("ffmpeg", "-i", temp_path, "-vn", "-acodec", "libmp3lame", "-ab", "192k", "-f", "mp3", file_path); err == nil {
							_ = os.Remove(temp_path)
						} else {
							_ = os.Rename(temp_path, file_path)
						}
					}
				}
			}()
		}
	})
	return nil
}

func (c *APIClient) Stop() error {
	if c.downloader != nil {
		// 暂停所有 gopeed 下载任务（旧系统）
		c.downloader.Pause(nil)
		// 等待 listener 处理完 pause 事件
		time.Sleep(100 * time.Millisecond)
	}
	// 暂停所有 V1 原生下载任务
	if c.v1Nat != nil {
		c.v1Nat.PauseAll()
	}
	// 直接更新 V1 数据库：将进行中的下载任务置为暂停状态
	if c.db != nil {
		now := time.Now().UnixMilli()
		c.db.Model(&model.DownloadTaskV1{}).
			Where("status = ? AND deleted_at IS NULL", model.TaskStatusDownloading).
			Updates(map[string]any{"status": model.TaskStatusPaused, "updated_at": now})
		c.db.Model(&model.DownloadConnection{}).
			Where("status = 1 AND deleted_at IS NULL").
			Updates(map[string]any{"status": 0, "speed": 0, "last_active": now, "updated_at": now})
	}
	// if c.channels != nil {
	// 	c.channels.Stop()
	// }
	if c.downloader_ws != nil {
		c.downloader_ws.Stop()
	}
	if c.status_ws != nil {
		c.status_ws.Stop()
	}
	return nil
}

func (c *APIClient) Engine() *gin.Engine {
	return c.engine
}

func (c *APIClient) HTTPHandler() http.Handler {
	return withCORS(c)
}

func (c *APIClient) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if c.httpHandler == nil {
		c.httpHandler = c.buildHTTPHandler()
	}
	c.httpHandler.ServeHTTP(w, r)
}

func (c *APIClient) recordDownloadTaskEvent(evt *downloadpkg.Event) {
	if c.db == nil || evt == nil || evt.Task == nil || evt.Task.ID == "" {
		return
	}
	if evt.Key == downloadpkg.EventKeyProgress {
		return
	}

	var rec model.DownloadTask
	if err := c.db.Where("task_id = ?", evt.Task.ID).First(&rec).Error; err != nil {
		return
	}

	message := ""
	if evt.Err != nil {
		message = evt.Err.Error()
	}
	if evt.Key == downloadpkg.EventKeyError && message != "" {
		outputPath := ""
		if evt.Task.Meta != nil {
			outputPath = evt.Task.Meta.SingleFilepath()
		}
		_ = c.db.Model(&model.DownloadTask{}).
			Where("task_id = ?", evt.Task.ID).
			Updates(map[string]any{
				"status":      5,
				"error":       message,
				"output_path": outputPath,
				"updated_at":  util.NowMillis(),
			}).Error
	}
	data := map[string]any{
		"task_id": evt.Task.ID,
		"status":  string(evt.Task.Status),
		"error":   message,
	}
	if evt.Task.Meta != nil && evt.Task.Meta.Opts != nil {
		data["name"] = evt.Task.Meta.Opts.Name
		data["path"] = evt.Task.Meta.Opts.Path
	}
	if evt.Task.Meta != nil && evt.Task.Meta.Req != nil && evt.Task.Meta.Req.Labels != nil {
		data["labels"] = evt.Task.Meta.Req.Labels
	}
	dataBytes, _ := json.Marshal(data)

	_ = c.db.Create(&model.DownloadTaskEvent{
		TaskId:    rec.Id,
		Type:      string(evt.Key),
		Message:   message,
		Data:      string(dataBytes),
		CreatedAt: util.NowMillis(),
	}).Error
}

func (c *APIClient) ensureDownloadTaskBaselineEvents(tasks []model.DownloadTask) {
	if c.db == nil || len(tasks) == 0 {
		return
	}
	for _, task := range tasks {
		if task.Id == 0 {
			continue
		}
		var count int64
		if err := c.db.Model(&model.DownloadTaskEvent{}).Where("task_id = ?", task.Id).Count(&count).Error; err != nil || count > 0 {
			continue
		}
		createdAt := task.CreatedAt
		if createdAt == 0 {
			createdAt = task.UpdatedAt
		}
		if createdAt == 0 {
			createdAt = util.NowMillis()
		}
		_ = c.db.Create(&model.DownloadTaskEvent{
			TaskId:    task.Id,
			Type:      "create",
			Message:   "创建下载任务",
			CreatedAt: createdAt,
		}).Error

		statusEvent := downloadTaskStatusEventType(task.Status)
		if statusEvent == "" {
			continue
		}
		statusAt := task.UpdatedAt
		if statusAt == 0 {
			statusAt = createdAt
		}
		message := ""
		if statusEvent == "error" {
			message = task.Error
		}
		_ = c.db.Create(&model.DownloadTaskEvent{
			TaskId:    task.Id,
			Type:      statusEvent,
			Message:   message,
			CreatedAt: statusAt,
		}).Error
	}
}

func downloadTaskStatusEventType(status int) string {
	switch status {
	case 1:
		return "start"
	case 2:
		return "pause"
	case 4:
		return "done"
	case 5:
		return "error"
	default:
		return ""
	}
}

func (c *APIClient) setupStaticAssetRoutes() {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		c.engine.Handle(method, "/__wx_channels_assets/lib/*filepath", c.handleChannelLibAsset)
		c.engine.Handle(method, "/__wx_channels_assets/src/*filepath", c.handleChannelSrcAsset)
		c.engine.Handle(method, "/__wx_channels_assets/inject/*filepath", c.handleChannelInjectAsset)
	}
}

func (c *APIClient) handleChannelLibAsset(ctx *gin.Context) {
	rel := ctx.Param("filepath")
	data, err := frontend.Assets.ReadLib(rel)
	if err != nil {
		ctx.Status(http.StatusNotFound)
		return
	}
	data = frontend.ChannelStaticAssetResponseData(rel, data)
	ctx.Header("Content-Type", frontend.ChannelStaticAssetContentType(rel))
	ctx.Header("Cache-Control", frontend.ChannelLibAssetCacheControl)
	if ctx.Request.Method == http.MethodHead {
		ctx.Status(http.StatusOK)
		return
	}
	ctx.Data(http.StatusOK, frontend.ChannelStaticAssetContentType(rel), data)
}

func (c *APIClient) handleChannelInjectAsset(ctx *gin.Context) {
	rel := ctx.Param("filepath")
	data, err := frontend.Assets.ReadInject(rel)
	if err != nil {
		ctx.Status(http.StatusNotFound)
		return
	}
	etag := frontend.ChannelStaticAssetETag(data)
	ctx.Header("Content-Type", frontend.ChannelStaticAssetContentType(rel))
	ctx.Header("Cache-Control", frontend.ChannelSrcAssetCacheControl)
	ctx.Header("ETag", etag)
	if strings.Contains(ctx.GetHeader("If-None-Match"), etag) {
		ctx.Status(http.StatusNotModified)
		return
	}
	if ctx.Request.Method == http.MethodHead {
		ctx.Status(http.StatusOK)
		return
	}
	ctx.Data(http.StatusOK, frontend.ChannelStaticAssetContentType(rel), data)
}

func (c *APIClient) handleChannelSrcAsset(ctx *gin.Context) {
	rel := ctx.Param("filepath")
	data, err := frontend.Assets.ReadSrc(rel)
	if err != nil {
		ctx.Status(http.StatusNotFound)
		return
	}
	etag := frontend.ChannelStaticAssetETag(data)
	ctx.Header("Content-Type", frontend.ChannelStaticAssetContentType(rel))
	ctx.Header("Cache-Control", frontend.ChannelSrcAssetCacheControl)
	ctx.Header("ETag", etag)
	if strings.Contains(ctx.GetHeader("If-None-Match"), etag) {
		ctx.Status(http.StatusNotModified)
		return
	}
	if ctx.Request.Method == http.MethodHead {
		ctx.Status(http.StatusOK)
		return
	}
	ctx.Data(http.StatusOK, frontend.ChannelStaticAssetContentType(rel), data)
}

func (c *APIClient) resolve_connections(url string) int {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Head(url)
	if err != nil {
		return 1
	}
	defer resp.Body.Close()

	if resp.ContentLength > 0 && resp.ContentLength < 1024*1024 {
		return 1
	}
	return 4
}

func (c *APIClient) check_existing_feed(tasks []*downloadpkg.Task, body *services.FeedDownloadTaskBody) bool {
	return len(c.find_existing_feed_tasks(tasks, body)) > 0
}

func (c *APIClient) find_existing_feed_tasks(tasks []*downloadpkg.Task, body *services.FeedDownloadTaskBody) []*downloadpkg.Task {
	var matches []*downloadpkg.Task
	for _, t := range tasks {
		if t == nil || t.Meta == nil || t.Meta.Req == nil || t.Meta.Req.Labels == nil {
			continue
		}
		same_id := t.Meta.Req.Labels["id"] == body.Id
		same_spec := t.Meta.Req.Labels["spec"] == body.Spec
		same_suffix := t.Meta.Req.Labels["suffix"] == body.Suffix
		if same_id && same_spec && same_suffix {
			matches = append(matches, t)
		}
	}
	return matches
}

func (c *APIClient) createFeedTaskBody(oid, nid, reqUrl, eid string, isMp3, isCover bool, customSpec ...string) (*services.FeedDownloadTaskBody, *model.Content, *model.Account, error) {
	return nil, nil, nil, fmt.Errorf("need to process")
	// 获取视频详情
	// r, err := c.channels.FetchChannelsFeedProfile(oid, nid, reqUrl, eid)
	// if err != nil {
	// 	return nil, nil, nil, fmt.Errorf("获取详情失败: %w", err)
	// }
	// if r.ErrCode != 0 {
	// 	return nil, nil, nil, fmt.Errorf("获取详情失败: %s", r.ErrMsg)
	// }

	// feed := r.Data.Object
	// if feed.LiveInfo != nil {
	// 	return nil, nil, nil, fmt.Errorf("直播类型请使用直播下载")
	// }

	// isPicture := feed.Type == "picture" || feed.ObjectDesc.MediaType == 2
	// var media *channels.ChannelsMediaItem
	// if !isPicture {
	// 	if len(feed.ObjectDesc.Media) == 0 {
	// 		return nil, nil, nil, fmt.Errorf("缺少可下载的视频内容")
	// 	}
	// 	media = &feed.ObjectDesc.Media[0]
	// }

	// key := 0
	// if media != nil && media.DecodeKey != "" {
	// 	k, err := strconv.Atoi(media.DecodeKey)
	// 	if err != nil {
	// 		return nil, nil, nil, fmt.Errorf("解析 DecodeKey 失败: %w", err)
	// 	}
	// 	key = k
	// }

	// spec := ""
	// if len(customSpec) > 0 && customSpec[0] != "" {
	// 	spec = customSpec[0]
	// } else if !c.cfg.Original.GetBool("download.defaultHighest") {
	// 	if media != nil && len(media.Spec) > 0 {
	// 		spec = media.Spec[0].FileFormat
	// 	}
	// }

	// title := wxchannels.ObjectTitle(&feed)
	// defaultName := title
	// template := c.cfg.Original.GetString("download.filenameTemplate")
	// filename := defaultName
	// if template != "" {
	// 	params := map[string]string{
	// 		"filename":    defaultName,
	// 		"id":          feed.ID,
	// 		"title":       title,
	// 		"spec":        spec,
	// 		"created_at":  strconv.Itoa(feed.CreateTime),
	// 		"download_at": util.NowSecondsStr(),
	// 		"author":      feed.Contact.Nickname,
	// 	}
	// 	re := regexp.MustCompile(`\{\{(\w+)\}\}`)
	// 	filename = re.ReplaceAllStringFunc(template, func(m string) string {
	// 		key := m[2 : len(m)-2]
	// 		if v, ok := params[key]; ok {
	// 			return v
	// 		}
	// 		return ""
	// 	})
	// }

	// downloadURL := wxchannels.ObjectURL(&feed)
	// coverURL := ""
	// if len(feed.ObjectDesc.Media) > 0 {
	// 	coverURL = feed.ObjectDesc.Media[0].CoverUrl
	// }
	// if isCover {
	// 	downloadURL = coverURL
	// }

	// payload := &services.FeedDownloadTaskBody{
	// 	Id:       feed.ID,
	// 	NonceId:  feed.ObjectNonceId,
	// 	Title:    title,
	// 	Key:      key,
	// 	Spec:     spec,
	// 	Suffix:   ".mp4",
	// 	URL:      downloadURL,
	// 	Filename: filename,
	// }
	// if payload.NonceId == "" {
	// 	payload.NonceId = nid
	// }

	// // 处理 URL：非空 spec 添加 X-snsvideoflag 参数，空 spec 则清理 URL 只保留 encfilekey 和 token
	// if !isCover && !isPicture {
	// 	if spec != "" {
	// 		payload.URL += "&X-snsvideoflag=" + spec
	// 	} else {
	// 		if u, err := url.Parse(payload.URL); err == nil {
	// 			filekey := u.Query().Get("encfilekey")
	// 			token := u.Query().Get("token")
	// 			if filekey != "" && token != "" {
	// 				newURL := u.Scheme + "://" + u.Host + u.Path
	// 				newURL += "?encfilekey=" + filekey + "&token=" + token
	// 				payload.URL = newURL
	// 			}
	// 		}
	// 	}
	// }

	// if isMp3 {
	// 	payload.Suffix = ".mp3"
	// }
	// if isCover {
	// 	payload.Suffix = ".jpg"
	// }

	// // 处理图集类型
	// if isPicture {
	// 	payload.Suffix = ".zip"
	// 	payload.URL = ""
	// 	var files []map[string]string
	// 	pictureFiles := feed.Files
	// 	if len(pictureFiles) == 0 {
	// 		pictureFiles = feed.ObjectDesc.Media
	// 	}
	// 	for i, m := range pictureFiles {
	// 		files = append(files, map[string]string{
	// 			"url":      m.URL + m.URLToken,
	// 			"filename": fmt.Sprintf("%d.jpg", i+1),
	// 		})
	// 	}
	// 	if bgmURL := feed.ObjectDesc.FollowPostInfo.MusicInfo.MediaStreamingUrl; bgmURL != "" {
	// 		files = append(files, map[string]string{
	// 			"url":      bgmURL,
	// 			"filename": "bgm.mp3",
	// 		})
	// 	}
	// 	if len(files) == 0 {
	// 		return nil, nil, nil, fmt.Errorf("图集类型缺少可下载图片")
	// 	}
	// 	data, _ := json.Marshal(files)
	// 	payload.URL = fmt.Sprintf("zip://weixin.qq.com?files=%s", url.QueryEscape(string(data)))
	// }

	// content, err := wxchannels.ToContent(&feed)
	// if err != nil {
	// 	return nil, nil, nil, fmt.Errorf("转换 content 失败: %w", err)
	// }
	// account, err := wxchannels.ToAccount(&feed)
	// if err != nil {
	// 	return nil, nil, nil, fmt.Errorf("转换 account 失败: %w", err)
	// }

	// return payload, content, account, nil
}

// autoCreateChannelsTask 根据视频号消息自动创建下载任务
func (c *APIClient) autoCreateChannelsTask(objectID, objectNonceID string) error {
	// c.logger.Info().
	// 	Str("objectID", objectID).
	// 	Str("objectNonceID", objectNonceID).
	// 	Msg("收到视频号消息，开始自动创建下载任务")

	// // 获取视频详情
	// r, err := c.channels.FetchChannelsFeedProfile(objectID, objectNonceID, "", "")
	// if err != nil {
	// 	errMsg := fmt.Sprintf("✗ 视频号下载失败: %s", err.Error())
	// 	c.logger.Error().Err(err).Msg("获取详情失败")
	// 	c.sendMessageToFilehelper(errMsg)
	// 	return err
	// }
	// if r.ErrCode != 0 {
	// 	errMsg := fmt.Sprintf("✗ 视频号下载失败: %s", r.ErrMsg)
	// 	c.logger.Error().Msgf("获取详情失败: %s", r.ErrMsg)
	// 	c.sendMessageToFilehelper(errMsg)
	// 	return fmt.Errorf("获取详情失败: %s", r.ErrMsg)
	// }
	// if len(r.Data.Object.ObjectDesc.Media) == 0 {
	// 	errMsg := "✗ 视频号下载失败: 缺少可下载的视频内容"
	// 	c.logger.Error().Msg("缺少可下载的视频内容")
	// 	c.sendMessageToFilehelper(errMsg)
	// 	return fmt.Errorf("缺少可下载的视频内容")
	// }

	// object, err := convertAPIChannelsObject(r.Data.Object)
	// if err != nil {
	// 	errMsg := fmt.Sprintf("✗ 视频号下载失败: %s", err.Error())
	// 	c.logger.Error().Err(err).Msg("转换视频号对象失败")
	// 	c.sendMessageToFilehelper(errMsg)
	// 	return err
	// }

	// // 构建请求，发送 ChannelsObject（后端处理全部逻辑）
	// req := ChannelsDownloadRequest{
	// 	Object: object,
	// 	Spec:   "",
	// 	Suffix: "",
	// }

	// // 发送创建任务请求
	// var targetURL string
	// if c.cfg.RemoteServerEnabled {
	// 	protocol := c.cfg.RemoteServerProtocol
	// 	if protocol == "" {
	// 		protocol = "http"
	// 	}
	// 	targetURL = fmt.Sprintf("%s://%s:%d/api/task/create", protocol, c.cfg.RemoteServerHostname, c.cfg.RemoteServerPort)
	// } else {
	// 	protocol := c.cfg.Protocol
	// 	if protocol == "" {
	// 		protocol = "http"
	// 	}
	// 	hostname := c.cfg.Hostname
	// 	if hostname == "0.0.0.0" {
	// 		hostname = "127.0.0.1"
	// 	}
	// 	targetURL = fmt.Sprintf("%s://%s:%d/api/task/create", protocol, hostname, c.cfg.Port)
	// }

	// jsonData, err := json.Marshal(req)
	// if err != nil {
	// 	errMsg := fmt.Sprintf("✗ 视频号下载失败: %s", err.Error())
	// 	c.logger.Error().Err(err).Msg("序列化请求参数失败")
	// 	c.sendMessageToFilehelper(errMsg)
	// 	return fmt.Errorf("序列化请求参数失败: %w", err)
	// }

	// resp, err := http.Post(targetURL, "application/json", bytes.NewBuffer(jsonData))
	// if err != nil {
	// 	errMsg := fmt.Sprintf("✗ 视频号下载失败: %s", err.Error())
	// 	c.logger.Error().Err(err).Str("url", targetURL).Msg("请求创建任务接口失败")
	// 	c.sendMessageToFilehelper(errMsg)
	// 	return fmt.Errorf("请求创建任务接口失败: %w", err)
	// }
	// defer resp.Body.Close()

	// if resp.StatusCode != http.StatusOK {
	// 	bodyBytes, _ := io.ReadAll(resp.Body)
	// 	errMsg := fmt.Sprintf("✗ 视频号下载失败: %s", string(bodyBytes))
	// 	c.logger.Error().Int("status", resp.StatusCode).Str("body", string(bodyBytes)).Msg("创建任务失败")
	// 	c.sendMessageToFilehelper(errMsg)
	// 	return fmt.Errorf("创建任务失败, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	// }

	// c.logger.Info().
	// 	Str("url", targetURL).
	// 	Msg("自动创建下载任务请求发送成功")

	// // 发送成功消息
	// successMsg := fmt.Sprintf("✓ 视频号已开始下载: %s", r.Data.Object.ObjectDesc.Description)
	// c.sendMessageToFilehelper(successMsg)

	// return nil
	return nil
}

func convertAPIChannelsObject(obj interface{}) (apitypes.ChannelsObject, error) {
	var converted apitypes.ChannelsObject
	data, err := json.Marshal(obj)
	if err != nil {
		return converted, fmt.Errorf("序列化视频号对象失败: %w", err)
	}
	if err := json.Unmarshal(data, &converted); err != nil {
		return converted, fmt.Errorf("解析视频号对象失败: %w", err)
	}
	return converted, nil
}

// sendMessageToFilehelper 发送消息到 filehelper
func (c *APIClient) sendMessageToFilehelper(msg string) {
	go func() {
		fhClient := c.filehelper.GetClient()
		if err := fhClient.SendText(msg); err != nil {
			c.logger.Warn().Err(err).Str("msg", msg).Msg("发送消息失败")
		} else {
			c.logger.Info().Str("msg", msg).Msg("消息发送成功")
		}
	}()
}

// autoDownloadSphVideo 从 SPH URL 自动下载视频
func (c *APIClient) autoDownloadSphVideo(sphUrl string) error {
	c.logger.Info().
		Str("url", sphUrl).
		Msg("收到 SPH URL，开始获取视频信息")

	// 从配置获取 cookie
	cookie := c.cfg.CloudflareSphCookie
	if cookie == "" {
		c.logger.Error().Msg("cloudflare.sphCookie not configured")
		c.sendMessageToFilehelper("✗ SPH 下载失败: 未配置 cookie")
		return fmt.Errorf("cloudflare.sphCookie not configured")
	}

	// 获取视频信息
	feedResp, err := fetchVideoProfileWithShareUrl(sphUrl, cookie)
	if err != nil {
		c.logger.Error().Err(err).Msg("获取视频信息失败")
		c.sendMessageToFilehelper(fmt.Sprintf("✗ SPH 下载失败: %s", err.Error()))
		return fmt.Errorf("获取视频信息失败: %w", err)
	}

	// 处理 video URL：仅保留 encfilekey 和 token 参数，存储为 originVideoUrl
	if feedResp != nil && feedResp.Data.Feedinfo.Videourl != "" {
		feedResp.Data.Feedinfo.OriginVideoUrl = cleanVideoURL(feedResp.Data.Feedinfo.Videourl)
	}

	if feedResp == nil || feedResp.Data.Feedinfo.OriginVideoUrl == "" {
		c.logger.Error().Msg("获取 originVideoUrl 失败")
		c.sendMessageToFilehelper("✗ SPH 下载失败: 无法获取视频链接")
		return fmt.Errorf("获取 originVideoUrl 失败")
	}

	// 构建下载任务
	downloadUrl := feedResp.Data.Feedinfo.OriginVideoUrl
	filename := feedResp.Data.Feedinfo.Description
	if filename == "" {
		filename = feedResp.Data.Authorinfo.Nickname + "_" + util.NowSecondsStr()
	}
	// 添加 .mp4 后缀
	if !strings.HasSuffix(filename, ".mp4") {
		filename += ".mp4"
	}

	// 创建下载任务
	taskBody := &DownloadTaskPayload{
		URL:      downloadUrl,
		Filename: filename,
		Dir:      "",
		Extra:    make(map[string]string),
	}

	// 发送创建任务请求
	var targetURL string
	if c.cfg.RemoteServerEnabled {
		protocol := c.cfg.RemoteServerProtocol
		if protocol == "" {
			protocol = "http"
		}
		targetURL = fmt.Sprintf("%s://%s:%d/api/task/create2", protocol, c.cfg.RemoteServerHostname, c.cfg.RemoteServerPort)
	} else {
		protocol := c.cfg.Protocol
		if protocol == "" {
			protocol = "http"
		}
		hostname := c.cfg.Hostname
		if hostname == "0.0.0.0" {
			hostname = "127.0.0.1"
		}
		targetURL = fmt.Sprintf("%s://%s:%d/api/task/create2", protocol, hostname, c.cfg.Port)
	}

	jsonData, err := json.Marshal(taskBody)
	if err != nil {
		c.logger.Error().Err(err).Msg("序列化请求参数失败")
		c.sendMessageToFilehelper(fmt.Sprintf("✗ SPH 下载失败: %s", err.Error()))
		return fmt.Errorf("序列化请求参数失败: %w", err)
	}

	resp, err := http.Post(targetURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		c.logger.Error().Err(err).Str("url", targetURL).Msg("请求创建任务接口失败")
		c.sendMessageToFilehelper(fmt.Sprintf("✗ SPH 下载失败: %s", err.Error()))
		return fmt.Errorf("请求创建任务接口失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		c.logger.Error().Int("status", resp.StatusCode).Str("body", string(bodyBytes)).Msg("创建任务失败")
		c.sendMessageToFilehelper(fmt.Sprintf("✗ SPH 下载失败: 创建任务失败"))
		return fmt.Errorf("创建任务失败, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	c.logger.Info().
		Str("url", targetURL).
		Str("filename", filename).
		Msg("自动创建 SPH 下载任务请求发送成功")

	// 发送下载开始的消息到 filehelper
	successMsg := fmt.Sprintf("✓ SPH 已开始下载: %s", filename)
	c.sendMessageToFilehelper(successMsg)

	return nil
}

// ---------------------------------------------------------------------------
// dbTaskStore — DownloadTaskStore 的 gorm 数据库实现
// ---------------------------------------------------------------------------

type dbTaskStore struct {
	db *gorm.DB
}

func (s *dbTaskStore) LoadTask(taskID int) (*TaskInfo, error) {
	var task model.DownloadTaskV1
	if err := s.db.Where("id = ?", taskID).First(&task).Error; err != nil {
		return nil, err
	}
	var ep model.DownloadEndpoint
	if err := s.db.Table("download_endpoint").
		Where("resource_id IN (SELECT id FROM download_resource WHERE task_id = ?)", task.Id).
		Order("priority ASC").First(&ep).Error; err != nil {
		return nil, err
	}
	return &TaskInfo{
		ID:         task.Id,
		Name:       task.Name,
		SavePath:   task.SavePath,
		URL:        ep.URL,
		ResourceID: ep.ResourceId,
	}, nil
}

func (s *dbTaskStore) UpdateStatus(taskID int, status int) error {
	now := time.Now().UnixMilli()
	return s.db.Model(&model.DownloadTaskV1{}).Where("id = ?", taskID).
		Updates(map[string]any{"status": status, "updated_at": now}).Error
}

func (s *dbTaskStore) ActivateTask(taskID int) error {
	now := time.Now().UnixMilli()
	s.db.Model(&model.DownloadResource{}).
		Where("task_id = ? AND status IN (0,1)", taskID).
		Updates(map[string]any{"status": 1, "updated_at": now})
	s.db.Model(&model.DownloadEndpoint{}).
		Where("resource_id IN (SELECT id FROM download_resource WHERE task_id = ?)", taskID).
		Updates(map[string]any{"status": 1, "updated_at": now})
	s.db.Model(&model.DownloadConnection{}).
		Where("endpoint_id IN (SELECT id FROM download_endpoint WHERE resource_id IN (SELECT id FROM download_resource WHERE task_id = ?))", taskID).
		Updates(map[string]any{"status": 1, "last_active": now, "updated_at": now})
	return nil
}

func (s *dbTaskStore) UpdateProgress(taskID int, downloaded int64, speed int64) error {
	now := time.Now().UnixMilli()
	s.db.Exec(`UPDATE download_segment SET downloaded = ?, updated_at = ?
		WHERE resource_id IN (SELECT id FROM download_resource WHERE task_id = ?)`,
		downloaded, now, taskID)
	s.db.Exec(`UPDATE download_connection SET speed = ?, bytes = ?, last_active = ?, updated_at = ?
		WHERE endpoint_id IN (
			SELECT id FROM download_endpoint WHERE resource_id IN (
				SELECT id FROM download_resource WHERE task_id = ?
			)
		)`, speed, downloaded, now, now, taskID)
	s.db.Exec(`UPDATE download_resource SET status = 1, updated_at = ? WHERE task_id = ? AND status IN (0,1)`,
		now, taskID)
	return nil
}

func (s *dbTaskStore) UpdateResourceSize(taskID int, size int64) error {
	now := time.Now().UnixMilli()
	s.db.Exec(`UPDATE download_resource SET size = ?, updated_at = ? WHERE task_id = ? AND size = 0`,
		size, now, taskID)
	s.db.Exec(`UPDATE download_segment SET size = ?, offset_end = ?, updated_at = ?
		WHERE resource_id IN (SELECT id FROM download_resource WHERE task_id = ?)`,
		size, size-1, now, taskID)
	return nil
}

func (s *dbTaskStore) DeactivateConnections(taskID int) error {
	now := time.Now().UnixMilli()
	return s.db.Exec(`UPDATE download_connection SET speed = 0, status = 2, updated_at = ?
		WHERE endpoint_id IN (
			SELECT id FROM download_endpoint WHERE resource_id IN (
				SELECT id FROM download_resource WHERE task_id = ?
			)
		)`, now, taskID).Error
}

func (s *dbTaskStore) FinishTask(taskID int) error {
	now := time.Now().UnixMilli()
	s.db.Model(&model.DownloadTaskV1{}).Where("id = ?", taskID).
		Updates(map[string]any{"status": model.TaskStatusFinished, "updated_at": now})
	s.db.Exec(`UPDATE download_segment SET downloaded = size, status = 2, updated_at = ?
		WHERE resource_id IN (SELECT id FROM download_resource WHERE task_id = ?)`, now, taskID)
	s.db.Exec(`UPDATE download_connection SET speed = 0, status = 2, updated_at = ?
		WHERE endpoint_id IN (
			SELECT id FROM download_endpoint WHERE resource_id IN (
				SELECT id FROM download_resource WHERE task_id = ?
			)
		)`, now, taskID)
	s.db.Exec(`UPDATE download_resource SET status = 2, updated_at = ? WHERE task_id = ?`, now, taskID)
	return nil
}

func (s *dbTaskStore) WriteLog(taskID int, level string, message string) error {
	now := time.Now().UnixMilli()
	return s.db.Create(&model.DownloadLog{
		TaskId:    taskID,
		Level:     level,
		Message:   message,
		CreatedAt: now,
	}).Error
}

func (s *dbTaskStore) CreateSegments(resourceID int, url string, ranges []SegmentRange) ([]int, error) {
	now := time.Now().UnixMilli()
	var ids []int
	for _, r := range ranges {
		seg := model.DownloadSegment{
			ResourceId:  resourceID,
			Index:       r.Index,
			URL:         url,
			OffsetStart: r.OffsetStart,
			OffsetEnd:   r.OffsetEnd,
			Size:        r.Size,
			Downloaded:  0,
			Status:      1,
		}
		seg.CreatedAt = now
		seg.UpdatedAt = now
		if err := s.db.Create(&seg).Error; err != nil {
			return nil, err
		}
		ids = append(ids, seg.Id)
	}
	return ids, nil
}

func (s *dbTaskStore) LoadSegmentInfo(resourceID int) ([]segmentInfo, error) {
	var segs []model.DownloadSegment
	if err := s.db.Where("resource_id = ?", resourceID).Order("`index` ASC").Find(&segs).Error; err != nil {
		return nil, err
	}
	infos := make([]segmentInfo, len(segs))
	for i, s := range segs {
		infos[i] = segmentInfo{
			ID:          s.Id,
			Index:       s.Index,
			URL:         s.URL,
			OffsetStart: s.OffsetStart,
			OffsetEnd:   s.OffsetEnd,
			Size:        s.Size,
			Downloaded:  s.Downloaded,
		}
	}
	return infos, nil
}

func (c *APIClient) CreateContentDownloadTask(content *model.Content, t *downloadpkg.Task, reason string) (*model.DownloadTask, error) {
	if content == nil {
		return nil, errors.New("content is nil")
	}
	if t == nil {
		return nil, errors.New("download task is nil")
	}
	if c.db == nil {
		return nil, errors.New("db is nil")
	}

	db := c.db

	title := ""
	if t.Meta != nil && t.Meta.Opts != nil {
		title = strings.TrimSpace(t.Meta.Opts.Name)
	}
	if title == "" {
		title = strings.TrimSpace(content.Title)
	}

	taskURL := strings.TrimSpace(content.ContentURL)
	if taskURL == "" {
		taskURL = strings.TrimSpace(content.URL)
	}
	if taskURL == "" && t.Meta != nil && t.Meta.Req != nil {
		taskURL = strings.TrimSpace(t.Meta.Req.URL)
	}

	size := content.Size
	if size <= 0 {
		size = content.FileSize
	}

	meta2Bytes, _ := json.Marshal(map[string]any{
		"platform":    content.PlatformId,
		"external_id": content.ExternalId,
		"nonce_id":    content.ExternalId2,
		"eid":         "",
		"source_url":  content.SourceURL,
		"url":         content.URL,
		"content_url": content.ContentURL,
	})

	statusToInt := func(s base.Status) int {
		switch s {
		case base.DownloadStatusReady:
			return 0
		case base.DownloadStatusRunning:
			return 1
		case base.DownloadStatusPause:
			return 2
		case base.DownloadStatusWait:
			return 3
		case base.DownloadStatusDone:
			return 4
		case base.DownloadStatusError:
			return 5
		default:
			return 0
		}
	}

	var rec model.DownloadTask
	err := db.Where("task_id = ?", t.ID).First(&rec).Error
	updates := map[string]any{
		"url":         taskURL,
		"external_id": content.ExternalId,
		"title":       title,
		"cover_url":   content.CoverURL,
		"metadata2":   string(meta2Bytes),
		"reason":      reason,
		"updated_at":  util.TimeToMillisInt64(t.UpdatedAt),
	}
	outputPath := ""
	if t.Meta != nil {
		outputPath = t.Meta.SingleFilepath()
		updates["output_path"] = outputPath
	}
	if size > 0 {
		updates["size"] = size
	}

	if err == nil {
		if err := db.Model(&model.DownloadTask{}).Where("id = ?", rec.Id).Updates(updates).Error; err != nil {
			return nil, err
		}
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		rec = model.DownloadTask{
			TaskId:     t.ID,
			Status:     statusToInt(t.Status),
			Protocol:   t.Protocol,
			URL:        taskURL,
			ExternalId: content.ExternalId,
			Title:      title,
			CoverURL:   content.CoverURL,
			Size:       size,
			OutputPath: outputPath,
			Reason:     reason,
			Metadata2:  string(meta2Bytes),
			Timestamps: model.Timestamps{
				CreatedAt: util.TimeToMillisInt64(t.CreatedAt),
				UpdatedAt: util.TimeToMillisInt64(t.UpdatedAt),
			},
		}
		if err := db.Create(&rec).Error; err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}

	now := util.NowMillis()
	downloadPath := rec.OutputPath
	if downloadPath == "" {
		downloadPath = rec.Filepath
	}
	if err := db.Model(&model.Content{}).Where("id = ?", content.Id).Updates(map[string]any{
		"download_task_id": rec.Id,
		"download_status":  rec.Status,
		"download_path":    downloadPath,
		"updated_at":       now,
	}).Error; err != nil {
		return &rec, err
	}

	return &rec, nil
}


