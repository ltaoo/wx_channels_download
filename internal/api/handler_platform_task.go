package api

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	result "wx_channel/internal/util"
	contentshuba69 "wx_channel/pkg/contentplatform/69shuba"
	contentbilibili "wx_channel/pkg/contentplatform/bilibili"
	contentdouban "wx_channel/pkg/contentplatform/douban"
	contentdouyin "wx_channel/pkg/contentplatform/douyin"
	contentdownload "wx_channel/pkg/contentplatform/download"
	contentfanqie "wx_channel/pkg/contentplatform/fanqienovel"
	contentinstagram "wx_channel/pkg/contentplatform/instagram"
	contentiqiyi "wx_channel/pkg/contentplatform/iqiyi"
	contentmgtv "wx_channel/pkg/contentplatform/mgtv"
	contentoa "wx_channel/pkg/contentplatform/officialaccount"
	contentqidian "wx_channel/pkg/contentplatform/qidian"
	contentqq "wx_channel/pkg/contentplatform/qq"
	contentquanben "wx_channel/pkg/contentplatform/quanben"
	contentsoundgasm "wx_channel/pkg/contentplatform/soundgasm"
	contenttelegram "wx_channel/pkg/contentplatform/telegram"
	contenttmdb "wx_channel/pkg/contentplatform/tmdb"
	contentttk "wx_channel/pkg/contentplatform/ttk"
	contentv2ex "wx_channel/pkg/contentplatform/v2ex"
	contentwebpage "wx_channel/pkg/contentplatform/webpage"
	contentweibo "wx_channel/pkg/contentplatform/weibo"
	contentwxchannels "wx_channel/pkg/contentplatform/wxchannels"
	contentx "wx_channel/pkg/contentplatform/x"
	contentxiaohongshu "wx_channel/pkg/contentplatform/xiaohongshu"
	contentyouku "wx_channel/pkg/contentplatform/youku"
	contentyoutube "wx_channel/pkg/contentplatform/youtube"
	contentzhihu "wx_channel/pkg/contentplatform/zhihu"
	"wx_channel/pkg/decrypt"
	weibopkg "wx_channel/pkg/scraper/weibo"
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

const (
	downloadNodeTypeContainer = "container"
	downloadNodeTypeFile      = "file"

	downloadEngineHTTP           = "http"
	downloadEngineClawreq        = "clawreq"
	downloadEngineCDP            = "cdp"
	downloadEngineSandboxCDP     = "sandbox_cdp"
	downloadEngineBrowserPoolCDP = "browser_pool_cdp"
	downloadEngineFS             = "fs"

	platformRetryFilePathsKey = "retry_file_paths"
)

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
	ID             string                           `json:"id"`
	URL            string                           `json:"url"`
	Platform       string                           `json:"platform,omitempty"`
	Status         string                           `json:"status"`
	CurrentNode    string                           `json:"current_node,omitempty"`
	Probe          *contentdownload.Probe           `json:"-"`
	Existing       []gin.H                          `json:"-"`
	Resolved       *contentdownload.ResolvedRequest `json:"-"`
	TaskID         string                           `json:"task_id,omitempty"`
	DownloadTaskID string                              `json:"download_task_id,omitempty"`
	Extra          map[string]any                   `json:"extra,omitempty"`
	Output         map[string]any                   `json:"output,omitempty"`
	Selection      map[string]any                   `json:"selection,omitempty"`
	Nodes          []platformWorkflowNode           `json:"nodes"`
	CreatedAt      time.Time                        `json:"created_at"`
	UpdatedAt      time.Time                        `json:"updated_at"`
	mu             sync.Mutex
	onChange       func(*platformWorkflowRun)
}

const platformJSONVariantID = "json"

var platformWorkflowRuns sync.Map
var platformActiveDownloads sync.Map

type platformActiveDownload struct {
	downloader *contentdownload.Downloader
	taskID     string
	recID      string
	run        *platformWorkflowRun

	mu      sync.Mutex
	cancel  context.CancelFunc
	paused  bool
	running bool
	resume  bool
}

func (d *platformActiveDownload) begin() (context.Context, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.running {
		return nil, false
	}
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	d.paused = false
	d.running = true
	return ctx, true
}

func (d *platformActiveDownload) finish(paused bool) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	shouldResume := paused && d.resume
	d.cancel = nil
	d.running = false
	d.resume = false
	if shouldResume {
		d.paused = false
		return true
	}
	d.paused = paused
	return false
}

func (d *platformActiveDownload) pause() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.paused = true
	if d.cancel == nil {
		return false
	}
	d.cancel()
	return true
}

func (d *platformActiveDownload) isPaused() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.paused
}

func (d *platformActiveDownload) requestResume() (startNow bool, ok bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.paused {
		return false, false
	}
	if d.running {
		d.resume = true
		return false, true
	}
	d.paused = false
	return true, true
}

