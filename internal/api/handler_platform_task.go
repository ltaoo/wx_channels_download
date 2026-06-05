package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/database/model"
	result "wx_channel/internal/util"
	contentshuba69 "wx_channel/pkg/contentplatform/69shuba"
	contentchannels "wx_channel/pkg/contentplatform/channels"
	contentdouyin "wx_channel/pkg/contentplatform/douyin"
	contentdownload "wx_channel/pkg/contentplatform/download"
	contentfanqie "wx_channel/pkg/contentplatform/fanqienovel"
	contentoa "wx_channel/pkg/contentplatform/officialaccount"
	contentqidian "wx_channel/pkg/contentplatform/qidian"
	contentquanben "wx_channel/pkg/contentplatform/quanben"
	contentttk "wx_channel/pkg/contentplatform/ttk"
	contentyoutube "wx_channel/pkg/contentplatform/youtube"
	contentzhihu "wx_channel/pkg/contentplatform/zhihu"
	"wx_channel/pkg/decrypt"
	"wx_channel/pkg/system"
	"wx_channel/pkg/util"
)

type platformCreateTaskBody struct {
	URL       string                  `json:"url"`
	RunID     string                  `json:"run_id"`
	ProbeID   string                  `json:"probe_id"`
	VariantID string                  `json:"variant_id"`
	Spec      string                  `json:"spec"`
	Suffix    string                  `json:"suffix"`
	Filename  string                  `json:"filename"`
	Cover     bool                    `json:"cover"`
	Options   contentdownload.Options `json:"options"`
	Extra     map[string]any          `json:"extra"`
}

type platformWorkflowNode struct {
	ID          string                       `json:"id"`
	Type        string                       `json:"type"`
	Status      string                       `json:"status"`
	Error       string                       `json:"error,omitempty"`
	Output      map[string]any               `json:"output,omitempty"`
	Interaction *platformWorkflowInteraction `json:"interaction,omitempty"`
	StartedAt   time.Time                    `json:"started_at,omitempty"`
	EndedAt     time.Time                    `json:"ended_at,omitempty"`
}

type platformWorkflowInteraction struct {
	Kind           string         `json:"kind"`
	Required       bool           `json:"required"`
	Title          string         `json:"title"`
	Description    string         `json:"description,omitempty"`
	SubmitLabel    string         `json:"submit_label"`
	ResumeEndpoint string         `json:"resume_endpoint"`
	Content        gin.H          `json:"content,omitempty"`
	Form           []gin.H        `json:"form,omitempty"`
	Existing       []gin.H        `json:"existing,omitempty"`
	Output         map[string]any `json:"output,omitempty"`
}

type platformWorkflowRun struct {
	ID          string                           `json:"id"`
	URL         string                           `json:"url"`
	Platform    string                           `json:"platform,omitempty"`
	Status      string                           `json:"status"`
	CurrentNode string                           `json:"current_node,omitempty"`
	Probe       *contentdownload.Probe           `json:"-"`
	Existing    []gin.H                          `json:"-"`
	Resolved    *contentdownload.ResolvedRequest `json:"-"`
	TaskID      string                           `json:"task_id,omitempty"`
	Extra       map[string]any                   `json:"extra,omitempty"`
	Output      map[string]any                   `json:"output,omitempty"`
	Nodes       []platformWorkflowNode           `json:"nodes"`
	CreatedAt   time.Time                        `json:"created_at"`
	UpdatedAt   time.Time                        `json:"updated_at"`
	mu          sync.Mutex
}

var platformWorkflowRuns sync.Map

