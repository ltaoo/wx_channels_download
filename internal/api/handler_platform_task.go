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
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	result "wx_channel/internal/util"
	contentshuba69 "wx_channel/pkg/contentplatform/69shuba"
	contentdouyin "wx_channel/pkg/contentplatform/douyin"
	contentdownload "wx_channel/pkg/contentplatform/download"
	contentfanqie "wx_channel/pkg/contentplatform/fanqienovel"
	contentoa "wx_channel/pkg/contentplatform/officialaccount"
	contentqidian "wx_channel/pkg/contentplatform/qidian"
	contentquanben "wx_channel/pkg/contentplatform/quanben"
	contentttk "wx_channel/pkg/contentplatform/ttk"
	contentv2ex "wx_channel/pkg/contentplatform/v2ex"
	contentwxchannels "wx_channel/pkg/contentplatform/wxchannels"
	contentxiaohongshu "wx_channel/pkg/contentplatform/xiaohongshu"
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

const platformJSONVariantID = "json"

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
	platformProbeAddJSONDefault(probe)
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
		if opts.VariantID == "" && probe != nil {
			opts.VariantID = probe.Defaults.VariantID
		}
		run.resumeAfterProbe(platformConfirmationOutput(body, opts))
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
		run.Resolved = resolved
		run.completeNode("resolve")
	}
	if resolved.Suffix == ".mp3" && !system.ExistingCommand("ffmpeg") {
		return "", fmt.Errorf("下载 mp3 需要支持 ffmpeg 命令")
	}
	platformEnsureMetadataPipelineNodes(resolved)

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

	go c.runPlatformDownloadTask(context.Background(), downloader, task.ID, rec.Id, run)
	return task.ID, nil
}

type platformPlanExecution struct {
	Resolved       *contentdownload.ResolvedRequest
	DownloadTaskID int
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

func (c *APIClient) upsertPlatformResolvedContent(resolved *contentdownload.ResolvedRequest, downloadTaskID int, account *model.Account) (*model.Content, error) {
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
	contentType := strings.TrimSpace(firstNonEmpty(summary.Type, resolved.Labels["content_type"], platformContentMetadataString(resolved.Metadata, "content_type")))
	if contentType == "" {
		contentType = platformContentTypeFromSuffix(resolved.Suffix)
	}
	metaBytes, _ := json.Marshal(map[string]any{
		"resolved": resolved.Metadata,
		"content":  metadata,
		"labels":   resolved.Labels,
	})
	taskID := downloadTaskID
	now := util.NowMillis()
	content := model.Content{
		PlatformId:     platformID,
		ContentType:    contentType,
		ExternalId:     contentID,
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
	switch strings.ToLower(strings.TrimSpace(suffix)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return "image"
	case ".mp3", ".m4a", ".aac", ".wav", ".flac":
		return "audio"
	case ".html", ".htm", ".json", ".txt", ".md":
		return "article"
	default:
		return "video"
	}
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
		contentwxchannels.New(platformChannelsFetcher{
			FeedProfileFetcher: c.channels,
			sphCookie:          c.cfg.CloudflareSphCookie,
		}),
		contentdouyin.New(nil),
		contentxiaohongshu.New(nil),
		contentzhihu.New(nil),
		contentoa.New(nil),
		contentyoutube.New(nil),
		c.shuba69Handler(),
		contentqidian.New(nil),
		contentquanben.New(nil),
		contentttk.New(nil),
		contentv2ex.New(nil),
		contentfanqie.New(nil),
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
	contentOutput := contentdownload.ContentOutputOf(probe.Content)
	out := make(map[string]any, len(contentOutput))
	for k, v := range contentOutput {
		if platformProbeOutputOmitField(k) {
			continue
		}
		out[k] = v
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
	probe.Defaults.VariantID = platformJSONVariantID
	probe.Defaults.Suffix = ".json"
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