func (c *APIClient) handleProbePlatformDownloadTask(ctx *gin.Context) {
	var body struct {
		URL     string         `json:"url"`
		RunID   string         `json:"run_id"`
		ProbeID string         `json:"probe_id"`
		Extra   map[string]any `json:"extra"`
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
	run := c.attachPlatformWorkflowPersistence(newPlatformWorkflowRunWithID(body.URL, body.Extra, firstNonEmpty(body.RunID, body.ProbeID)))
	platformWorkflowRuns.Store(run.ID, run)
	_ = c.persistPlatformWorkflowRun(run)
	probe, err := c.runPlatformWorkflowToProbe(ctx.Request.Context(), run)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	resp := gin.H{
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
	}
	if pageJSON := platformProbePageJSON(probe); pageJSON != nil {
		resp["pagejson"] = pageJSON
	}
	if pageHTML := platformProbePageHTML(probe); pageHTML != "" {
		resp["pagehtml"] = pageHTML
	}
	if probePipeline := platformProbePipeline(probe); probePipeline != nil {
		resp["probe_pipeline"] = probePipeline
	}
	result.Ok(ctx, resp)
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
		if strings.TrimSpace(body.URL) == "" {
			body.URL = run.URL
		}
		id, needsUser, err := c.resumePlatformWorkflow(ctx.Request.Context(), run, body)
		if err != nil {
			if c.logger != nil {
				c.logger.Error().Str("url", body.URL).Str("run_id", runID).Err(err).Msg("继续平台下载流程失败")
			}
			result.Err(ctx, 500, "继续流程失败："+err.Error())
			return
		}
		resp := gin.H{
			"run_id":              run.ID,
			"probe_id":            run.ID,
			"workflow":            run.snapshot(),
			"interaction":         run.currentInteractionView(),
			"requires_user_input": needsUser,
		}
		if id != "" {
			resp["id"] = id
		}
		result.Ok(ctx, resp)
		return
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

func (c *APIClient) handleFetchPlatformDownloadWorkflow(ctx *gin.Context) {
	runID := firstNonEmpty(ctx.Query("run_id"), ctx.Query("probe_id"))
	run := c.lookupPlatformWorkflow(runID)
	if run == nil {
		result.Err(ctx, 404, "流程不存在")
		return
	}
	result.Ok(ctx, c.platformWorkflowResponse(run, "", false))
}

func (c *APIClient) resumePlatformWorkflow(ctx context.Context, run *platformWorkflowRun, body platformCreateTaskBody) (string, bool, error) {
	if run == nil {
		return "", false, fmt.Errorf("workflow is nil")
	}
	if strings.TrimSpace(body.URL) == "" {
		body.URL = run.URL
	}
	body.RunID = run.ID

	run.mu.Lock()
	status := run.Status
	currentNode := run.CurrentNode
	taskID := run.TaskID
	downloadTaskID := run.DownloadTaskID
	run.mu.Unlock()

	if status == "completed" {
		return taskID, false, nil
	}
	if currentNode == "pause_after_probe" && !platformWorkflowHasUserSelection(body) {
		return "", true, nil
	}
	if currentNode == "pause_after_probe" && platformWorkflowHasUserSelection(body) {
		id, err := c.startPlatformDownloadTask(ctx, body)
		return id, false, err
	}
	if taskID != "" || downloadTaskID != "" {
		id, err := c.resumePlatformWorkflowDownloadTask(ctx, run)
		if err == nil || !errors.Is(err, gorm.ErrRecordNotFound) {
			return id, false, err
		}
	}
	if savedBody, ok := platformWorkflowCreateBodyFromSelection(run); ok {
		id, err := c.startPlatformDownloadTask(ctx, savedBody)
		return id, false, err
	}
	if _, err := c.runPlatformWorkflowToProbe(ctx, run); err != nil {
		return "", false, err
	}
	return "", true, nil
}

func (c *APIClient) resumePlatformWorkflowDownloadTask(ctx context.Context, run *platformWorkflowRun) (string, error) {
	if c == nil || c.db == nil {
		return "", fmt.Errorf("数据库未初始化")
	}
	run.mu.Lock()
	taskID := strings.TrimSpace(run.TaskID)
	downloadTaskID := run.DownloadTaskID
	run.mu.Unlock()

	if taskID != "" && c.resumeActivePlatformDownload(taskID) {
		return taskID, nil
	}

	var rec model.DownloadTask
	var err error
	switch {
	case downloadTaskID != "":
		err = c.db.First(&rec, "id = ?", downloadTaskID).Error
	case taskID != "":
		err = c.db.First(&rec, "task_id = ?", taskID).Error
	default:
		err = gorm.ErrRecordNotFound
	}
	if err != nil {
		return "", err
	}
	if rec.Status == 4 {
		run.finish()
		return rec.TaskId, nil
	}
	return c.restartPlatformDownloadTaskRecordWithFilePathsAndRun(ctx, rec, nil, run)
}

func (c *APIClient) platformWorkflowResponse(run *platformWorkflowRun, taskID string, needsUser bool) gin.H {
	resp := gin.H{
		"run_id":              run.ID,
		"probe_id":            run.ID,
		"workflow":            run.snapshot(),
		"interaction":         run.currentInteractionView(),
		"requires_user_input": needsUser,
	}
	if taskID != "" {
		resp["id"] = taskID
	}
	return resp
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
	_ = c.persistPlatformWorkflowRun(run)
	run.completeNode("match_platform")

	run.startNode("probe", "probe")
	probe, err := handler.Probe(ctx, contentdownload.ProbeInput{URL: run.URL, Extra: run.Extra})
	if err != nil {
		run.failNode("probe", err)
		return nil, err
	}
	probe.ID = run.ID
	platformProbeAddJSONDefault(probe)
	run.Probe = probe
	run.Output = platformProbeOutput(probe)
	_ = c.persistPlatformWorkflowRun(run)
	run.completeNode("probe")

	run.startNode("check_existing", "check_existing")
	run.Existing = c.platformProbeExisting(probe)
	_ = c.persistPlatformWorkflowRun(run)
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
	opts = platformOptionsWithRuntimeCapabilities(opts)
	if opts.Suffix == ".mp3" && !system.ExistingCommand("ffmpeg") {
		return "", fmt.Errorf("下载 mp3 需要支持 ffmpeg 命令")
	}

	router := c.platformDownloadRouter()
	run := c.lookupPlatformWorkflow(firstNonEmpty(body.RunID, body.ProbeID))
	probe := (*contentdownload.Probe)(nil)
	if run != nil {
		probe = run.Probe
		if opts.VariantID == "" && probe != nil {
			opts.VariantID = probe.Defaults.VariantID
		}
		selection := platformConfirmationOutput(body, opts)
		run.mu.Lock()
		currentNode := run.CurrentNode
		run.mu.Unlock()
		if currentNode == "pause_after_probe" {
			run.resumeAfterProbe(selection)
		} else {
			run.mu.Lock()
			run.Selection = clonePlatformWorkflowMap(selection)
			run.UpdatedAt = time.Now()
			run.mu.Unlock()
			_ = c.persistPlatformWorkflowRun(run)
		}
		run.startNode("resolve", "resolve")
	}
	var resolved *contentdownload.ResolvedRequest
	var err error
	if opts.VariantID == platformJSONVariantID {
		if probe == nil {
			handler := router.Match(body.URL)
			if handler == nil {
				err := contentdownload.ErrUnsupportedURL
				if run != nil {
					run.failNode("resolve", err)
				}
				return "", err
			}
			probe, err = handler.Probe(ctx, contentdownload.ProbeInput{URL: body.URL, Extra: body.Extra})
			if err != nil {
				if run != nil {
					run.failNode("resolve", err)
				}
				return "", err
			}
			platformProbeAddJSONDefault(probe)
		}
		if platformProbeJSONVariantDisabled(probe) {
			err := contentdownload.ErrVariantNotFound
			if run != nil {
				run.failNode("resolve", err)
			}
			return "", err
		}
		resolved = platformJSONResolvedRequest(body.URL, probe, opts)
	} else {
		resolved, err = router.Resolve(ctx, contentdownload.ResolveInput{
			URL:     body.URL,
			Probe:   probe,
			Options: opts,
			Extra:   body.Extra,
		})
	}
	if err != nil {
		if run != nil {
			run.failNode("resolve", err)
		}
		return "", err
	}
	if run != nil {
		if resolved.Metadata == nil {
			resolved.Metadata = map[string]any{}
		}
		resolved.Metadata["workflow_run_id"] = run.ID
	}
	if run != nil {
		run.Resolved = resolved
		_ = c.persistPlatformWorkflowRun(run)
		run.completeNode("resolve")
	}
	if resolvedRequiresFFmpeg(resolved) && !system.ExistingCommand("ffmpeg") {
		return "", fmt.Errorf("当前下载选项需要支持 ffmpeg 命令")
	}
	platformEnsureMetadataPipelineNodes(resolved)

	downloadDir := ""
	if c.cfg != nil {
		downloadDir = c.cfg.DownloadDir
	}
	var recID string
	downloader := contentdownload.NewDownloader(
		router,
		downloadDir,
		contentdownload.WithExecutor(contentoa.NewExecutor(nil)),
		contentdownload.WithExecutor(contentzhihu.NewExecutor(nil)),
		contentdownload.WithExecutor(contentshuba69.NewExecutor(c.shuba69Handler().Fetcher)),
		contentdownload.WithExecutor(contentshuba69.NewPDFExecutor()),
		contentdownload.WithExecutor(contentttk.NewExecutor(nil)),
		contentdownload.WithEventHandler(func(evt contentdownload.Event) {
			if recID == "" || evt.Task == nil {
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
				"downloaded": evt.Task.Progress.DownloadedBytes,
				"speed":      int64(0),
				"updated_at": util.NowMillis(),
			})
			c.syncPlatformDownloadTaskChildren(recID, evt.Task)
			c.broadcastPlatformDownloadTask(evt.Task, recID)
		}),
	)
	task, err := downloader.CreateResolved(resolved)
	if err != nil {
		return "", err
	}
	task.FilePath = platformTaskFilePath(downloadDir, resolved)
	if run != nil {
		run.TaskID = task.ID
		_ = c.persistPlatformWorkflowRun(run)
		run.startNode("create_task", "create_task")
		run.completeNode("create_task")
	}

	rec, err := c.createPlatformDownloadTaskRecord(task)
	if err != nil {
		return "", err
	}
	recID = rec.Id
	if run != nil {
		run.DownloadTaskID = rec.Id
		_ = c.persistPlatformWorkflowRun(run)
	}
	c.syncPlatformDownloadTaskChildren(rec.Id, task)

	planExec := &platformPlanExecution{
		Resolved:       resolved,
		DownloadTaskID: rec.Id,
	}
	if err := c.runPlatformPlanNodeTypes(ctx, run, planExec, map[string]bool{
		"create_account": true,
		"create_content": true,
	}); err != nil {
		return "", err
	}

	active := &platformActiveDownload{
		downloader: downloader,
		taskID:     task.ID,
		recID:      rec.Id,
		run:        run,
	}
	platformActiveDownloads.Store(task.ID, active)
	go c.runPlatformDownloadTask(active)
	return task.ID, nil
}

func (c *APIClient) restartPlatformDownloadTaskRecord(ctx context.Context, rec model.DownloadTask) (string, error) {
	return c.restartPlatformDownloadTaskRecordWithFilePaths(ctx, rec, nil)
}

func (c *APIClient) restartPlatformDownloadTaskRecordWithFilePaths(ctx context.Context, rec model.DownloadTask, retryFilePaths []string) (string, error) {
	return c.restartPlatformDownloadTaskRecordWithFilePathsAndRun(ctx, rec, retryFilePaths, nil)
}

func (c *APIClient) restartPlatformDownloadTaskRecordWithFilePathsAndRun(ctx context.Context, rec model.DownloadTask, retryFilePaths []string, run *platformWorkflowRun) (string, error) {
	body, err := compatPlatformRetryBody(rec)
	if err != nil {
		return "", err
	}
	if run != nil {
		body.RunID = run.ID
	}
	router := c.platformDownloadRouter()
	probe := (*contentdownload.Probe)(nil)
	var resolved *contentdownload.ResolvedRequest
	if body.Options.VariantID == platformJSONVariantID {
		handler := router.Match(body.URL)
		if handler == nil {
			return "", contentdownload.ErrUnsupportedURL
		}
		probe, err = handler.Probe(ctx, contentdownload.ProbeInput{URL: body.URL, Extra: body.Extra})
		if err != nil {
			return "", err
		}
		platformProbeAddJSONDefault(probe)
		if platformProbeJSONVariantDisabled(probe) {
			return "", contentdownload.ErrVariantNotFound
		}
		resolved = platformJSONResolvedRequest(body.URL, probe, body.Options)
	} else {
		body.Options = platformOptionsWithRuntimeCapabilities(body.Options)
		resolved, err = router.Resolve(ctx, contentdownload.ResolveInput{
			URL:     body.URL,
			Options: body.Options,
			Extra:   body.Extra,
		})
		if err != nil {
			return "", err
		}
	}
	if resolved == nil {
		return "", fmt.Errorf("平台下载解析结果为空")
	}
	if run != nil {
		if resolved.Metadata == nil {
			resolved.Metadata = map[string]any{}
		}
		resolved.Metadata["workflow_run_id"] = run.ID
		run.Resolved = resolved
		_ = c.persistPlatformWorkflowRun(run)
	}
	if resolvedRequiresFFmpeg(resolved) && !system.ExistingCommand("ffmpeg") {
		return "", fmt.Errorf("当前下载选项需要支持 ffmpeg 命令")
	}
	platformEnsureMetadataPipelineNodes(resolved)
	if existingFiles := platformRetryFilesFromMetadata(firstNonEmpty(rec.Metadata, rec.Metadata2)); len(existingFiles) > 0 && len(retryFilePaths) > 0 {
		resolved.Files = platformRetryMarkedFiles(existingFiles, retryFilePaths)
	}
	applyPlatformRetryFilePaths(resolved, retryFilePaths)

	downloadDir := ""
	if c.cfg != nil {
		downloadDir = c.cfg.DownloadDir
	}
	recID := rec.Id
	downloader := contentdownload.NewDownloader(
		router,
		downloadDir,
		contentdownload.WithExecutor(contentoa.NewExecutor(nil)),
		contentdownload.WithExecutor(contentzhihu.NewExecutor(nil)),
		contentdownload.WithExecutor(contentshuba69.NewExecutor(c.shuba69Handler().Fetcher)),
		contentdownload.WithExecutor(contentshuba69.NewPDFExecutor()),
		contentdownload.WithExecutor(contentttk.NewExecutor(nil)),
		contentdownload.WithEventHandler(func(evt contentdownload.Event) {
			if recID == "" || evt.Task == nil || evt.Kind != contentdownload.EventTaskProgress {
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
				"downloaded": evt.Task.Progress.DownloadedBytes,
				"speed":      int64(0),
				"updated_at": util.NowMillis(),
			})
			c.syncPlatformDownloadTaskChildren(recID, evt.Task)
			c.broadcastPlatformDownloadTask(evt.Task, recID)
		}),
	)
	task, err := downloader.CreateResolved(resolved)
	if err != nil {
		return "", err
	}
	task.FilePath = platformTaskFilePath(downloadDir, resolved)
	if run != nil {
		run.TaskID = task.ID
		run.DownloadTaskID = rec.Id
		run.startNode("create_task", "create_task")
		run.completeNode("create_task")
		_ = c.persistPlatformWorkflowRun(run)
	}
	progressBytes, _ := json.Marshal(map[string]any{
		"downloaded": 0,
		"total":      0,
		"speed":      0,
	})
	platformActiveDownloads.Delete(rec.TaskId)
	if err := c.updatePlatformDownloadTask(rec.Id, map[string]any{
		"task_uid":       task.ID,
		"task_id":        task.ID,
		"engine_task_id": task.ID,
		"node_type":      platformTaskNodeType(task),
		"engine":         c.platformTaskEngine(task),
		"status":         1,
		"external_id":    resolved.ContentID,
		"protocol":       platformTaskRecordProtocol(task),
		"url":            platformTaskRecordURL(task),
		"source_uri":     platformTaskSourceURI(task),
		"method":         firstNonEmpty(resolved.Download.Method, "GET"),
		"title":          firstNonEmpty(resolved.Title, resolved.Filename, resolved.ContentID, rec.Title),
		"filename":       firstNonEmpty(resolved.Filename, resolved.Title, resolved.ContentID),
		"cover_url":      contentCoverURL(resolved),
		"mime_type":      platformResolvedMimeType(resolved),
		"size":           contentdownload.FileNodesSize(platformTaskFiles(task)),
		"downloaded":     int64(0),
		"speed":          int64(0),
		"progress":       string(progressBytes),
		"filepath":       task.FilePath,
		"output_path":    "",
		"error":          "",
		"reason":         "platform",
		"metadata":       c.platformTaskMetadataJSON(task),
		"metadata2":      c.platformTaskMetadataJSON(task),
		"updated_at":     util.NowMillis(),
	}); err != nil {
		return "", err
	}
	c.syncPlatformDownloadTaskChildren(rec.Id, task)
	planExec := &platformPlanExecution{
		Resolved:       resolved,
		DownloadTaskID: rec.Id,
	}
	if err := c.runPlatformPlanNodeTypes(ctx, nil, planExec, map[string]bool{
		"create_account": true,
		"create_content": true,
	}); err != nil {
		return "", err
	}
	active := &platformActiveDownload{
		downloader: downloader,
		taskID:     task.ID,
		recID:      rec.Id,
		run:        run,
	}
	platformActiveDownloads.Store(task.ID, active)
	go c.runPlatformDownloadTask(active)
	return task.ID, nil
}

func applyPlatformRetryFilePaths(resolved *contentdownload.ResolvedRequest, retryFilePaths []string) {
	paths := normalizePlatformRetryFilePaths(retryFilePaths)
	if resolved == nil || len(paths) == 0 {
		return
	}
	if resolved.Metadata == nil {
		resolved.Metadata = map[string]any{}
	}
	if resolved.Internal == nil {
		resolved.Internal = map[string]any{}
	}
	resolved.Metadata[platformRetryFilePathsKey] = paths
	resolved.Internal[platformRetryFilePathsKey] = paths
}

func normalizePlatformRetryFilePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, path := range paths {
		path = strings.Trim(filepath.ToSlash(strings.TrimSpace(path)), "/")
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	return out
}

func platformRetryFilesFromMetadata(raw string) []contentdownload.FileNode {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var meta struct {
		Files []contentdownload.FileNode `json:"files"`
	}
	if err := json.Unmarshal([]byte(raw), &meta); err != nil || len(meta.Files) == 0 {
		return nil
	}
	return meta.Files
}