func (c *APIClient) handleProbePlatformDownloadTask(ctx *gin.Context) {
	var body struct {
		URL   string         `json:"url"`
		Extra map[string]any `json:"extra"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	body.URL = strings.TrimSpace(body.URL)
	if body.URL == "" {
		result.Err(ctx, 400, "缺少 url 参数")
		return
	}
	run := newPlatformWorkflowRun(body.URL, body.Extra)
	platformWorkflowRuns.Store(run.ID, run)
	probe, err := c.runPlatformWorkflowToProbe(ctx.Request.Context(), run)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, gin.H{
		"run_id":      run.ID,
		"probe_id":    run.ID,
		"platform":    probe.Platform,
		"content":     platformProbeContent(probe),
		"probe":       platformProbeView(probe),
		"form":        platformProbeForm(probe),
		"existing":    run.Existing,
		"output":      run.Output,
		"interaction": run.currentInteractionView(),
		"workflow":    run.snapshot(),
	})
}

func (c *APIClient) handleResumePlatformDownloadPipeline(ctx *gin.Context) {
	var body platformCreateTaskBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}

	runID := firstNonEmpty(body.RunID, body.ProbeID)
	run := c.lookupPlatformWorkflow(runID)
	if runID != "" && run == nil {
		result.Err(ctx, 404, "流程不存在或已过期，请重新解析链接")
		return
	}
	if run != nil {
		run.mu.Lock()
		status := run.Status
		currentNode := run.CurrentNode
		run.mu.Unlock()
		if status != "paused" || currentNode != "pause_after_probe" {
			result.Err(ctx, 409, "流程当前状态不能继续")
			return
		}
		if strings.TrimSpace(body.URL) == "" {
			body.URL = run.URL
		}
	}

	id, err := c.startPlatformDownloadTask(ctx.Request.Context(), body)
	if err != nil {
		if c.logger != nil {
			c.logger.Error().Str("url", body.URL).Str("run_id", runID).Err(err).Msg("继续平台下载流程失败")
		}
		result.Err(ctx, 500, "继续流程失败："+err.Error())
		return
	}

	resp := gin.H{"id": id}
	if run != nil {
		resp["run_id"] = run.ID
		resp["workflow"] = run.snapshot()
	}
	result.Ok(ctx, resp)
}

func (c *APIClient) runPlatformWorkflowToProbe(ctx context.Context, run *platformWorkflowRun) (*contentdownload.Probe, error) {
	router := c.platformDownloadRouter()
	run.startNode("match_platform", "match_platform")
	handler := router.Match(run.URL)
	if handler == nil {
		err := contentdownload.ErrUnsupportedURL
		run.failNode("match_platform", err)
		return nil, err
	}
	run.Platform = handler.Platform()
	run.completeNode("match_platform")

	run.startNode("probe", "probe")
	probe, err := handler.Probe(ctx, contentdownload.ProbeInput{URL: run.URL, Extra: run.Extra})
	if err != nil {
		run.failNode("probe", err)
		return nil, err
	}
	probe.ID = run.ID
	run.Probe = probe
	run.Output = platformProbeOutput(probe)
	run.completeNode("probe")

	run.startNode("check_existing", "check_existing")
	run.Existing = c.platformProbeExisting(probe)
	run.completeNode("check_existing")

	run.startNode("pause_after_probe", "user_confirmation")
	run.waitForUserConfirmation("pause_after_probe", platformProbeConfirmation(probe, run.Existing))
	return probe, nil
}

func (c *APIClient) startPlatformDownloadTask(ctx context.Context, body platformCreateTaskBody) (string, error) {
	body.URL = strings.TrimSpace(body.URL)
	if body.URL == "" {
		return "", fmt.Errorf("缺少 url 参数")
	}
	opts := body.Options
	if opts.VariantID == "" {
		opts.VariantID = body.VariantID
	}
	if opts.Spec == "" {
		opts.Spec = body.Spec
	}
	if opts.Suffix == "" {
		opts.Suffix = body.Suffix
	}
	if opts.Filename == "" {
		opts.Filename = body.Filename
	}
	if body.Cover && opts.VariantID == "" {
		opts.VariantID = "cover"
	}
	if opts.Extra == nil {
		opts.Extra = body.Extra
	}
	if opts.Suffix == ".mp3" && !system.ExistingCommand("ffmpeg") {
		return "", fmt.Errorf("下载 mp3 需要支持 ffmpeg 命令")
	}

	router := c.platformDownloadRouter()
	run := c.lookupPlatformWorkflow(firstNonEmpty(body.RunID, body.ProbeID))
	probe := (*contentdownload.Probe)(nil)
	if run != nil {
		probe = run.Probe
		run.resumeAfterProbe(platformConfirmationOutput(body, opts))
		run.startNode("resolve", "resolve")
	}
	resolved, err := router.Resolve(ctx, contentdownload.ResolveInput{
		URL:     body.URL,
		Probe:   probe,
		Options: opts,
		Extra:   body.Extra,
	})
	if err != nil {
		if run != nil {
			run.failNode("resolve", err)
		}
		return "", err
	}
	if run != nil {
		run.Resolved = resolved
		run.completeNode("resolve")
	}
	if resolved.Suffix == ".mp3" && !system.ExistingCommand("ffmpeg") {
		return "", fmt.Errorf("下载 mp3 需要支持 ffmpeg 命令")
	}

	downloadDir := ""
	if c.cfg != nil {
		downloadDir = c.cfg.DownloadDir
	}
	var recID int
	downloader := contentdownload.NewDownloader(
		router,
		downloadDir,
		contentdownload.WithExecutor(contentoa.NewExecutor(nil)),
		contentdownload.WithExecutor(contentzhihu.NewExecutor(nil)),
		contentdownload.WithEventHandler(func(evt contentdownload.Event) {
			if recID <= 0 || evt.Task == nil {
				return
			}
			if evt.Kind != contentdownload.EventTaskProgress {
				return
			}
			progressBytes, _ := json.Marshal(map[string]any{
				"downloaded": evt.Task.Progress.DownloadedBytes,
				"total":      evt.Task.Progress.TotalBytes,
				"speed":      0,
			})
			_ = c.updatePlatformDownloadTask(recID, map[string]any{
				"status":     1,
				"progress":   string(progressBytes),
				"updated_at": util.NowMillis(),
			})
			c.broadcastPlatformDownloadTask(evt.Task)
		}),
	)
	task, err := downloader.CreateResolved(resolved)
	if err != nil {
		return "", err
	}
	task.FilePath = platformTaskFilePath(downloadDir, resolved)
	if run != nil {
		run.TaskID = task.ID
		run.startNode("create_task", "create_task")
		run.completeNode("create_task")
	}

	rec, err := c.createPlatformDownloadTaskRecord(task)
	if err != nil {
		return "", err
	}
	recID = rec.Id

	go c.runPlatformDownloadTask(context.Background(), downloader, task.ID, rec.Id, run)
	return task.ID, nil
}

func (c *APIClient) lookupPlatformWorkflow(runID string) *platformWorkflowRun {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil
	}
	value, ok := platformWorkflowRuns.Load(runID)
	if !ok {
		return nil
	}
	run, ok := value.(*platformWorkflowRun)
	if !ok || run == nil {
		platformWorkflowRuns.Delete(runID)
		return nil
	}
	if time.Since(run.CreatedAt) > 10*time.Minute {
		platformWorkflowRuns.Delete(runID)
		return nil
	}
	return run
}

func (c *APIClient) runPlatformDownloadTask(ctx context.Context, downloader *contentdownload.Downloader, taskID string, recID int, run *platformWorkflowRun) {
	_ = c.updatePlatformDownloadTask(recID, map[string]any{
		"status":     1,
		"updated_at": util.NowMillis(),
	})
	if run != nil {
		run.startNode("download", "download_asset")
	}
	if task := downloader.GetTask(taskID); task != nil {
		c.broadcastPlatformDownloadTask(task)
	}
	err := downloader.Start(ctx, taskID)
	task := downloader.GetTask(taskID)
	if run != nil && err == nil {
		run.completeNode("download")
	}
	if err == nil && task != nil && task.Resolved != nil {
		if run != nil {
			run.startNode("postprocess", "postprocess")
		}
		err = c.runPlatformPostprocess(task)
		if run != nil && err == nil {
			run.completeNode("postprocess")
		}
	}
	if err != nil {
		if run != nil {
			run.failCurrent(err)
		}
		_ = c.updatePlatformDownloadTask(recID, map[string]any{
			"status":     5,
			"error":      err.Error(),
			"updated_at": util.NowMillis(),
		})
		if task != nil {
			c.broadcastPlatformDownloadTask(task)
		}
		return
	}
	if run != nil {
		run.startNode("persist", "persist")
	}
	progress := `{"downloaded":0,"total":0,"speed":0}`
	if task != nil {
		progressBytes, _ := json.Marshal(map[string]any{
			"downloaded": task.Progress.DownloadedBytes,
			"total":      task.Progress.TotalBytes,
			"speed":      0,
		})
		progress = string(progressBytes)
	}
	_ = c.updatePlatformDownloadTask(recID, map[string]any{
		"status":     4,
		"progress":   progress,
		"filepath":   task.FilePath,
		"updated_at": util.NowMillis(),
	})
	if task != nil {
		c.broadcastPlatformDownloadTask(task)
	}
	if run != nil {
		run.completeNode("persist")
		run.finish()
	}
}

func (c *APIClient) broadcastPlatformDownloadTask(task *contentdownload.Task) {
	if c.downloader_ws == nil || task == nil || task.Resolved == nil {
		return
	}
	status := "ready"
	switch task.Status {
	case contentdownload.TaskStatusResolving:
		status = "wait"
	case contentdownload.TaskStatusDownloading, contentdownload.TaskStatusProcessing:
		status = "running"
	case contentdownload.TaskStatusDone:
		status = "done"
	case contentdownload.TaskStatusError:
		status = "error"
	}
	name := filepath.Base(task.FilePath)
	dir := filepath.Dir(task.FilePath)
	c.downloader_ws.Broadcast(APIClientWSMessage{
		Type: "event",
		Data: map[string]any{
			"task": map[string]any{
				"id":        task.ID,
				"status":    status,
				"error":     task.Error,
				"createdAt": task.CreatedAt,
				"updatedAt": task.UpdatedAt,
				"meta": map[string]any{
					"req": map[string]any{
						"url": task.Resolved.Download.URL,
					},
					"opts": map[string]any{
						"name": name,
						"path": dir,
					},
					"res": map[string]any{
						"name": name,
						"size": task.Progress.TotalBytes,
						"files": []map[string]any{
							{"name": name, "path": "", "size": task.Progress.TotalBytes},
						},
					},
				},
				"progress": map[string]any{
					"downloaded": task.Progress.DownloadedBytes,
					"speed":      0,
				},
			},
		},
	})
}

func (c *APIClient) runPlatformPostprocess(task *contentdownload.Task) error {
	if task == nil || task.Resolved == nil {
		return nil
	}
	key := strings.TrimSpace(task.Resolved.Labels["key"])
	if key != "" && key != "0" && task.Resolved.Suffix != ".jpg" && task.Resolved.Suffix != ".zip" {
		k, err := strconv.Atoi(key)
		if err == nil && k != 0 {
			data, err := os.ReadFile(task.FilePath)
			if err != nil {
				return err
			}
			decrypt.DecryptData(data, 131072, uint64(k))
			if err := os.WriteFile(task.FilePath, data, 0o644); err != nil {
				return err
			}
		}
	}
	if task.Resolved.Suffix == ".mp3" {
		tempPath := task.FilePath + ".temp"
		if err := os.Rename(task.FilePath, tempPath); err != nil {
			return err
		}
		if err := system.RunCommand("ffmpeg", "-i", tempPath, "-vn", "-acodec", "libmp3lame", "-ab", "192k", "-f", "mp3", task.FilePath); err != nil {
			_ = os.Rename(tempPath, task.FilePath)
			return err
		}
		_ = os.Remove(tempPath)
	}
	return nil
}

func (c *APIClient) createPlatformDownloadTaskRecord(task *contentdownload.Task) (*model.DownloadTask, error) {
	if c.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	if task == nil || task.Resolved == nil {
		return nil, fmt.Errorf("下载任务为空")
	}
	resolved := task.Resolved
	now := util.NowMillis()
	metaBytes, _ := json.Marshal(map[string]any{
		"platform":   resolved.Platform,
		"content_id": resolved.ContentID,
		"source_url": resolved.SourceURL,
		"pipeline":   resolved.Pipeline,
		"metadata":   resolved.Metadata,
	})
	progressBytes, _ := json.Marshal(map[string]any{
		"downloaded": 0,
		"total":      0,
		"speed":      0,
	})
	rec := model.DownloadTask{
		TaskId:     task.ID,
		Status:     1,
		ExternalId: resolved.ContentID,
		Protocol:   resolved.Download.Protocol,
		URL:        resolved.Download.URL,
		Title:      firstNonEmpty(resolved.Title, resolved.Filename, resolved.ContentID),
		CoverURL:   contentCoverURL(resolved),
		Progress:   string(progressBytes),
		Filepath:   task.FilePath,
		Reason:     "platform",
		Metadata2:  string(metaBytes),
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
	if err := c.db.Create(&rec).Error; err != nil {
		return nil, err
	}
	return &rec, nil
}

func (c *APIClient) updatePlatformDownloadTask(id int, updates map[string]any) error {
	if c.db == nil {
		return nil
	}
	return c.db.Model(&model.DownloadTask{}).Where("id = ?", id).Updates(updates).Error
}

func (c *APIClient) platformDownloadRouter() *contentdownload.Router {
	return contentdownload.NewRouter(
		contentchannels.New(platformChannelsFetcher{
			FeedProfileFetcher: c.channels,
			sphCookie:          c.cfg.CloudflareSphCookie,
		}),
		contentdouyin.New(nil),
		contentzhihu.New(nil),
		contentoa.New(nil),
		contentyoutube.New(nil),
		contentshuba69.New(nil),
		contentqidian.New(nil),
		contentquanben.New(nil),
		contentttk.New(nil),
		contentfanqie.New(nil),
	)
}

type platformChannelsFetcher struct {
	contentchannels.FeedProfileFetcher
	sphCookie string
}

func (f platformChannelsFetcher) FetchChannelsSphProfile(reqURL string) (*contentchannels.SphProfile, error) {
	if strings.TrimSpace(f.sphCookie) == "" {
		return nil, fmt.Errorf("cloudflare.sphCookie not configured")
	}
	feedResp, err := fetchVideoProfileWithShareUrl(reqURL, f.sphCookie)
	if err != nil {
		return nil, err
	}
	if feedResp == nil {
		return nil, fmt.Errorf("empty sph feed response")
	}
	videoURL := strings.TrimSpace(feedResp.Data.Feedinfo.Videourl)
	return &contentchannels.SphProfile{
		ShareURL:        reqURL,
		SphID:           "",
		ExportID:        firstNonEmpty(feedResp.Data.Sceneinfo.Dynamicexportid),
		VideoURL:        videoURL,
		OriginVideoURL:  cleanVideoURL(videoURL),
		Description:     feedResp.Data.Feedinfo.Description,
		CoverURL:        firstNonEmpty(feedResp.Data.Feedinfo.Coverurl),
		MediaType:       feedResp.Data.Feedinfo.Mediatype,
		CreateTime:      int64(feedResp.Data.Feedinfo.Createtime),
		AuthorNickname:  feedResp.Data.Authorinfo.Nickname,
		AuthorAvatarURL: feedResp.Data.Authorinfo.Headimgurl,
		ErrCode:         feedResp.Errcode,
		ErrMsg:          feedResp.Errmsg,
	}, nil
}

func platformTaskFilePath(downloadDir string, resolved *contentdownload.ResolvedRequest) string {
	filename := strings.TrimSpace(firstNonEmpty(resolved.Filename, resolved.Title, resolved.ContentID, "download"))
	suffix := strings.TrimSpace(resolved.Suffix)
	if suffix != "" && !strings.HasSuffix(strings.ToLower(filename), strings.ToLower(suffix)) {
		filename += suffix
	}
	return filepath.Join(downloadDir, filename)
}

func platformProbeContent(probe *contentdownload.Probe) gin.H {
	if probe == nil {
		return gin.H{}
	}
	return platformContentView(probe.Content)
}

func platformContentView(content any) gin.H {
	summary := contentdownload.ContentSummaryOf(content)
	if summary == (contentdownload.ContentSummary{}) {
		return gin.H{}
	}
	return gin.H{
		"platform":          summary.Platform,
		"content_type":      summary.Type,
		"content_id":        summary.ID,
		"title":             summary.Title,
		"description":       summary.Description,
		"author":            summary.Author,
		"url":               summary.URL,
		"source_url":        summary.SourceURL,
		"author_nickname":   summary.AuthorNickname,
		"author_avatar_url": summary.AuthorAvatarURL,
		"cover_url":         summary.CoverURL,
		"duration":          summary.Duration,
	}
}

func platformProbeOutput(probe *contentdownload.Probe) map[string]any {
	if probe == nil {
		return nil
	}
	size := 1
	contentOutput := contentdownload.ContentOutputOf(probe.Content)
	size += len(contentOutput)
	out := make(map[string]any, size)
	for k, v := range contentOutput {
		out[k] = v
	}
	out["content"] = probe.Content
	return out
}

func platformProbeView(probe *contentdownload.Probe) gin.H {
	if probe == nil {
		return gin.H{}
	}
	return gin.H{
		"id":            probe.ID,
		"platform":      probe.Platform,
		"source_url":    probe.SourceURL,
		"canonical_url": probe.CanonicalURL,
		"content_id":    probe.ContentID,
		"content":       platformProbeContent(probe),
		"variants":      probe.Variants,
		"defaults":      probe.Defaults,
		"warnings":      probe.Warnings,
	}
}

func platformProbeForm(probe *contentdownload.Probe) []gin.H {
	form := []gin.H{
		{
			"name":     "variant_id",
			"label":    "下载内容",
			"type":     "select",
			"required": true,
			"default":  probe.Defaults.VariantID,
			"options":  probe.Variants,
		},
		{
			"name":  "filename",
			"label": "文件名",
			"type":  "text",
			"value": contentdownload.ContentTitle(probe.Content),
		},
	}
	if probe.Platform == contentchannels.PlatformID {
		form = append(form, gin.H{
			"name":  "spec",
			"label": "清晰度参数",
			"type":  "hidden",
			"value": probe.Defaults.Spec,
		})
	}
	return form
}

func platformProbeConfirmation(probe *contentdownload.Probe, existing []gin.H) *platformWorkflowInteraction {
	return &platformWorkflowInteraction{
		Kind:           "confirmation",
		Required:       true,
		Title:          "确认下载内容",
		Description:    "请选择下载内容并确认参数后继续流程。",
		SubmitLabel:    "开始下载",
		ResumeEndpoint: "/api/task/pipeline/resume",
		Content:        platformProbeContent(probe),
		Form:           platformProbeForm(probe),
		Existing:       existing,
	}
}

func platformConfirmationOutput(body platformCreateTaskBody, opts contentdownload.Options) map[string]any {
	return map[string]any{
		"variant_id": opts.VariantID,
		"spec":       opts.Spec,
		"suffix":     opts.Suffix,
		"filename":   opts.Filename,
		"options":    opts,
		"extra":      firstNonNilMap(opts.Extra, body.Extra),
	}
}

func firstNonNilMap(values ...map[string]any) map[string]any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func contentCoverURL(resolved *contentdownload.ResolvedRequest) string {
	if resolved != nil {
		return contentdownload.ContentCoverURL(resolved.Content)
	}
	return ""
}

func (c *APIClient) platformProbeExisting(probe *contentdownload.Probe) []gin.H {
	if c.db == nil || probe == nil || probe.ContentID == "" {
		return nil
	}
	var tasks []model.DownloadTask
	if err := c.db.Where("external_id = ?", probe.ContentID).Order("id DESC").Limit(20).Find(&tasks).Error; err != nil {
		return nil
	}
	out := make([]gin.H, 0, len(tasks))
	for _, task := range tasks {
		meta := map[string]any{}
		_ = json.Unmarshal([]byte(task.Metadata2), &meta)
		platform := strings.TrimSpace(fmt.Sprint(meta["platform"]))
		if platform != "" && platform != probe.Platform {
			continue
		}
		out = append(out, gin.H{
			"id":         task.Id,
			"task_id":    task.TaskId,
			"status":     task.Status,
			"title":      task.Title,
			"filepath":   task.Filepath,
			"platform":   firstNonEmpty(platform, probe.Platform),
			"content_id": task.ExternalId,
			"updated_at": task.UpdatedAt,
		})
	}
	return out
}

func newPlatformWorkflowRun(rawURL string, extra map[string]any) *platformWorkflowRun {
	now := time.Now()
	return &platformWorkflowRun{
		ID:          fmt.Sprintf("run_%d", now.UnixNano()),
		URL:         rawURL,
		Status:      "running",
		CurrentNode: "start",
		Extra:       extra,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (r *platformWorkflowRun) startNode(id, typ string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	r.Status = "running"
	r.CurrentNode = id
	r.UpdatedAt = now
	for i := range r.Nodes {
		if r.Nodes[i].ID == id {
			r.Nodes[i].Type = typ
			r.Nodes[i].Status = "running"
			r.Nodes[i].Error = ""
			r.Nodes[i].StartedAt = now
			r.Nodes[i].EndedAt = time.Time{}
			return
		}
	}
	r.Nodes = append(r.Nodes, platformWorkflowNode{
		ID:        id,
		Type:      typ,
		Status:    "running",
		StartedAt: now,
	})
}

func (r *platformWorkflowRun) completeNode(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	r.UpdatedAt = now
	for i := range r.Nodes {
		if r.Nodes[i].ID == id {
			r.Nodes[i].Status = "completed"
			r.Nodes[i].EndedAt = now
			return
		}
	}
}

func (r *platformWorkflowRun) failNode(id string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	r.Status = "failed"
	r.CurrentNode = id
	r.UpdatedAt = now
	for i := range r.Nodes {
		if r.Nodes[i].ID == id {
			r.Nodes[i].Status = "failed"
			r.Nodes[i].Error = err.Error()
			r.Nodes[i].EndedAt = now
			return
		}
	}
	r.Nodes = append(r.Nodes, platformWorkflowNode{
		ID:      id,
		Type:    id,
		Status:  "failed",
		Error:   err.Error(),
		EndedAt: now,
	})
}

func (r *platformWorkflowRun) failCurrent(err error) {
	if err == nil {
		return
	}
	r.failNode(r.CurrentNode, err)
}

func (r *platformWorkflowRun) resumeAfterProbe(output map[string]any) {
	r.mu.Lock()
	now := time.Now()
	if r.CurrentNode != "pause_after_probe" {
		r.mu.Unlock()
		return
	}
	r.Status = "running"
	r.CurrentNode = "resume_after_probe"
	r.UpdatedAt = now
	for i := range r.Nodes {
		if r.Nodes[i].ID == "pause_after_probe" &&
			(r.Nodes[i].Status == "paused" || r.Nodes[i].Status == "waiting_user") {
			r.Nodes[i].Status = "completed"
			r.Nodes[i].Output = output
			r.Nodes[i].EndedAt = now
			break
		}
	}
	r.Nodes = append(r.Nodes, platformWorkflowNode{
		ID:        "resume_after_probe",
		Type:      "resume",
		Status:    "completed",
		StartedAt: now,
		EndedAt:   now,
	})
	r.mu.Unlock()
}

func (r *platformWorkflowRun) waitForUserConfirmation(id string, interaction *platformWorkflowInteraction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	r.Status = "paused"
	r.CurrentNode = id
	r.UpdatedAt = now
	for i := range r.Nodes {
		if r.Nodes[i].ID == id {
			r.Nodes[i].Type = "user_confirmation"
			r.Nodes[i].Status = "waiting_user"
			r.Nodes[i].Output = nil
			r.Nodes[i].Interaction = interaction
			r.Nodes[i].EndedAt = now
			return
		}
	}
	r.Nodes = append(r.Nodes, platformWorkflowNode{
		ID:          id,
		Type:        "user_confirmation",
		Status:      "waiting_user",
		Interaction: interaction,
		StartedAt:   now,
		EndedAt:     now,
	})
}

func (r *platformWorkflowRun) finish() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Status = "completed"
	r.CurrentNode = "done"
	r.UpdatedAt = time.Now()
}

func (r *platformWorkflowRun) snapshot() gin.H {
	r.mu.Lock()
	defer r.mu.Unlock()
	nodes := make([]gin.H, 0, len(r.Nodes))
	for _, node := range r.Nodes {
		view := gin.H{
			"id":         node.ID,
			"type":       node.Type,
			"status":     node.Status,
			"started_at": node.StartedAt,
			"ended_at":   node.EndedAt,
		}
		if node.Error != "" {
			view["error"] = node.Error
		}
		if node.Interaction != nil {
			view["interaction"] = platformInteractionSummary(node.Interaction)
		}
		if node.Type == "user_confirmation" && node.Status == "completed" && len(node.Output) > 0 {
			view["output"] = node.Output
		}
		nodes = append(nodes, view)
	}
	return gin.H{
		"id":           r.ID,
		"url":          r.URL,
		"platform":     r.Platform,
		"status":       r.Status,
		"current_node": r.CurrentNode,
		"task_id":      r.TaskID,
		"nodes":        nodes,
		"created_at":   r.CreatedAt,
		"updated_at":   r.UpdatedAt,
	}
}

func (r *platformWorkflowRun) currentInteractionView() gin.H {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.Nodes {
		if r.Nodes[i].ID == r.CurrentNode {
			return platformInteractionSummary(r.Nodes[i].Interaction)
		}
	}
	return nil
}

func platformInteractionSummary(interaction *platformWorkflowInteraction) gin.H {
	if interaction == nil {
		return nil
	}
	return gin.H{
		"kind":            interaction.Kind,
		"required":        interaction.Required,
		"title":           interaction.Title,
		"description":     interaction.Description,
		"submit_label":    interaction.SubmitLabel,
		"resume_endpoint": interaction.ResumeEndpoint,
		"form":            interaction.Form,
	}
}