func platformRetryMarkedFiles(files []contentdownload.FileNode, retryFilePaths []string) []contentdownload.FileNode {
	out := contentdownload.CloneFileNodes(files)
	targets := map[string]bool{}
	for _, path := range normalizePlatformRetryFilePaths(retryFilePaths) {
		targets[path] = true
	}
	markPlatformRetryFiles(out, targets)
	return out
}

func markPlatformRetryFiles(files []contentdownload.FileNode, targets map[string]bool) bool {
	anyTarget := false
	for i := range files {
		path := strings.Trim(filepath.ToSlash(strings.TrimSpace(files[i].Path)), "/")
		childTarget := false
		if len(files[i].Children) > 0 {
			childTarget = markPlatformRetryFiles(files[i].Children, targets)
			files[i].Status = platformRetryDirectoryStatus(files[i].Children)
		}
		if targets[path] {
			files[i].Status = contentdownload.FileNodeStatusPending
			files[i].Error = ""
			childTarget = true
		}
		if childTarget {
			anyTarget = true
		}
	}
	return anyTarget
}

func platformRetryDirectoryStatus(children []contentdownload.FileNode) string {
	allDone := len(children) > 0
	anyError := false
	for _, child := range children {
		switch strings.ToLower(strings.TrimSpace(child.Status)) {
		case contentdownload.FileNodeStatusDone:
		case contentdownload.FileNodeStatusError, "failed", "fail":
			allDone = false
			anyError = true
		default:
			allDone = false
		}
	}
	if anyError {
		return contentdownload.FileNodeStatusError
	}
	if allDone {
		return contentdownload.FileNodeStatusDone
	}
	return contentdownload.FileNodeStatusPending
}

func compatPlatformRetryBody(rec model.DownloadTask) (platformCreateTaskBody, error) {
	var meta struct {
		SourceURL  string         `json:"source_url"`
		URL        string         `json:"url"`
		ContentURL string         `json:"content_url"`
		Metadata   map[string]any `json:"metadata"`
		Source     map[string]any `json:"source"`
	}
	rawMetadata := firstNonEmpty(rec.Metadata, rec.Metadata2)
	if strings.TrimSpace(rawMetadata) != "" {
		_ = json.Unmarshal([]byte(rawMetadata), &meta)
	}
	sourceURL := firstNonEmpty(
		meta.SourceURL,
		meta.URL,
		meta.ContentURL,
		toCompatString(meta.Source["source_url"]),
		toCompatString(meta.Source["canonical_url"]),
		toCompatString(meta.Metadata["source_url"]),
		toCompatString(meta.Metadata["canonical_url"]),
		toCompatString(meta.Metadata["url"]),
		toCompatString(meta.Metadata["content_url"]),
		rec.SourceURI,
	)
	if sourceURL == "" && compatURLLooksHTTP(rec.URL) {
		sourceURL = rec.URL
	}
	if sourceURL == "" {
		return platformCreateTaskBody{}, fmt.Errorf("平台任务缺少 source_url，无法重新解析下载")
	}
	options := contentdownload.Options{
		VariantID: toCompatString(meta.Metadata["variant_id"]),
		Spec:      toCompatString(meta.Metadata["spec"]),
		Suffix:    toCompatString(meta.Metadata["suffix"]),
		Filename:  compatPlatformRetryFilename(rec),
	}
	if options.VariantID == "" && strings.EqualFold(strings.TrimSpace(rec.Protocol), "inline_json") {
		options.VariantID = platformJSONVariantID
	}
	if options.Suffix == "" {
		options.Suffix = compatPlatformRetrySuffix(rec)
	}
	return platformCreateTaskBody{
		URL:       sourceURL,
		VariantID: options.VariantID,
		Spec:      options.Spec,
		Suffix:    options.Suffix,
		Filename:  options.Filename,
		Options:   options,
	}, nil
}

func compatURLLooksHTTP(rawURL string) bool {
	rawURL = strings.TrimSpace(strings.ToLower(rawURL))
	return strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://")
}

func compatPlatformRetryFilename(rec model.DownloadTask) string {
	name := strings.TrimSpace(rec.Filepath)
	if name == "" {
		return ""
	}
	name = filepath.Base(name)
	if ext := filepath.Ext(name); ext != "" {
		name = strings.TrimSuffix(name, ext)
	}
	return strings.TrimSpace(name)
}

func compatPlatformRetrySuffix(rec model.DownloadTask) string {
	if ext := filepath.Ext(strings.TrimSpace(rec.Filepath)); ext != "" {
		return ext
	}
	return ""
}

type platformPlanExecution struct {
	Resolved       *contentdownload.ResolvedRequest
	DownloadTaskID string
	Account        *model.Account
}

func (c *APIClient) runPlatformPlanNodeTypes(ctx context.Context, run *platformWorkflowRun, exec *platformPlanExecution, nodeTypes map[string]bool) error {
	if exec == nil || exec.Resolved == nil || exec.Resolved.Pipeline == nil || len(nodeTypes) == 0 {
		return nil
	}
	nodes := exec.Resolved.Pipeline.Nodes
	byID := make(map[string]contentdownload.PipelineNode, len(nodes))
	for _, node := range nodes {
		byID[node.ID] = node
	}
	visiting := make(map[string]bool, len(nodes))
	done := make(map[string]bool, len(nodes))
	var runNode func(string) error
	runNode = func(id string) error {
		if done[id] {
			return nil
		}
		node, ok := byID[id]
		if !ok || !nodeTypes[node.Type] {
			return nil
		}
		if visiting[id] {
			return fmt.Errorf("pipeline node cycle at %q", id)
		}
		visiting[id] = true
		for _, depID := range node.DependsOn {
			if err := runNode(depID); err != nil {
				return err
			}
		}
		visiting[id] = false
		if err := c.executePlatformPlanNode(ctx, run, exec, node); err != nil {
			return err
		}
		done[id] = true
		return nil
	}
	for _, node := range nodes {
		if !nodeTypes[node.Type] {
			continue
		}
		if err := runNode(node.ID); err != nil {
			return err
		}
	}
	return nil
}

func (c *APIClient) executePlatformPlanNode(ctx context.Context, run *platformWorkflowRun, exec *platformPlanExecution, node contentdownload.PipelineNode) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if run != nil {
		run.startNode(node.ID, node.Type)
	}
	var err error
	switch node.Type {
	case "create_account":
		exec.Account, err = c.upsertPlatformResolvedAccount(exec.Resolved)
	case "create_content":
		_, err = c.upsertPlatformResolvedContent(exec.Resolved, exec.DownloadTaskID, exec.Account)
	default:
		err = fmt.Errorf("unsupported platform pipeline node type %q", node.Type)
	}
	if err != nil {
		if run != nil {
			run.failNode(node.ID, err)
		}
		return err
	}
	if run != nil {
		run.completeNode(node.ID)
	}
	return nil
}

func (c *APIClient) upsertPlatformResolvedAccount(resolved *contentdownload.ResolvedRequest) (*model.Account, error) {
	if c.db == nil || resolved == nil {
		return nil, nil
	}
	summary := contentdownload.ContentSummaryOf(resolved.Content)
	metadata := contentdownload.ContentMetadataOf(resolved.Content)
	platformID := strings.TrimSpace(firstNonEmpty(summary.Platform, resolved.Platform))
	if platformID == "" {
		return nil, nil
	}
	externalID := strings.TrimSpace(firstNonEmpty(
		platformContentMetadataString(metadata, "account_external_id", "account_id", "author_id", "author_url_token", "author_username", "author_sec_id"),
		summary.Author,
		summary.AuthorNickname,
	))
	if externalID == "" {
		return nil, nil
	}
	username := strings.TrimSpace(firstNonEmpty(
		platformContentMetadataString(metadata, "account_username", "author_username", "author_url_token", "author_sec_id"),
		externalID,
	))
	nickname := strings.TrimSpace(firstNonEmpty(summary.AuthorNickname, summary.Author, username, externalID))
	avatarURL := strings.TrimSpace(summary.AuthorAvatarURL)
	now := util.NowMillis()
	var account *model.Account
	err := c.db.Transaction(func(tx *gorm.DB) error {
		next, err := upsertContentAccount(tx, model.Account{
			PlatformId: platformID,
			ExternalId: externalID,
			Username:   username,
			Nickname:   nickname,
			AvatarURL:  avatarURL,
		}, now)
		if err != nil {
			return err
		}
		account = next
		return nil
	})
	return account, err
}

func (c *APIClient) upsertPlatformResolvedContent(resolved *contentdownload.ResolvedRequest, downloadTaskID string, account *model.Account) (*model.Content, error) {
	if c.db == nil || resolved == nil {
		return nil, nil
	}
	if downloadTaskID != "" && c.db.Migrator().HasTable(&model.DownloadTask{}) {
		var children []model.DownloadTask
		if err := c.db.Where("parent_id = ?", downloadTaskID).Order("idx ASC").Find(&children).Error; err != nil {
			if !platformDownloadTaskTableMissing(err) {
				return nil, err
			}
		} else if len(children) > 0 {
			return c.upsertPlatformDownloadTaskFileContents(resolved, downloadTaskID, children, account)
		}
		var rec model.DownloadTask
		if err := c.db.First(&rec, "id = ?", downloadTaskID).Error; err == nil {
			return c.upsertPlatformDownloadTaskRecordContent(resolved, rec, account)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) && !platformDownloadTaskTableMissing(err) {
			return nil, err
		}
	}
	return c.upsertPlatformResolvedSingleContent(resolved, downloadTaskID, account)
}

func (c *APIClient) upsertPlatformResolvedSingleContent(resolved *contentdownload.ResolvedRequest, downloadTaskID string, account *model.Account) (*model.Content, error) {
	if c.db == nil || resolved == nil {
		return nil, nil
	}
	summary := contentdownload.ContentSummaryOf(resolved.Content)
	metadata := contentdownload.ContentMetadataOf(resolved.Content)
	platformID := strings.TrimSpace(firstNonEmpty(summary.Platform, resolved.Platform))
	contentID := strings.TrimSpace(firstNonEmpty(summary.ID, resolved.ContentID, resolved.Labels["id"]))
	if platformID == "" || contentID == "" {
		return nil, nil
	}
	sourceContentType := platformSourceContentType(resolved)
	outputFormat := platformResolvedOutputFormat(resolved)
	mimeType := platformResolvedMimeType(resolved)
	contentType := platformContentTypeFromOutput(outputFormat, mimeType, sourceContentType)
	metaBytes, _ := json.Marshal(map[string]any{
		"resolved":            resolved.Metadata,
		"content":             metadata,
		"labels":              resolved.Labels,
		"source_content_id":   contentID,
		"source_content_type": sourceContentType,
		"output_format":       outputFormat,
		"mime_type":           mimeType,
	})
	taskID := downloadTaskID
	now := util.NowMillis()
	content := model.Content{
		PlatformId:     platformID,
		ContentType:    contentType,
		ExternalId:     platformContentExternalID(contentID, platformResolvedOutputIdentity(resolved, outputFormat)),
		ExternalId2:    platformContentMetadataString(metadata, "nonce_id", "answer_id", "article_id", "book_id"),
		Title:          firstNonEmpty(summary.Title, resolved.Title, resolved.Filename, contentID),
		Description:    summary.Description,
		ContentURL:     firstNonEmpty(summary.URL, resolved.CanonicalURL, resolved.SourceURL),
		URL:            firstNonEmpty(summary.URL, resolved.Download.URL),
		SourceURL:      firstNonEmpty(summary.SourceURL, resolved.SourceURL, resolved.CanonicalURL),
		CoverURL:       summary.CoverURL,
		Metadata:       string(metaBytes),
		Duration:       summary.Duration,
		DownloadTaskId: &taskID,
		DownloadStatus: 1,
		DownloadPath:   platformTaskFilePath("", resolved),
		Timestamps:     model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
	err := c.db.Transaction(func(tx *gorm.DB) error {
		if err := upsertContentByPlatformExternalID(tx, &content, now); err != nil {
			return err
		}
		if account != nil {
			if err := upsertContentOwner(tx, content.Id, account.Id, now); err != nil {
				return err
			}
		}
		return nil
	})
	return &content, err
}

func (c *APIClient) upsertPlatformDownloadTaskRecordContent(resolved *contentdownload.ResolvedRequest, rec model.DownloadTask, account *model.Account) (*model.Content, error) {
	if c.db == nil || resolved == nil || rec.Id == "" {
		return nil, nil
	}
	now := util.NowMillis()
	content := platformContentFromDownloadTaskRecord(resolved, rec, "", now)
	err := c.db.Transaction(func(tx *gorm.DB) error {
		if err := upsertContentByPlatformExternalID(tx, &content, now); err != nil {
			return err
		}
		if account != nil {
			if err := upsertContentOwner(tx, content.Id, account.Id, now); err != nil {
				return err
			}
		}
		return nil
	})
	return &content, err
}

func (c *APIClient) upsertPlatformDownloadTaskFileContents(resolved *contentdownload.ResolvedRequest, parentTaskID string, children []model.DownloadTask, account *model.Account) (*model.Content, error) {
	if c.db == nil || resolved == nil || parentTaskID == "" || len(children) == 0 {
		return nil, nil
	}
	var parent model.DownloadTask
	_ = c.db.First(&parent, "id = ?", parentTaskID).Error
	now := util.NowMillis()
	var first model.Content
	hasFirst := false
	err := c.db.Transaction(func(tx *gorm.DB) error {
		if err := deletePlatformContainerContents(tx, parentTaskID); err != nil {
			return err
		}
		for _, child := range children {
			treePath := platformDownloadTaskTreePath(child)
			content := platformContentFromDownloadTaskRecord(resolved, child, treePath, now)
			if strings.TrimSpace(content.CoverURL) == "" {
				content.CoverURL = parent.CoverURL
			}
			if err := upsertContentByPlatformExternalID(tx, &content, now); err != nil {
				return err
			}
			if account != nil {
				if err := upsertContentOwner(tx, content.Id, account.Id, now); err != nil {
					return err
				}
			}
			if !hasFirst {
				first = content
				hasFirst = true
			}
		}
		return nil
	})
	if err != nil || !hasFirst {
		return nil, err
	}
	return &first, nil
}

func deletePlatformContainerContents(tx *gorm.DB, parentTaskID string) error {
	var ids []int
	if err := tx.Model(&model.Content{}).Where("download_task_id = ?", parentTaskID).Pluck("id", &ids).Error; err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	if err := tx.Where("content_id IN ?", ids).Delete(&model.ContentAccount{}).Error; err != nil {
		return err
	}
	return tx.Where("id IN ?", ids).Delete(&model.Content{}).Error
}

func platformDownloadTaskTableMissing(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "no such table") && strings.Contains(text, "download_task")
}

func platformContentFromDownloadTaskRecord(resolved *contentdownload.ResolvedRequest, rec model.DownloadTask, treePath string, now int64) model.Content {
	summary := contentdownload.ContentSummaryOf(resolved.Content)
	metadata := contentdownload.ContentMetadataOf(resolved.Content)
	platformID := strings.TrimSpace(firstNonEmpty(summary.Platform, resolved.Platform))
	sourceContentID := strings.TrimSpace(firstNonEmpty(summary.ID, resolved.ContentID, resolved.Labels["id"], rec.ExternalId, rec.TaskId))
	sourceContentType := platformSourceContentType(resolved)
	treePath = strings.TrimSpace(firstNonEmpty(treePath, platformDownloadTaskTreePath(rec)))
	outputFormat := platformDownloadTaskOutputFormat(rec, treePath, resolved)
	mimeType := firstNonEmpty(rec.MimeType, platformResolvedMimeType(resolved), platformMimeTypeFromOutputFormat(outputFormat))
	contentType := platformContentTypeFromOutput(outputFormat, mimeType, sourceContentType)
	downloadPath := firstNonEmpty(rec.Filepath, rec.OutputPath, platformDownloadTaskMetadataString(rec, "filepath"))
	taskID := rec.Id
	downloadTime := (*int64)(nil)
	if rec.Status == 4 {
		doneAt := rec.UpdatedAt
		if doneAt <= 0 {
			doneAt = now
		}
		downloadTime = &doneAt
	}
	size := rec.Size
	if size <= 0 {
		size = rec.Downloaded
	}
	metaBytes, _ := json.Marshal(map[string]any{
		"source_content_id":   sourceContentID,
		"source_content_type": sourceContentType,
		"output_format":       outputFormat,
		"mime_type":           mimeType,
		"tree_path":           treePath,
		"file_role":           platformDownloadTaskMetadataString(rec, "role"),
		"download_task_id":    rec.Id,
		"parent_download_id":  rec.ParentId,
		"labels":              resolved.Labels,
		"content":             metadata,
	})
	externalID := sourceContentID
	if treePath != "" {
		externalID = platformContentExternalID(sourceContentID, "file:"+filepath.ToSlash(treePath))
	} else {
		externalID = platformContentExternalID(sourceContentID, platformResolvedOutputIdentity(resolved, outputFormat))
	}
	return model.Content{
		PlatformId:     platformID,
		ContentType:    contentType,
		ExternalId:     externalID,
		ExternalId2:    sourceContentID,
		ExternalId3:    treePath,
		Title:          firstNonEmpty(rec.Title, rec.Filename, filepath.Base(treePath), summary.Title, resolved.Title, sourceContentID),
		Description:    summary.Description,
		ContentURL:     firstNonEmpty(summary.URL, resolved.CanonicalURL, resolved.SourceURL),
		URL:            firstNonEmpty(rec.SourceURI, rec.URL, summary.URL, resolved.Download.URL),
		SourceURL:      firstNonEmpty(summary.SourceURL, resolved.SourceURL, resolved.CanonicalURL),
		CoverURL:       firstNonEmpty(rec.CoverURL, summary.CoverURL),
		Metadata:       string(metaBytes),
		Duration:       summary.Duration,
		DownloadTaskId: &taskID,
		DownloadStatus: rec.Status,
		DownloadPath:   downloadPath,
		FileSize:       size,
		Size:           size,
		DownloadTime:   downloadTime,
		ErrorMsg:       rec.Error,
		Timestamps:     model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
}

func platformJSONMap(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	out := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func platformDownloadTaskMetadataString(rec model.DownloadTask, keys ...string) string {
	return platformContentMetadataString(platformJSONMap(firstNonEmpty(rec.Metadata, rec.Metadata2)), keys...)
}

func platformDownloadTaskTreePath(rec model.DownloadTask) string {
	if treePath := platformDownloadTaskMetadataString(rec, "tree_path"); treePath != "" {
		return treePath
	}
	if rec.ParentId != "" {
		return firstNonEmpty(rec.Filename, filepath.Base(rec.Filepath), filepath.Base(rec.OutputPath))
	}
	return ""
}

func platformDownloadTaskOutputFormat(rec model.DownloadTask, treePath string, resolved *contentdownload.ResolvedRequest) string {
	return firstNonEmpty(
		platformOutputFormatFromPath(treePath),
		platformOutputFormatFromPath(rec.Filename),
		platformOutputFormatFromPath(rec.Filepath),
		platformOutputFormatFromPath(rec.OutputPath),
		platformOutputFormatFromMimeType(rec.MimeType),
		platformResolvedOutputFormat(resolved),
	)
}

func platformSourceContentType(resolved *contentdownload.ResolvedRequest) string {
	if resolved == nil {
		return ""
	}
	summary := contentdownload.ContentSummaryOf(resolved.Content)
	return strings.TrimSpace(firstNonEmpty(summary.Type, resolved.Labels["content_type"], platformContentMetadataString(resolved.Metadata, "content_type")))
}

func platformResolvedOutputFormat(resolved *contentdownload.ResolvedRequest) string {
	if resolved == nil {
		return ""
	}
	return firstNonEmpty(
		platformOutputFormatFromSuffix(resolved.Suffix),
		platformOutputFormatFromSuffix(resolved.Labels["suffix"]),
		platformOutputFormatFromPath(resolved.Filename),
		platformOutputFormatFromPath(resolved.Download.URL),
		platformOutputFormatFromMimeType(firstNonEmpty(resolved.Labels["mime_type"], platformContentMetadataString(resolved.Metadata, "mime_type"))),
	)
}

func platformResolvedMimeType(resolved *contentdownload.ResolvedRequest) string {
	if resolved == nil {
		return ""
	}
	mimeType := firstNonEmpty(resolved.Labels["mime_type"], platformContentMetadataString(resolved.Metadata, "mime_type"))
	if mimeType != "" {
		return mimeType
	}
	return platformMimeTypeFromOutputFormat(firstNonEmpty(
		platformOutputFormatFromSuffix(resolved.Suffix),
		platformOutputFormatFromSuffix(resolved.Labels["suffix"]),
		platformOutputFormatFromPath(resolved.Filename),
	))
}

func platformResolvedOutputIdentity(resolved *contentdownload.ResolvedRequest, outputFormat string) string {
	if resolved == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	appendPart := func(value string) {
		value = strings.TrimSpace(strings.TrimPrefix(value, "."))
		if value == "" {
			return
		}
		for _, existing := range parts {
			if strings.EqualFold(existing, value) {
				return
			}
		}
		parts = append(parts, value)
	}
	appendPart(firstNonEmpty(platformContentMetadataString(resolved.Metadata, "variant_id"), resolved.Labels["variant_id"]))
	appendPart(firstNonEmpty(resolved.Labels["spec"], platformContentMetadataString(resolved.Metadata, "spec")))
	appendPart(outputFormat)
	if len(parts) == 0 {
		appendPart(firstNonEmpty(platformOutputFormatFromSuffix(resolved.Suffix), "download"))
	}
	return strings.Join(parts, ":")
}

func platformContentExternalID(sourceContentID string, identity string) string {
	sourceContentID = strings.TrimSpace(sourceContentID)
	identity = strings.TrimSpace(strings.ReplaceAll(identity, "#", "_"))
	if sourceContentID == "" || identity == "" {
		return sourceContentID
	}
	return sourceContentID + "#" + identity
}

func platformOutputFormatFromSuffix(suffix string) string {
	return platformNormalizeOutputFormat(strings.TrimPrefix(strings.TrimSpace(suffix), "."))
}

func platformOutputFormatFromPath(path string) string {
	ext := strings.TrimSpace(filepath.Ext(path))
	if ext == "" {
		return ""
	}
	return platformOutputFormatFromSuffix(ext)
}

func platformNormalizeOutputFormat(format string) string {
	format = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(format, ".")))
	switch format {
	case "jpeg":
		return "jpg"
	case "htm":
		return "html"
	default:
		return format
	}
}

func platformOutputFormatFromMimeType(mimeType string) string {
	mimeType = strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))
	switch mimeType {
	case "application/json", "text/json":
		return "json"
	case "text/html", "application/xhtml+xml":
		return "html"
	case "text/plain":
		return "txt"
	case "text/markdown":
		return "md"
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	case "image/gif":
		return "gif"
	case "audio/mpeg", "audio/mp3":
		return "mp3"
	case "audio/mp4":
		return "m4a"
	case "video/mp4":
		return "mp4"
	case "application/pdf":
		return "pdf"
	case "application/zip":
		return "zip"
	default:
		if slash := strings.Index(mimeType, "/"); slash >= 0 && slash+1 < len(mimeType) {
			return platformNormalizeOutputFormat(mimeType[slash+1:])
		}
		return ""
	}
}

func platformMimeTypeFromOutputFormat(format string) string {
	switch platformNormalizeOutputFormat(format) {
	case "json":
		return "application/json"
	case "html":
		return "text/html"
	case "txt":
		return "text/plain"
	case "md":
		return "text/markdown"
	case "jpg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	case "gif":
		return "image/gif"
	case "mp3":
		return "audio/mpeg"
	case "m4a", "aac":
		return "audio/mp4"
	case "wav":
		return "audio/wav"
	case "flac":
		return "audio/flac"
	case "mp4", "m4s":
		return "video/mp4"
	case "webm":
		return "video/webm"
	case "pdf":
		return "application/pdf"
	case "zip":
		return "application/zip"
	default:
		return ""
	}
}

func platformContentTypeFromOutput(format string, mimeType string, fallback string) string {
	format = platformNormalizeOutputFormat(format)
	switch format {
	case "json":
		return "json"
	case "html":
		return "html"
	case "txt", "md":
		return "text"
	case "jpg", "png", "webp", "gif", "bmp", "tiff":
		return "image"
	case "mp3", "m4a", "aac", "wav", "flac", "ogg":
		return "audio"
	case "mp4", "m4s", "mov", "mkv", "webm", "flv":
		return "video"
	case "zip", "rar", "7z", "tar", "gz":
		return "archive"
	case "pdf":
		return "pdf"
	}
	mimeType = strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))
	switch {
	case mimeType == "application/json" || mimeType == "text/json":
		return "json"
	case mimeType == "text/html" || mimeType == "application/xhtml+xml":
		return "html"
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	case strings.HasPrefix(mimeType, "audio/"):
		return "audio"
	case strings.HasPrefix(mimeType, "video/"):
		return "video"
	case strings.HasPrefix(mimeType, "text/"):
		return "text"
	}
	if strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	return "file"
}

func platformContentMetadataString(metadata map[string]any, keys ...string) string {
	for _, key := range keys {
		if metadata == nil {
			return ""
		}
		value := metadata[key]
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return v
			}
		case fmt.Stringer:
			text := strings.TrimSpace(v.String())
			if text != "" {
				return text
			}
		case nil:
		default:
			text := strings.TrimSpace(fmt.Sprint(v))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func platformContentTypeFromSuffix(suffix string) string {
	format := platformOutputFormatFromSuffix(suffix)
	if format == "" {
		return "file"
	}
	return platformContentTypeFromOutput(format, platformMimeTypeFromOutputFormat(format), "")
}

func platformEnsureMetadataPipelineNodes(resolved *contentdownload.ResolvedRequest) {
	if resolved == nil {
		return
	}
	if resolved.Pipeline == nil {
		resolved.Pipeline = &contentdownload.PipelinePlan{Platform: resolved.Platform}
	}
	if strings.TrimSpace(resolved.Pipeline.Platform) == "" {
		resolved.Pipeline.Platform = resolved.Platform
	}
	accountID := platformPipelineNodeIDByType(resolved.Pipeline, "create_account")
	if accountID == "" {
		accountID = "create_account"
		resolved.Pipeline.Nodes = append(resolved.Pipeline.Nodes, contentdownload.PipelineNode{
			ID:    accountID,
			Type:  "create_account",
			Stage: "prepare",
		})
	}
	if platformPipelineNodeIDByType(resolved.Pipeline, "create_content") == "" {
		resolved.Pipeline.Nodes = append(resolved.Pipeline.Nodes, contentdownload.PipelineNode{
			ID:        "create_content",
			Type:      "create_content",
			Stage:     "prepare",
			DependsOn: []string{accountID},
		})
	}
}

func platformPipelineNodeIDByType(plan *contentdownload.PipelinePlan, nodeType string) string {
	if plan == nil {
		return ""
	}
	for _, node := range plan.Nodes {
		if node.Type == nodeType && strings.TrimSpace(node.ID) != "" {
			return node.ID
		}
	}
	return ""
}

func (c *APIClient) lookupPlatformWorkflow(runID string) *platformWorkflowRun {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil
	}
	value, ok := platformWorkflowRuns.Load(runID)
	if !ok {
		run, err := c.loadPlatformWorkflowRun(runID)
		if err != nil {
			return nil
		}
		return run
	}
	run, ok := value.(*platformWorkflowRun)
	if !ok || run == nil {
		platformWorkflowRuns.Delete(runID)
		return nil
	}
	c.attachPlatformWorkflowPersistence(run)
	return run
}

func (c *APIClient) runPlatformDownloadTask(active *platformActiveDownload) {
	if active == nil || active.downloader == nil || active.taskID == "" {
		return
	}
	ctx, ok := active.begin()
	if !ok {
		return
	}
	paused := false
	defer func() {
		shouldResume := active.finish(paused)
		if shouldResume {
			go c.runPlatformDownloadTask(active)
			return
		}
		if !paused {
			platformActiveDownloads.Delete(active.taskID)
		}
	}()

	recID := active.recID
	run := active.run
	downloader := active.downloader
	taskID := active.taskID
	_ = c.updatePlatformDownloadTask(recID, map[string]any{
		"status":     1,
		"updated_at": util.NowMillis(),
	})
	task := downloader.GetTask(taskID)
	downloadNodeID, downloadNodeType := platformPipelineStageNode(task, "download", "download", "download_asset")
	if run != nil {
		run.startNode(downloadNodeID, downloadNodeType)
	}
	if task != nil {
		c.broadcastPlatformDownloadTask(task, recID)
	}
	err := downloader.Start(ctx, taskID)
	task = downloader.GetTask(taskID)
	if err != nil && errors.Is(err, context.Canceled) && active.isPaused() {
		paused = true
		task = downloader.MarkPaused(taskID)
		_ = c.updatePlatformDownloadTask(recID, map[string]any{
			"status":     2,
			"error":      "",
			"updated_at": util.NowMillis(),
		})
		if task != nil {
			c.syncPlatformDownloadTaskChildren(recID, task)
			c.broadcastPlatformDownloadTask(task, recID)
		}
		return
	}
	if run != nil && err == nil {
		run.completeNode(downloadNodeID)
	}
	if err == nil && task == nil {
		err = fmt.Errorf("下载任务不存在: %s", taskID)
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
			c.syncPlatformDownloadTaskChildren(recID, task)
			c.broadcastPlatformDownloadTask(task, recID)
		}
		return
	}
	persistNodeID, persistNodeType := platformPipelineStageNode(task, "persist", "persist", "persist")
	if run != nil {
		run.startNode(persistNodeID, persistNodeType)
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
		"status":      4,
		"progress":    progress,
		"filepath":    task.FilePath,
		"output_path": "",
		"size":        contentdownload.FileNodesSize(platformTaskFiles(task)),
		"downloaded":  task.Progress.DownloadedBytes,
		"speed":       int64(0),
		"metadata":    c.platformTaskMetadataJSON(task),
		"metadata2":   c.platformTaskMetadataJSON(task),
		"updated_at":  util.NowMillis(),
	})
	if task != nil {
		c.syncPlatformDownloadTaskChildren(recID, task)
		c.broadcastPlatformDownloadTask(task, recID)
	}
	if run != nil {
		run.completeNode(persistNodeID)
		run.finish()
	}
}

func platformPipelineStageNode(task *contentdownload.Task, stage string, fallbackID string, fallbackType string) (string, string) {
	if task == nil || task.Resolved == nil || task.Resolved.Pipeline == nil {
		return fallbackID, fallbackType
	}
	for _, node := range task.Resolved.Pipeline.Nodes {
		if strings.EqualFold(strings.TrimSpace(node.Stage), stage) && strings.TrimSpace(node.ID) != "" {
			return node.ID, firstNonEmpty(node.Type, fallbackType)
		}
	}
	for _, node := range task.Resolved.Pipeline.Nodes {
		if strings.EqualFold(strings.TrimSpace(node.ID), fallbackID) {
			return node.ID, firstNonEmpty(node.Type, fallbackType)
		}
	}
	return fallbackID, fallbackType
}

func (c *APIClient) broadcastPlatformDownloadTask(task *contentdownload.Task, recID string) {
	if c.downloader_ws == nil || task == nil || task.Resolved == nil {
		return
	}
	status := "ready"
	switch task.Status {
	case contentdownload.TaskStatusResolving:
		status = "wait"
	case contentdownload.TaskStatusDownloading, contentdownload.TaskStatusProcessing:
		status = "running"
	case contentdownload.TaskStatusPaused:
		status = "paused"
	case contentdownload.TaskStatusDone:
		status = "done"
	case contentdownload.TaskStatusError:
		status = "error"
	}
	name := filepath.Base(task.FilePath)
	dir := filepath.Dir(task.FilePath)
	files := platformTaskFiles(task)
	size := contentdownload.FileNodesSize(files)
	if size == 0 {
		size = task.Progress.TotalBytes
	}
	c.downloader_ws.Broadcast(APIClientWSMessage{
		Type: "event",
		Data: map[string]any{
			"download_task_id": recID,
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
						"name":       name,
						"size":       size,
						"file_count": contentdownload.FileNodesCount(files),
						"files":      files,
					},
				},
				"progress": map[string]any{
					"downloaded": task.Progress.DownloadedBytes,
					"total":      task.Progress.TotalBytes,
					"speed":      0,
				},
			},
		},
	})
}

func (c *APIClient) pauseActivePlatformDownload(taskID string) bool {
	value, ok := platformActiveDownloads.Load(strings.TrimSpace(taskID))
	if !ok {
		return false
	}
	active, ok := value.(*platformActiveDownload)
	if !ok || active == nil {
		platformActiveDownloads.Delete(taskID)
		return false
	}
	active.pause()
	task := active.downloader.MarkPaused(active.taskID)
	_ = c.updatePlatformDownloadTask(active.recID, map[string]any{
		"status":     2,
		"error":      "",
		"updated_at": util.NowMillis(),
	})
	if task != nil {
		c.broadcastPlatformDownloadTask(task, active.recID)
	}
	return true
}

func (c *APIClient) resumeActivePlatformDownload(taskID string) bool {
	value, ok := platformActiveDownloads.Load(strings.TrimSpace(taskID))
	if !ok {
		return false
	}
	active, ok := value.(*platformActiveDownload)
	if !ok || active == nil {
		platformActiveDownloads.Delete(taskID)
		return false
	}
	startNow, ok := active.requestResume()
	if !ok {
		return true
	}
	if startNow {
		go c.runPlatformDownloadTask(active)
	}
	return true
}

func platformTaskFiles(task *contentdownload.Task) []contentdownload.FileNode {
	if task == nil {
		return nil
	}
	if len(task.Files) > 0 {
		return contentdownload.FileNodesWithOutputPath(task.FilePath, task.Files)
	}
	status := contentdownload.FileNodeStatusPending
	switch task.Status {
	case contentdownload.TaskStatusDownloading, contentdownload.TaskStatusProcessing:
		status = contentdownload.FileNodeStatusDownloading
	case contentdownload.TaskStatusPaused:
		status = contentdownload.FileNodeStatusPending
	case contentdownload.TaskStatusDone:
		status = contentdownload.FileNodeStatusDone
	case contentdownload.TaskStatusError:
		status = contentdownload.FileNodeStatusError
	}
	return contentdownload.SingleFileNodes(task.FilePath, task.Progress.TotalBytes, status)
}

func platformTaskNodeType(task *contentdownload.Task) string {
	if contentdownload.FileNodesCount(platformTaskFiles(task)) > 1 {
		return downloadNodeTypeContainer
	}
	return downloadNodeTypeFile
}

func (c *APIClient) platformTaskEngine(task *contentdownload.Task) string {
	if platformTaskNodeType(task) == downloadNodeTypeContainer {
		return ""
	}
	if task != nil && task.Resolved != nil && strings.EqualFold(task.Resolved.Platform, contentshuba69.PlatformID) {
		return c.shuba69DownloadEngine()
	}
	return downloadEngineFromSpec(task)
}

func platformTaskRecordProtocol(task *contentdownload.Task) string {
	if platformTaskNodeType(task) == downloadNodeTypeContainer {
		return ""
	}
	if task == nil || task.Resolved == nil {
		return ""
	}
	return genericDownloadProtocol(task.Resolved.Download.Protocol, task.Resolved.Download.URL)
}

func platformTaskRecordURL(task *contentdownload.Task) string {
	if platformTaskNodeType(task) == downloadNodeTypeContainer {
		return ""
	}
	if task == nil || task.Resolved == nil {
		return ""
	}
	return task.Resolved.Download.URL
}

func platformTaskSourceURI(task *contentdownload.Task) string {
	if platformTaskNodeType(task) == downloadNodeTypeContainer {
		return ""
	}
	if task == nil || task.Resolved == nil {
		return ""
	}
	return firstNonEmpty(task.Resolved.SourceURL, task.Resolved.CanonicalURL, task.Resolved.Download.URL)
}

func downloadEngineFromSpec(task *contentdownload.Task) string {
	if task == nil || task.Resolved == nil {
		return ""
	}
	protocol := strings.ToLower(strings.TrimSpace(task.Resolved.Download.Protocol))
	if protocol == "" {
		protocol = genericDownloadProtocol("", task.Resolved.Download.URL)
	}
	switch protocol {
	case "inline_html", "inline_json", "file":
		return downloadEngineFS
	default:
		return downloadEngineHTTP
	}
}

func genericDownloadProtocol(protocol string, rawURL string) string {
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	if protocol != "" {
		switch protocol {
		case contentshuba69.ArchiveProtocol, contentttk.ArchiveProtocol:
			return ""
		case "inline_html", "inline_json":
			return "file"
		default:
			return protocol
		}
	}
	rawURL = strings.TrimSpace(rawURL)
	if idx := strings.Index(rawURL, "://"); idx > 0 {
		return strings.ToLower(rawURL[:idx])
	}
	return ""
}

func platformTaskDownloadMetadata(task *contentdownload.Task) map[string]any {
	if task == nil || task.Resolved == nil || platformTaskNodeType(task) == downloadNodeTypeContainer {
		return nil
	}
	return map[string]any{
		"protocol": genericDownloadProtocol(task.Resolved.Download.Protocol, task.Resolved.Download.URL),
		"url":      task.Resolved.Download.URL,
		"method":   firstNonEmpty(task.Resolved.Download.Method, "GET"),
	}
}

func (c *APIClient) platformTaskFetchMetadata(task *contentdownload.Task) map[string]any {
	if task == nil || task.Resolved == nil {
		return nil
	}
	if strings.EqualFold(task.Resolved.Platform, contentshuba69.PlatformID) {
		return map[string]any{
			"engine":    c.shuba69DownloadEngine(),
			"purpose":   "fetch_html",
			"cf_bypass": c.shuba69DownloadEngine() != downloadEngineHTTP,
		}
	}
	return map[string]any{
		"engine": downloadEngineFromSpec(task),
	}
}

func (c *APIClient) shuba69DownloadEngine() string {
	if c == nil || c.cfg == nil {
		return downloadEngineClawreq
	}
	switch strings.ToLower(strings.TrimSpace(c.cfg.Shuba69Fetcher)) {
	case "", "clawreq":
		return downloadEngineClawreq
	case "http":
		return downloadEngineHTTP
	case "cdp":
		return downloadEngineCDP
	case "sandbox":
		if strings.TrimSpace(c.cfg.Shuba69SandboxID) == "" {
			return downloadEngineBrowserPoolCDP
		}
		return downloadEngineSandboxCDP
	default:
		return downloadEngineClawreq
	}
}

func (c *APIClient) platformTaskMetadataJSON(task *contentdownload.Task) string {
	if task == nil || task.Resolved == nil {
		return "{}"
	}
	resolved := task.Resolved
	files := platformTaskFiles(task)
	summary := contentdownload.ContentSummaryOf(resolved.Content)
	outputFormat := platformResolvedOutputFormat(resolved)
	mimeType := platformResolvedMimeType(resolved)
	metaBytes, _ := json.Marshal(map[string]any{
		"source": map[string]any{
			"platform":      resolved.Platform,
			"content_id":    resolved.ContentID,
			"source_url":    resolved.SourceURL,
			"canonical_url": resolved.CanonicalURL,
		},
		"summary": map[string]any{
			"title":       firstNonEmpty(summary.Title, resolved.Title),
			"author":      firstNonEmpty(summary.Author, summary.AuthorNickname),
			"description": summary.Description,
			"cover_url":   summary.CoverURL,
		},
		"aggregate": map[string]any{
			"file_count": contentdownload.FileNodesCount(files),
			"size":       contentdownload.FileNodesSize(files),
		},
		"output": map[string]any{
			"format":              outputFormat,
			"mime_type":           mimeType,
			"content_type":        platformContentTypeFromOutput(outputFormat, mimeType, platformSourceContentType(resolved)),
			"source_content_type": platformSourceContentType(resolved),
		},
		"download": platformTaskDownloadMetadata(task),
		"fetch":    c.platformTaskFetchMetadata(task),
		"metadata": resolved.Metadata,
		"labels":   resolved.Labels,
		"files":    files,
	})
	return string(metaBytes)
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

func resolvedRequiresFFmpeg(resolved *contentdownload.ResolvedRequest) bool {
	if resolved == nil {
		return false
	}
	if resolved.Suffix == ".mp3" || resolved.Download.Protocol == contentdownload.ProtocolMultiHTTP {
		return true
	}
	if resolved.Metadata != nil {
		if value, ok := resolved.Metadata["requires_ffmpeg"].(bool); ok && value {
			return true
		}
	}
	return false
}

func platformOptionsWithRuntimeCapabilities(options contentdownload.Options) contentdownload.Options {
	if options.Extra == nil {
		options.Extra = map[string]any{}
	}
	if _, ok := options.Extra["ffmpeg_available"]; !ok {
		options.Extra["ffmpeg_available"] = system.ExistingCommand("ffmpeg")
	}
	return options
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
	metadata := c.platformTaskMetadataJSON(task)
	progressBytes, _ := json.Marshal(map[string]any{
		"downloaded": 0,
		"total":      0,
		"speed":      0,
	})
	rec := model.DownloadTask{
		TaskUID:      task.ID,
		TaskId:       task.ID,
		NodeType:     platformTaskNodeType(task),
		Engine:       c.platformTaskEngine(task),
		EngineTaskID: task.ID,
		Status:       1,
		ExternalId:   resolved.ContentID,
		Protocol:     platformTaskRecordProtocol(task),
		URL:          platformTaskRecordURL(task),
		SourceURI:    platformTaskSourceURI(task),
		Method:       firstNonEmpty(resolved.Download.Method, "GET"),
		Title:        firstNonEmpty(resolved.Title, resolved.Filename, resolved.ContentID),
		Filename:     firstNonEmpty(resolved.Filename, resolved.Title, resolved.ContentID),
		CoverURL:     contentCoverURL(resolved),
		MimeType:     platformResolvedMimeType(resolved),
		Size:         contentdownload.FileNodesSize(platformTaskFiles(task)),
		Progress:     string(progressBytes),
		Filepath:     task.FilePath,
		Reason:       "platform",
		Metadata:     metadata,
		Metadata2:    metadata,
		Timestamps:   model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
	if err := c.db.Create(&rec).Error; err != nil {
		return nil, err
	}
	_ = c.db.Model(&model.DownloadTask{}).Where("id = ?", rec.Id).Updates(map[string]any{
		"root_id":    rec.Id,
		"updated_at": util.NowMillis(),
	}).Error
	rec.RootId = rec.Id
	return &rec, nil
}

func (c *APIClient) updatePlatformDownloadTask(id string, updates map[string]any) error {
	if c.db == nil {
		return nil
	}
	return c.db.Model(&model.DownloadTask{}).Where("id = ?", id).Updates(updates).Error
}

func (c *APIClient) syncPlatformDownloadTaskChildren(parentID string, task *contentdownload.Task) {
	if c.db == nil || parentID == "" || task == nil {
		return
	}
	files := platformTaskFiles(task)
	if contentdownload.FileNodesCount(files) <= 1 {
		c.syncPlatformSingleDownloadTaskContent(parentID, task)
		return
	}
	var parent model.DownloadTask
	if err := c.db.First(&parent, "id = ?", parentID).Error; err != nil {
		return
	}
	rootID := parent.RootId
	if rootID == "" {
		rootID = parent.Id
	}
	leaves := platformFileNodeLeaves(files)
	sourceURIs := platformFileNodeSourceURIs(task, leaves)
	childEngine := c.platformTaskFileEngine(task)
	for index, leaf := range leaves {
		treePath := strings.TrimSpace(leaf.Path)
		if treePath == "" {
			treePath = strings.TrimSpace(leaf.Name)
		}
		if treePath == "" {
			continue
		}
		outputPath := strings.TrimSpace(leaf.OutputPath)
		if outputPath == "" {
			outputPath = downloadTaskFullPath(parent.Filepath, treePath)
		}
		sourceURI := firstNonEmpty(sourceURIs[treePath], platformSourceURLFromMetadata(firstNonEmpty(parent.Metadata, parent.Metadata2)))
		childMeta := platformFileNodeMetadata(task, parent, leaf, index, treePath, outputPath, sourceURI, childEngine)
		metaBytes, _ := json.Marshal(childMeta)
		taskID := platformFileNodeTaskID(parent.Id, treePath)
		updates := map[string]any{
			"task_uid":       taskID,
			"parent_id":      parent.Id,
			"root_id":        rootID,
			"node_type":      downloadNodeTypeFile,
			"engine":         childEngine,
			"engine_task_id": "",
			"type":           parent.Type,
			"status":         platformFileNodeStatus(leaf.Status),
			"external_id":    parent.ExternalId,
			"protocol":       "file",
			"url":            sourceURI,
			"source_uri":     sourceURI,
			"method":         "GET",
			"title":          firstNonEmpty(leaf.Name, filepath.Base(treePath), treePath),
			"filename":       firstNonEmpty(leaf.Name, filepath.Base(treePath), treePath),
			"cover_url":      parent.CoverURL,
			"mime_type":      platformFileNodeMimeType(treePath),
			"size":           leaf.Size,
			"downloaded":     platformFileNodeDownloaded(leaf),
			"speed":          int64(0),
			"progress":       platformFileNodeProgress(leaf),
			"filepath":       outputPath,
			"output_path":    "",
			"error":          leaf.Error,
			"reason":         "platform_file",
			"metadata":       string(metaBytes),
			"metadata2":      string(metaBytes),
			"idx":            index,
			"updated_at":     util.NowMillis(),
		}
		var child model.DownloadTask
		err := c.db.First(&child, "task_id = ?", taskID).Error
		if err == nil {
			_ = c.db.Model(&model.DownloadTask{}).Where("id = ?", child.Id).Updates(updates).Error
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		now := util.NowMillis()
		child = model.DownloadTask{
			TaskUID:    taskID,
			TaskId:     taskID,
			ParentId:   parent.Id,
			RootId:     rootID,
			NodeType:   downloadNodeTypeFile,
			Engine:     childEngine,
			Type:       parent.Type,
			Status:     platformFileNodeStatus(leaf.Status),
			ExternalId: parent.ExternalId,
			Protocol:   "file",
			URL:        sourceURI,
			SourceURI:  sourceURI,
			Method:     "GET",
			Title:      firstNonEmpty(leaf.Name, filepath.Base(treePath), treePath),
			Filename:   firstNonEmpty(leaf.Name, filepath.Base(treePath), treePath),
			CoverURL:   parent.CoverURL,
			MimeType:   platformFileNodeMimeType(treePath),
			Size:       leaf.Size,
			Downloaded: platformFileNodeDownloaded(leaf),
			Progress:   platformFileNodeProgress(leaf),
			Filepath:   outputPath,
			Error:      leaf.Error,
			Reason:     "platform_file",
			Metadata:   string(metaBytes),
			Metadata2:  string(metaBytes),
			Idx:        index,
			Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
		}
		_ = c.db.Create(&child).Error
	}
	if err := c.syncPlatformDownloadTaskFileContents(parent.Id, task); err != nil && c.logger != nil {
		c.logger.Warn().Err(err).Str("download_task_id", parent.Id).Msg("sync platform file contents failed")
	}
}

func (c *APIClient) syncPlatformSingleDownloadTaskContent(taskID string, task *contentdownload.Task) {
	if c.db == nil || taskID == "" || task == nil || task.Resolved == nil {
		return
	}
	var rec model.DownloadTask
	if err := c.db.First(&rec, "id = ?", taskID).Error; err != nil {
		return
	}
	account, err := c.upsertPlatformResolvedAccount(task.Resolved)
	if err != nil {
		if c.logger != nil {
			c.logger.Warn().Err(err).Str("download_task_id", taskID).Msg("sync platform content account failed")
		}
		return
	}
	if _, err := c.upsertPlatformDownloadTaskRecordContent(task.Resolved, rec, account); err != nil && c.logger != nil {
		c.logger.Warn().Err(err).Str("download_task_id", taskID).Msg("sync platform content failed")
	}
}

func (c *APIClient) syncPlatformDownloadTaskFileContents(parentID string, task *contentdownload.Task) error {
	if c.db == nil || parentID == "" || task == nil || task.Resolved == nil {
		return nil
	}
	var children []model.DownloadTask
	if err := c.db.Where("parent_id = ?", parentID).Order("idx ASC").Find(&children).Error; err != nil {
		return err
	}
	if len(children) == 0 {
		return nil
	}
	account, err := c.upsertPlatformResolvedAccount(task.Resolved)
	if err != nil {
		return err
	}
	_, err = c.upsertPlatformDownloadTaskFileContents(task.Resolved, parentID, children, account)
	return err
}

func platformFileNodeLeaves(files []contentdownload.FileNode) []contentdownload.FileNode {
	out := make([]contentdownload.FileNode, 0)
	var walk func([]contentdownload.FileNode)
	walk = func(nodes []contentdownload.FileNode) {
		for _, node := range nodes {
			if len(node.Children) > 0 {
				walk(node.Children)
				continue
			}
			out = append(out, node)
		}
	}
	walk(files)
	return out
}

func (c *APIClient) platformTaskFileEngine(task *contentdownload.Task) string {
	if task != nil && task.Resolved != nil && strings.EqualFold(task.Resolved.Platform, contentshuba69.PlatformID) {
		return c.shuba69DownloadEngine()
	}
	return downloadEngineFromSpec(task)
}

func platformFileNodeSourceURIs(task *contentdownload.Task, leaves []contentdownload.FileNode) map[string]string {
	out := make(map[string]string, len(leaves))
	if task == nil || task.Resolved == nil {
		return out
	}
	resolved := task.Resolved
	if strings.EqualFold(resolved.Platform, contentshuba69.PlatformID) {
		bookURL := firstNonEmpty(resolved.SourceURL, resolved.CanonicalURL)
		catalogURL := firstNonEmpty(toCompatString(resolved.Metadata["full_catalog_url"]), resolved.CanonicalURL, resolved.SourceURL)
		chapters := shuba69ChaptersFromResolved(resolved)
		chapterIndex := 0
		for _, leaf := range leaves {
			treePath := strings.TrimSpace(leaf.Path)
			switch {
			case treePath == "source/book.html":
				out[treePath] = bookURL
			case treePath == "source/full_catalog.html":
				out[treePath] = catalogURL
			case strings.HasPrefix(treePath, "chapters/"):
				if chapterIndex < len(chapters) {
					out[treePath] = chapters[chapterIndex].URL
				}
				chapterIndex++
			}
		}
		return out
	}
	for _, leaf := range leaves {
		treePath := strings.TrimSpace(leaf.Path)
		if treePath != "" {
			out[treePath] = firstNonEmpty(resolved.SourceURL, resolved.CanonicalURL, resolved.Download.URL)
		}
	}
	return out
}

func shuba69ChaptersFromResolved(resolved *contentdownload.ResolvedRequest) []contentshuba69.Chapter {
	if resolved == nil {
		return nil
	}
	data := contentdownload.ContentDataOf(resolved.Content)
	switch value := data.(type) {
	case *contentshuba69.Novel:
		if value != nil {
			return value.Chapters
		}
	case contentshuba69.Novel:
		return value.Chapters
	case *contentshuba69.NovelFetchResult:
		if value != nil && value.Novel != nil {
			return value.Novel.Chapters
		}
	case contentshuba69.NovelFetchResult:
		if value.Novel != nil {
			return value.Novel.Chapters
		}
	}
	if data != nil {
		if bytes, err := json.Marshal(data); err == nil {
			var novel contentshuba69.Novel
			if err := json.Unmarshal(bytes, &novel); err == nil && len(novel.Chapters) > 0 {
				return novel.Chapters
			}
			var fetch contentshuba69.NovelFetchResult
			if err := json.Unmarshal(bytes, &fetch); err == nil && fetch.Novel != nil {
				return fetch.Novel.Chapters
			}
		}
	}
	return nil
}

func platformFileNodeMetadata(task *contentdownload.Task, parent model.DownloadTask, leaf contentdownload.FileNode, index int, treePath string, outputPath string, sourceURI string, engine string) map[string]any {
	role := platformFileNodeRole(treePath)
	meta := map[string]any{
		"kind":            "file",
		"role":            role,
		"tree_path":       treePath,
		"parent_id":       parent.Id,
		"parent_task_uid": firstNonEmpty(parent.TaskUID, parent.TaskId),
		"source_uri":      sourceURI,
		"download": map[string]any{
			"engine": engine,
			"uri":    sourceURI,
			"method": "GET",
		},
		"fetch": map[string]any{
			"engine": engine,
		},
	}
	if strings.TrimSpace(outputPath) != "" {
		meta["filepath"] = outputPath
	}
	if task != nil && task.Resolved != nil && strings.EqualFold(task.Resolved.Platform, contentshuba69.PlatformID) {
		fetch, _ := meta["fetch"].(map[string]any)
		fetch["cf_bypass"] = engine != downloadEngineHTTP
		if role == "chapter" {
			chapterIndex, chapterTitle := shuba69ChapterInfoFromFileNode(leaf, index)
			meta["chapter_index"] = chapterIndex
			meta["chapter_title"] = chapterTitle
			meta["chapter_url"] = sourceURI
		}
	}
	return meta
}

func platformFileNodeRole(treePath string) string {
	switch treePath {
	case "source/book.html":
		return "source_page"
	case "source/full_catalog.html":
		return "catalog_page"
	default:
		if strings.HasPrefix(treePath, "chapters/") {
			return "chapter"
		}
		return "file"
	}
}

func shuba69ChapterInfoFromFileNode(leaf contentdownload.FileNode, index int) (int, string) {
	name := firstNonEmpty(leaf.Name, filepath.Base(leaf.Path))
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if underscore := strings.Index(base, "_"); underscore > 0 {
		if n, err := strconv.Atoi(strings.TrimLeft(base[:underscore], "0")); err == nil {
			return n, strings.TrimSpace(base[underscore+1:])
		}
	}
	return index + 1, strings.TrimSpace(base)
}

func platformFileNodeMimeType(treePath string) string {
	switch strings.ToLower(filepath.Ext(treePath)) {
	case ".html", ".htm":
		return "text/html"
	case ".json":
		return "application/json"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	default:
		return ""
	}
}

func platformFileNodeDownloaded(leaf contentdownload.FileNode) int64 {
	if strings.EqualFold(strings.TrimSpace(leaf.Status), contentdownload.FileNodeStatusDone) {
		return leaf.Size
	}
	return 0
}

func platformFileNodeProgress(leaf contentdownload.FileNode) string {
	downloaded := platformFileNodeDownloaded(leaf)
	progressBytes, _ := json.Marshal(map[string]any{
		"downloaded": downloaded,
		"total":      leaf.Size,
		"speed":      0,
	})
	return string(progressBytes)
}

func platformFileNodeStatus(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case contentdownload.FileNodeStatusDownloading:
		return 1
	case "paused", "pause":
		return 2
	case contentdownload.FileNodeStatusDone:
		return 4
	case contentdownload.FileNodeStatusError, "failed", "fail":
		return 5
	default:
		return 0
	}
}

func platformFileNodeTaskID(parentID string, treePath string) string {
	sum := sha1.Sum([]byte(treePath))
	return fmt.Sprintf("download-%d-file-%s", parentID, hex.EncodeToString(sum[:])[:16])
}

func platformSourceURLFromMetadata(raw string) string {
	var meta struct {
		SourceURL string         `json:"source_url"`
		URL       string         `json:"url"`
		Metadata  map[string]any `json:"metadata"`
		Source    map[string]any `json:"source"`
	}
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return ""
	}
	return firstNonEmpty(
		meta.SourceURL,
		meta.URL,
		toCompatString(meta.Source["source_url"]),
		toCompatString(meta.Source["canonical_url"]),
		toCompatString(meta.Metadata["source_url"]),
		toCompatString(meta.Metadata["canonical_url"]),
	)
}

func (c *APIClient) platformDownloadRouter() *contentdownload.Router {
	return contentdownload.NewRouter(
		contentwxchannels.New(platformChannelsFetcher{
			FeedProfileFetcher: c.channels,
			sphCookie:          c.cfg.CloudflareSphCookie,
		}),
		contentdouyin.New(nil),
		contentxiaohongshu.New(nil),
		contentzhihu.New(nil),
		contentoa.New(nil),
		contentbilibili.New(&contentbilibili.Client{Cookie: c.cfg.BilibiliCookie}),
		contentyoutube.New(&contentyoutube.Client{Cookie: c.cfg.YouTubeCookie, PoToken: c.cfg.YouTubePoToken}),
		contentinstagram.New(nil),
		contentweibo.New(weibopkg.NewClientWithOptions(nil, c.cfg.WeiboCookie, "")),
		contentx.New(nil),
		contenttmdb.New(nil),
		contentdouban.New(nil),
		contentiqiyi.New(nil),
		contentyouku.New(nil),
		contentmgtv.New(nil),
		contentqq.New(nil),
		c.shuba69Handler(),
		contentqidian.New(nil),
		contentquanben.New(nil),
		contentsoundgasm.New(nil),
		contenttelegram.New(nil),
		contentttk.New(nil),
		contentv2ex.New(nil),
		contentfanqie.New(nil),
		contentwebpage.New(nil),
	)
}

func (c *APIClient) shuba69Handler() *contentshuba69.Handler {
	cookie := ""
	fetcherName := ""
	cdpEndpoint := ""
	cdpTimeout := 0
	cdpWait := 0
	sandboxAPIBaseURL := ""
	sandboxID := ""
	if c != nil && c.cfg != nil {
		cookie = c.cfg.Shuba69Cookie
		fetcherName = c.cfg.Shuba69Fetcher
		cdpEndpoint = c.cfg.Shuba69CDPEndpoint
		cdpTimeout = c.cfg.Shuba69CDPTimeout
		cdpWait = c.cfg.Shuba69CDPWait
		sandboxAPIBaseURL = c.cfg.Shuba69SandboxAPIBaseURL
		sandboxID = c.cfg.Shuba69SandboxID
	}
	client := contentshuba69.NewClientWithOptions(cookie, "")
	switch strings.ToLower(strings.TrimSpace(fetcherName)) {
	case "", "clawreq":
		client = contentshuba69.NewClientWithOptions(cookie, "")
	case "http":
		client = contentshuba69.NewHTTPClientWithOptions(nil, cookie, "")
	case "cdp":
		cdpFetcher := contentshuba69.NewCDPFetcher(cdpEndpoint)
		if cdpTimeout > 0 {
			cdpFetcher.Timeout = time.Duration(cdpTimeout) * time.Second
		}
		if cdpWait >= 0 {
			cdpFetcher.WaitAfterLoad = time.Duration(cdpWait) * time.Second
		}
		client = contentshuba69.NewClientWithHTMLFetcher(cdpFetcher, cookie, "")
	case "sandbox":
		timeout := time.Duration(0)
		wait := time.Duration(cdpWait) * time.Second
		if cdpTimeout > 0 {
			timeout = time.Duration(cdpTimeout) * time.Second
		}
		if strings.TrimSpace(sandboxID) == "" {
			client = contentshuba69.NewClientWithHTMLFetcher(newShuba69BrowserPoolFetcher(c.browserMgr, timeout, wait), cookie, "")
		} else {
			sandboxFetcher := contentshuba69.NewSandboxCDPFetcher(sandboxAPIBaseURL, sandboxID)
			if timeout > 0 {
				sandboxFetcher.Timeout = timeout
			}
			if cdpWait >= 0 {
				sandboxFetcher.WaitAfterLoad = wait
			}
			client = contentshuba69.NewClientWithHTMLFetcher(sandboxFetcher, cookie, "")
		}
	}
	return contentshuba69.New(client)
}

type platformChannelsFetcher struct {
	contentwxchannels.FeedProfileFetcher
	sphCookie string
}

func (f platformChannelsFetcher) FetchChannelsSphProfile(reqURL string) (*contentwxchannels.SphProfile, error) {
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
	return &contentwxchannels.SphProfile{
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
	view := gin.H{
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
	if authorHomepageURL := platformContentAuthorHomepageURL(content); authorHomepageURL != "" {
		view["author_homepage_url"] = authorHomepageURL
	}
	return view
}

func platformContentAuthorHomepageURL(content any) string {
	return contentdownload.ContentAuthorHomepageURL(content)
}

func platformProbeOutput(probe *contentdownload.Probe) map[string]any {
	if probe == nil {
		return nil
	}
	contentOutput := contentdownload.ContentOutputOf(probe.Content)
	out := make(map[string]any, len(contentOutput))
	for k, v := range contentOutput {
		if platformProbeOutputOmitField(k) {
			continue
		}
		out[k] = v
	}
	if platformContentMetadataString(out, "author_homepage_url") == "" {
		if homepageURL := contentdownload.ContentAuthorHomepageURL(probe.Content); homepageURL != "" {
			out["author_homepage_url"] = homepageURL
		}
	}
	return out
}

func platformProbeOutputOmitField(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "content", "data", "raw_data":
		return true
	default:
		return false
	}
}

func platformProbePageJSON(probe *contentdownload.Probe) any {
	if probe == nil || probe.Internal == nil {
		return nil
	}
	value := probe.Internal["pagejson"]
	switch v := value.(type) {
	case nil:
		return nil
	case json.RawMessage:
		if len(v) == 0 {
			return nil
		}
	case []byte:
		if len(v) == 0 {
			return nil
		}
		return json.RawMessage(v)
	}
	return value
}

func platformProbePageHTML(probe *contentdownload.Probe) string {
	if probe == nil || probe.Internal == nil {
		return ""
	}
	value, _ := probe.Internal["pagehtml"].(string)
	return value
}

func platformProbePipeline(probe *contentdownload.Probe) any {
	if probe == nil || probe.Internal == nil {
		return nil
	}
	return probe.Internal["probe_pipeline"]
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
			"options":  platformProbeVariants(probe),
		},
		{
			"name":  "filename",
			"label": "文件名",
			"type":  "text",
			"value": contentdownload.ContentTitle(probe.Content),
		},
	}
	if probe.Platform == contentwxchannels.PlatformID {
		form = append(form, gin.H{
			"name":  "spec",
			"label": "清晰度参数",
			"type":  "hidden",
			"value": probe.Defaults.Spec,
		})
	}
	return form
}

func platformProbeAddJSONDefault(probe *contentdownload.Probe) {
	if probe == nil {
		return
	}
	if platformProbeJSONVariantDisabled(probe) {
		return
	}
	probe.Variants = platformProbeVariants(probe)
	if platformProbeKeepDefaultVariant(probe) {
		return
	}
	probe.Defaults.VariantID = platformJSONVariantID
	probe.Defaults.Suffix = ".json"
}

func platformProbeKeepDefaultVariant(probe *contentdownload.Probe) bool {
	if probe == nil {
		return false
	}
	return (probe.Platform == contentshuba69.PlatformID && contentdownload.ContentType(probe.Content) == "novel") ||
		probe.Platform == contentbilibili.PlatformID ||
		probe.Platform == contentsoundgasm.PlatformID
}

func platformProbeVariants(probe *contentdownload.Probe) []contentdownload.Variant {
	if probe == nil {
		return nil
	}
	variants := make([]contentdownload.Variant, 0, len(probe.Variants)+1)
	hasJSON := false
	for _, variant := range probe.Variants {
		if variant.ID == platformJSONVariantID {
			hasJSON = true
		}
		variants = append(variants, variant)
	}
	if platformProbeJSONVariantDisabled(probe) {
		return variants
	}
	if !hasJSON {
		variants = append(variants, contentdownload.Variant{
			ID:     platformJSONVariantID,
			Type:   platformJSONVariantID,
			Label:  "JSON",
			Suffix: ".json",
		})
	}
	return variants
}

func platformProbeJSONVariantDisabled(probe *contentdownload.Probe) bool {
	if probe == nil || probe.Internal == nil {
		return false
	}
	disabled, _ := probe.Internal[contentdownload.InternalKeyDisableJSONVariant].(bool)
	return disabled
}

func platformJSONResolvedRequest(sourceURL string, probe *contentdownload.Probe, opts contentdownload.Options) *contentdownload.ResolvedRequest {
	summary := contentdownload.ContentSummaryOf(probe.Content)
	contentID := firstNonEmpty(probe.ContentID, summary.ID)
	title := firstNonEmpty(summary.Title, contentID, "content")
	filename := firstNonEmpty(opts.Filename, title, contentID, "content")
	canonicalURL := firstNonEmpty(probe.CanonicalURL, summary.URL, sourceURL, probe.SourceURL)
	sourceURL = firstNonEmpty(probe.SourceURL, sourceURL, summary.SourceURL, canonicalURL)
	payload := gin.H{
		"id":            probe.ID,
		"platform":      probe.Platform,
		"source_url":    sourceURL,
		"canonical_url": canonicalURL,
		"content_id":    contentID,
		"content":       probe.Content,
		"variants":      platformProbeVariants(probe),
		"defaults":      contentdownload.Defaults{VariantID: platformJSONVariantID, Suffix: ".json"},
		"warnings":      probe.Warnings,
		"output":        platformProbeOutput(probe),
	}
	if probePipeline := platformProbePipeline(probe); probePipeline != nil {
		payload["probe_pipeline"] = probePipeline
	}
	return &contentdownload.ResolvedRequest{
		Platform:     probe.Platform,
		SourceURL:    sourceURL,
		CanonicalURL: canonicalURL,
		ContentID:    contentID,
		Title:        title,
		Filename:     filename,
		Suffix:       ".json",
		Download: contentdownload.DownloadSpec{
			URL:         "inline-json://" + probe.Platform + "/" + contentID,
			Method:      "GET",
			Protocol:    "inline_json",
			Connections: 1,
		},
		Labels: map[string]string{
			"platform":     probe.Platform,
			"id":           contentID,
			"title":        title,
			"key":          "0",
			"spec":         "",
			"suffix":       ".json",
			"source_url":   sourceURL,
			"content_type": summary.Type,
		},
		Metadata: map[string]any{
			"variant_id":    platformJSONVariantID,
			"content_type":  summary.Type,
			"source_url":    sourceURL,
			"canonical_url": canonicalURL,
			"json":          payload,
		},
		Content: probe.Content,
	}
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
			"id":          task.Id,
			"task_id":     task.TaskId,
			"status":      task.Status,
			"title":       task.Title,
			"filepath":    task.Filepath,
			"output_path": firstNonEmpty(task.OutputPath, task.Filepath),
			"platform":    firstNonEmpty(platform, probe.Platform),
			"content_id":  task.ExternalId,
			"updated_at":  task.UpdatedAt,
		})
	}
	return out
}

func newPlatformWorkflowRun(rawURL string, extra map[string]any) *platformWorkflowRun {
	return newPlatformWorkflowRunWithID(rawURL, extra, "")
}

func newPlatformWorkflowRunWithID(rawURL string, extra map[string]any, runID string) *platformWorkflowRun {
	now := time.Now()
	runID = normalizePlatformWorkflowRunID(runID)
	if runID == "" {
		runID = fmt.Sprintf("run_%d", now.UnixNano())
	}
	return &platformWorkflowRun{
		ID:          runID,
		URL:         rawURL,
		Status:      "running",
		CurrentNode: "start",
		Extra:       extra,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func normalizePlatformWorkflowRunID(runID string) string {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return ""
	}
	if len(runID) > 96 {
		runID = runID[:96]
	}
	var b strings.Builder
	for _, r := range runID {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-' || r == '.':
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (r *platformWorkflowRun) startNode(id, typ string) {
	r.mu.Lock()
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
			r.mu.Unlock()
			r.changed("node_started", id)
			return
		}
	}
	r.Nodes = append(r.Nodes, platformWorkflowNode{
		ID:        id,
		Type:      typ,
		Status:    "running",
		StartedAt: now,
	})
	r.mu.Unlock()
	r.changed("node_started", id)
}

func (r *platformWorkflowRun) completeNode(id string) {
	r.mu.Lock()
	now := time.Now()
	r.UpdatedAt = now
	for i := range r.Nodes {
		if r.Nodes[i].ID == id {
			r.Nodes[i].Status = "completed"
			r.Nodes[i].EndedAt = now
			r.mu.Unlock()
			r.changed("node_completed", id)
			return
		}
	}
	r.mu.Unlock()
}

func (r *platformWorkflowRun) failNode(id string, err error) {
	r.mu.Lock()
	now := time.Now()
	r.Status = "failed"
	r.CurrentNode = id
	r.UpdatedAt = now
	for i := range r.Nodes {
		if r.Nodes[i].ID == id {
			r.Nodes[i].Status = "failed"
			r.Nodes[i].Error = err.Error()
			r.Nodes[i].EndedAt = now
			r.mu.Unlock()
			r.changed("node_failed", id)
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
	r.mu.Unlock()
	r.changed("node_failed", id)
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
	r.Selection = clonePlatformWorkflowMap(output)
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
	r.changed("node_completed", "resume_after_probe")
}

func (r *platformWorkflowRun) waitForUserConfirmation(id string, interaction *platformWorkflowInteraction) {
	r.mu.Lock()
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
			r.mu.Unlock()
			r.changed("node_waiting", id)
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
	r.mu.Unlock()
	r.changed("node_waiting", id)
}

func (r *platformWorkflowRun) finish() {
	r.mu.Lock()
	r.Status = "completed"
	r.CurrentNode = "done"
	r.UpdatedAt = time.Now()
	r.mu.Unlock()
	r.changed("workflow_completed", "")
}

func (r *platformWorkflowRun) changed(event string, nodeID string) {
	if r == nil {
		return
	}
	if r.onChange != nil {
		r.onChange(r)
	}
	platformBroadcastWorkflowRun(r, event, nodeID)
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

func (r *platformWorkflowRun) nodeSnapshot(id string) gin.H {
	id = strings.TrimSpace(id)
	if r == nil || id == "" {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, node := range r.Nodes {
		if node.ID != id {
			continue
		}
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
		return view
	}
	return nil
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
