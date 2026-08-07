package wxmpadapter

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
	"wx_channel/pkg/flowengine"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/hermes"
)

const (
	wxmpPostprocessPipelineName         = "wxmp_postprocess"
	wxmpPostprocessRunKey               = "wxmp_postprocess_run"
	wxmpPostprocessContextEmbeddedNames = "embedded_resource_names"
	wxmpPostprocessContextOutputFile    = "output_file"
	wxmpPostprocessContextFinalHTML     = "final_html"
	wxmpPostprocessContextKeyCtx        = "context"
	wxmpPostprocessFlowNodeAssembleHTML = "assemble_html"
	wxmpPostprocessFlowNodeCleanup      = "cleanup_embedded_resources"
	wxmpPostprocessFlowNodeDone         = "done"
)

type wxmpPostprocessRun struct {
	task        *hermes.TaskJob
	db          *gorm.DB
	logger      zerolog.Logger
	savePath    string
	taskName    string
	bizType     int
	contentHTML string
}

// Postprocess assembles downloaded wxmp resources into the final HTML file.
func (a *OfficialAccountAdapter) Postprocess(ctx context.Context, info *hermes.TaskJob, deps adapter.PostprocessDeps) error {
	taskID := info.ID
	var meta struct {
		BizType int `json:"biz_type"`
	}
	if info.Metadata != nil {
		metadataJSON, err := json.Marshal(info.Metadata)
		if err != nil {
			return fmt.Errorf("wxmp postprocess: encode task metadata: %w", err)
		}
		if err := json.Unmarshal(metadataJSON, &meta); err != nil {
			return fmt.Errorf("wxmp postprocess: decode task metadata: %w", err)
		}
	}
	if meta.BizType != 1 {
		deps.Logger.Info().
			Int("task_id", taskID).
			Int("biz_type", meta.BizType).
			Msg("Postprocessor.wxmp: non-article task, skipping HTML image embedding")
		return nil
	}

	if deps.DB == nil {
		return fmt.Errorf("wxmp postprocess: database is nil")
	}

	var htmlResource *hermes.ResourceJob
	for i := range info.Resources {
		resource := &info.Resources[i]
		kind := strings.ToLower(strings.TrimSpace(resource.Kind))
		if (kind == "html" || kind == "text/html") && resource.FilePath != "" {
			htmlResource = resource
			break
		}
	}
	if htmlResource == nil {
		return fmt.Errorf("wxmp postprocess: article task %d has no downloaded HTML resource", taskID)
	}
	contentHTML, err := os.ReadFile(htmlResource.FilePath)
	if err != nil {
		return fmt.Errorf("wxmp postprocess: read HTML resource %s: %w", htmlResource.FilePath, err)
	}

	data := map[string]interface{}{
		wxmpPostprocessRunKey: &wxmpPostprocessRun{
			task:        info,
			db:          deps.DB,
			logger:      deps.Logger,
			savePath:    deps.BasePath,
			taskName:    info.Name,
			bizType:     meta.BizType,
			contentHTML: string(contentHTML),
		},
		wxmpPostprocessContextKeyCtx:     ctx,
		wxmpPostprocessContextOutputFile: htmlResource.FilePath,
	}

	if err := runWXMPPostprocessFlow(wxmpPostprocessFlow, data); err != nil {
		return err
	}
	return nil
}

func wxmpPostprocessRunFromContext(data map[string]interface{}) (*wxmpPostprocessRun, error) {
	run, _ := data[wxmpPostprocessRunKey].(*wxmpPostprocessRun)
	if run == nil || run.task == nil {
		return nil, fmt.Errorf("缺少 wxmp_postprocess_run")
	}
	if run.db == nil {
		return nil, fmt.Errorf("缺少 db")
	}
	return run, nil
}

// AssembleHTMLNode embeds downloaded images in an official-account article and
// wraps the fragment as a complete HTML document.
func assembleHTMLNode(values map[string]interface{}) (interface{}, error) {
	return runWXMPNode(values, wxmpPostprocessFlowNodeAssembleHTML, func(run *wxmpPostprocessRun) (interface{}, error) {
		return assembleHTMLNodeInternal(values, run)
	})
}

func assembleHTMLNodeInternal(values map[string]interface{}, run *wxmpPostprocessRun) (interface{}, error) {
	if run.savePath == "" {
		return nil, fmt.Errorf("缺少 save_path")
	}
	taskName := strings.TrimSpace(run.taskName)
	if taskName == "" {
		taskName = "article"
	}
	taskID := run.task.ID
	bizType := run.bizType

	logf := func(msg string, args ...interface{}) {
		run.logger.Info().Int("task_id", taskID).Msgf(msg, args...)
	}
	warnf := func(msg string, args ...interface{}) {
		run.logger.Warn().Int("task_id", taskID).Msgf(msg, args...)
	}

	logf("assemble_html: savePath=%q taskName=%q bizType=%d", run.savePath, taskName, bizType)

	if bizType != 1 {
		return nil, fmt.Errorf("wxmp postprocess: unsupported biz_type %d", bizType)
	}
	if run.contentHTML == "" {
		return nil, fmt.Errorf("缺少 content_html")
	}

	embeddedResources := make(map[string]bool)
	md5ToResource := imageResourcesByURL(run.task.Resources)
	logf("assemble_html: article mode, mapped %d downloaded image resources from task", len(md5ToResource))

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(run.contentHTML))
	if err != nil {
		return nil, fmt.Errorf("解析 content HTML 失败: %w", err)
	}

	totalImgs := 0
	matchedImgs := 0
	dataURIImgs := 0
	remoteFallbackImgs := 0
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		totalImgs++
		imgURL := s.AttrOr("data-src", "")
		srcAttr := "data-src"
		if imgURL == "" {
			imgURL = s.AttrOr("src", "")
			srcAttr = "src"
		}
		imgURL = normalizeImageURL(imgURL)
		if imgURL == "" {
			warnf("assemble_html: img[%d] no valid URL (both data-src and src are empty)", i)
			return
		}

		hash := md5.Sum([]byte(imgURL))
		hashStr := hex.EncodeToString(hash[:])
		logf("assemble_html: img[%d] URL=%q attr=%s → hash=%s", i, imgURL, srcAttr, hashStr)

		if resource, ok := md5ToResource[hashStr]; ok {
			matchedImgs++
			dataURI := imageResourceToDataURI(resource)

			if dataURI != "" {
				s.SetAttr("src", dataURI)
				dataURIImgs++
				embeddedResources[wxmpResourceKey(resource)] = true
				logf("assemble_html: img[%d] base64 inline success (resource_id=%d file=%s size=%d)",
					i, resource.ID, resource.FilePath, len(dataURI))
			} else {
				s.SetAttr("src", imgURL)
				remoteFallbackImgs++
				warnf("assemble_html: img[%d] base64 read failed, falling back to remote URL (resource_id=%d path=%s)",
					i, resource.ID, resource.FilePath)
			}
			s.RemoveAttr("data-src")
		} else {
			warnf("assemble_html: img[%d] no local resource matched (URL=%q hash=%q, md5ToResource keys=%v)",
				i, imgURL, hashStr, mapKeysResource(md5ToResource))
		}
	})

	logf("assemble_html: stats total_imgs=%d matched=%d data_uri=%d remote_fallback=%d unmatched=%d embedded_resources=%v",
		totalImgs, matchedImgs, dataURIImgs, remoteFallbackImgs, totalImgs-matchedImgs, mapKeysBool(embeddedResources))

	bodyHTML, err := doc.Find("body").Html()
	if err != nil {
		bodyHTML = run.contentHTML
	}
	values[wxmpPostprocessContextEmbeddedNames] = embeddedResources

	fullHTML := AssembleFullHTML(taskName, bodyHTML)

	// Ensure output file has .html extension.
	// Sanitize task name to replace "/" which can appear in titles (e.g. "zh-CN/zh-TW")
	// and would break filepath.Join by creating unintended subdirectories.
	var outputPath string
	if outFile, _ := values[wxmpPostprocessContextOutputFile].(string); outFile != "" {
		outputPath = outFile
	} else {
		safeName := strings.ReplaceAll(taskName, "/", "-")
		if !strings.HasSuffix(safeName, ".html") {
			safeName += ".html"
		}
		outputPath = filepath.Join(run.savePath, safeName)
	}

	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(fullHTML), 0644); err != nil {
		return nil, fmt.Errorf("写入 HTML 文件失败: %w", err)
	}
	for i := range run.task.Resources {
		resource := &run.task.Resources[i]
		if resource.FilePath != "" && filepath.Clean(resource.FilePath) == filepath.Clean(outputPath) {
			resource.Kind = "text/html"
			resource.Size = int64(len(fullHTML))
			break
		}
	}

	values[wxmpPostprocessContextFinalHTML] = outputPath
	return nil, nil
}

// GetWXMPPostprocessFlowVisualization returns a read-only flow graph payload for
// frontend visualization.
func GetWXMPPostprocessFlowVisualization(flowID string) (*WXMPPostprocessFlowVisualizationPayload, error) {
	payload, err := flowengine.BuildFlowVisualizationPayload([]flowengine.FlowDefinition{wxmpPostprocessFlow}, flowID, flowengine.FlowVisualizationOptions{
		Platform: "wxmp",
		Purpose:  "postprocess-flow-visualization",
		Editable: false,
	})
	if err != nil {
		return nil, err
	}

	return payload, nil
}

type WXMPPostprocessFlowVisualizationPayload = flowengine.FlowVisualizationPayload

var AssembleHTMLNode = flowengine.NodeDefinition{
	ID:   wxmpPostprocessFlowNodeAssembleHTML,
	Name: "assemble_html",
	Type: "FuncNode",
	InputSchema: []flowengine.FieldSchema{
		{Key: wxmpPostprocessRunKey, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
			{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
				{Key: "ID", Type: "int", Required: true},
				{Key: "Name", Type: "string", Required: false},
				{Key: "UniqueID", Type: "string", Required: false},
				{Key: "SavePath", Type: "string", Required: false},
				{Key: "FilenameTemplate", Type: "string", Required: false},
				{Key: "Platform", Type: "string", Required: false},
				{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
					{Key: "type", Type: "any", Required: false},
					{Key: "suffix", Type: "string", Required: false},
				}},
			}},
			{Key: "db", Type: "any", Required: false},
			{Key: "logger", Type: "any", Required: false},
			{Key: "savePath", Type: "string", Required: false},
			{Key: "taskName", Type: "string", Required: false},
			{Key: "bizType", Type: "int", Required: false},
			{Key: "contentHTML", Type: "string", Required: false},
		}},
		{Key: wxmpPostprocessContextOutputFile, Type: "string", Required: false},
	},
	OutputSchema: []flowengine.FieldSchema{
		{Key: wxmpPostprocessContextFinalHTML, Type: "string", Required: false},
		{Key: wxmpPostprocessContextEmbeddedNames, Type: "object", Required: false},
	},
	Config: map[string]interface{}{
		"func": assembleHTMLNode,
	},
	NextNodes: []flowengine.TargetNode{{TargetID: wxmpPostprocessFlowNodeCleanup}},
}

func cleanupEmbeddedImageResourcesNode(values map[string]interface{}) (interface{}, error) {
	return runWXMPNode(values, wxmpPostprocessFlowNodeCleanup, func(run *wxmpPostprocessRun) (interface{}, error) {
		warnf := func(msg string, args ...interface{}) {
			run.logger.Warn().Int("task_id", run.task.ID).Msgf(msg, args...)
		}

		embeddedResources, _ := values[wxmpPostprocessContextEmbeddedNames].(map[string]bool)
		if len(embeddedResources) == 0 {
			return nil, nil
		}

		keptResources := make([]hermes.ResourceJob, 0, len(run.task.Resources))
		for _, r := range run.task.Resources {
			if embeddedResources[wxmpResourceKey(&r)] {
				if err := os.Remove(r.FilePath); err != nil && !os.IsNotExist(err) {
					warnf("Postprocessor.wxmp: task_id=%d remove image %s: %v", run.task.ID, r.FilePath, err)
				}
				if err := run.db.Where("id = ? AND task_id = ?", r.ID, run.task.ID).Delete(&model.DownloadResource{}).Error; err != nil {
					warnf("Postprocessor.wxmp: task_id=%d remove image record id=%d: %v", run.task.ID, r.ID, err)
				}
				continue
			}
			keptResources = append(keptResources, r)
		}

		run.task.Resources = keptResources
		return nil, nil
	})
}

var CleanupEmbeddedImageResourcesNode = flowengine.NodeDefinition{
	ID:   wxmpPostprocessFlowNodeCleanup,
	Name: "cleanup_embedded_resources",
	Type: "FuncNode",
	InputSchema: []flowengine.FieldSchema{
		{Key: wxmpPostprocessRunKey, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
			{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
				{Key: "ID", Type: "int", Required: true},
				{Key: "Name", Type: "string", Required: false},
				{Key: "UniqueID", Type: "string", Required: false},
				{Key: "SavePath", Type: "string", Required: false},
				{Key: "FilenameTemplate", Type: "string", Required: false},
				{Key: "Platform", Type: "string", Required: false},
				{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
					{Key: "type", Type: "any", Required: false},
					{Key: "suffix", Type: "string", Required: false},
				}},
			}},
			{Key: "db", Type: "any", Required: false},
			{Key: "logger", Type: "any", Required: false},
			{Key: "savePath", Type: "string", Required: false},
			{Key: "taskName", Type: "string", Required: false},
			{Key: "bizType", Type: "int", Required: false},
			{Key: "contentHTML", Type: "string", Required: false},
		}},
		{Key: wxmpPostprocessContextEmbeddedNames, Type: "object", Required: false},
	},
	// No explicit output field; this node updates run.task.Resources in place.
	OutputSchema: []flowengine.FieldSchema{},
	Config: map[string]interface{}{
		"func": cleanupEmbeddedImageResourcesNode,
	},
	NextNodes: []flowengine.TargetNode{{TargetID: wxmpPostprocessFlowNodeDone}},
}

var wxmpPostprocessFlow = flowengine.FlowDefinition{
	ID:   wxmpPostprocessPipelineName,
	Name: wxmpPostprocessPipelineName,
	ContextSchema: []flowengine.FieldSchema{
		{Key: wxmpPostprocessRunKey, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
			{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
				{Key: "ID", Type: "int", Required: true},
				{Key: "Name", Type: "string", Required: false},
				{Key: "UniqueID", Type: "string", Required: false},
				{Key: "SavePath", Type: "string", Required: false},
				{Key: "FilenameTemplate", Type: "string", Required: false},
				{Key: "Platform", Type: "string", Required: false},
				{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
					{Key: "type", Type: "any", Required: false},
					{Key: "suffix", Type: "string", Required: false},
				}},
			}},
			{Key: "db", Type: "any", Required: false},
			{Key: "logger", Type: "any", Required: false},
			{Key: "savePath", Type: "string", Required: false},
			{Key: "taskName", Type: "string", Required: false},
			{Key: "bizType", Type: "int", Required: false},
			{Key: "contentHTML", Type: "string", Required: false},
		}},
		{Key: wxmpPostprocessContextKeyCtx, Type: "object", Required: false},
		{Key: wxmpPostprocessContextOutputFile, Type: "string", Required: false},
	},
	StartNodeID: wxmpPostprocessFlowNodeAssembleHTML,
	Nodes: map[string]flowengine.NodeDefinition{
		wxmpPostprocessFlowNodeAssembleHTML: AssembleHTMLNode,
		wxmpPostprocessFlowNodeCleanup:      CleanupEmbeddedImageResourcesNode,
		wxmpPostprocessFlowNodeDone: {
			ID:          wxmpPostprocessFlowNodeDone,
			Name:        "done",
			Type:        "EndNode",
			InputSchema: []flowengine.FieldSchema{},
			OutputSchema: []flowengine.FieldSchema{
				{Key: wxmpPostprocessContextFinalHTML, Type: "string", Required: false},
			},
		},
	},
}

func runWXMPNode(values map[string]interface{}, nodeID string, fn func(*wxmpPostprocessRun) (interface{}, error)) (interface{}, error) {
	run, err := wxmpPostprocessRunFromContext(values)
	if err != nil {
		return nil, err
	}
	taskID := 0
	if run.task != nil {
		taskID = run.task.ID
	}
	input := formatWXMPContextSnapshot(values)
	run.logger.Info().Int("task_id", taskID).Msgf("Postprocess.wxmp: node=%s status=started input=%s", nodeID, input)

	output, err := fn(run)
	if err != nil {
		run.logger.Info().Int("task_id", taskID).Msgf("Postprocess.wxmp: node=%s status=failed input=%s output=%s error=%v",
			nodeID, input, formatWXMPContextSnapshot(values), err)
		return nil, err
	}

	run.logger.Info().Int("task_id", taskID).Msgf("Postprocess.wxmp: node=%s status=succeeded input=%s output=%s",
		nodeID, input, formatWXMPContextSnapshot(values))
	return output, nil
}

func logWXMPFlow(flowID string, status string, run *wxmpPostprocessRun, err error) {
	if run == nil {
		return
	}
	taskID := 0
	if run.task != nil {
		taskID = run.task.ID
	}
	if err != nil {
		run.logger.Info().Int("task_id", taskID).Msgf("Postprocess.wxmp: flow=%s status=%s error=%v", flowID, status, err)
		return
	}
	run.logger.Info().Int("task_id", taskID).Msgf("Postprocess.wxmp: flow=%s status=%s", flowID, status)
}

func logWXMPFlowNodeExecution(run *wxmpPostprocessRun, flowEngine *flowengine.FlowEngine, flowID, instanceID string, err error) {
	if run == nil || flowEngine == nil || instanceID == "" {
		return
	}

	record, hasRecord := flowEngine.GetRunSnapshot(instanceID)
	if !hasRecord {
		if err != nil {
			run.logger.Info().Msgf("Postprocess.wxmp: flow=%s node_execution_summary unavailable: %v", flowID, err)
		} else {
			run.logger.Info().Msgf("Postprocess.wxmp: flow=%s node_execution_summary unavailable", flowID)
		}
		return
	}

	_, nodeStates := flowEngine.GetRunContext(instanceID)
	nodeIDs := make([]string, 0, len(record.NodeAttempts))
	for nodeID := range record.NodeAttempts {
		nodeIDs = append(nodeIDs, nodeID)
	}
	for nodeID := range nodeStates {
		exists := false
		for _, existingID := range nodeIDs {
			if existingID == nodeID {
				exists = true
				break
			}
		}
		if !exists {
			nodeIDs = append(nodeIDs, nodeID)
		}
	}
	if len(nodeIDs) == 0 {
		run.logger.Info().Msgf("Postprocess.wxmp: flow=%s node_execution_summary: no nodes were executed", flowID)
		return
	}

	sort.Strings(nodeIDs)
	for _, nodeID := range nodeIDs {
		state := "unknown"
		attempts := record.NodeAttempts[nodeID]
		if attempts == 0 {
			state = "skipped"
		}
		if len(nodeStates) > 0 {
			if nodeState, ok := nodeStates[nodeID]; ok {
				switch nodeState {
				case flowengine.StateCompleted:
					state = "succeeded"
				case flowengine.StateFailed:
					state = "failed"
				case flowengine.StateRunning, flowengine.StateRetrying:
					state = "running"
				case flowengine.StatePending:
					state = "pending"
				default:
					state = string(nodeState)
				}
			}
		}
		if state == "unknown" && attempts > 0 && record.Status == flowengine.RunStatusFailed && record.CurrentNode == nodeID {
			state = "failed"
		}
		if state == "unknown" && attempts > 0 && record.Status == flowengine.RunStatusCompleted {
			state = "succeeded"
		}

		taskSuffix := ""
		if attempts > 0 {
			taskSuffix = fmt.Sprintf(" attempts=%d", attempts)
		}
		run.logger.Info().Msgf("Postprocess.wxmp: flow=%s node=%s status=%s%s", flowID, nodeID, state, taskSuffix)
	}
}

func formatWXMPContextSnapshot(values map[string]interface{}) string {
	if values == nil {
		return "{}"
	}

	snapshot := make(map[string]interface{}, len(values))
	for key, value := range values {
		if key == wxmpPostprocessRunKey || key == wxmpPostprocessContextKeyCtx {
			continue
		}
		snapshot[key] = value
	}

	if run, ok := values[wxmpPostprocessRunKey].(*wxmpPostprocessRun); ok && run != nil {
		taskID := 0
		resourceCount := 0
		if run.task != nil {
			taskID = run.task.ID
			resourceCount = len(run.task.Resources)
		}
		snapshot["task_id"] = taskID
		snapshot["task_name"] = run.taskName
		snapshot["save_path"] = run.savePath
		snapshot["resource_count"] = resourceCount
	}

	keys := make([]string, 0, len(snapshot))
	for key := range snapshot {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("{")
	for i, key := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(key)
		b.WriteString("=")
		b.WriteString(formatWXMPContextValue(snapshot[key]))
	}
	b.WriteString("}")
	return b.String()
}

func formatWXMPContextValue(value interface{}) string {
	if value == nil {
		return "<nil>"
	}
	switch typed := value.(type) {
	case string:
		return "\"" + typed + "\""
	case int:
		return fmt.Sprintf("%d", typed)
	case int64:
		return fmt.Sprintf("%d", typed)
	case float64:
		return fmt.Sprintf("%v", typed)
	case bool:
		return fmt.Sprintf("%t", typed)
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func runWXMPPostprocessFlow(flow flowengine.FlowDefinition, data map[string]interface{}) error {
	if data == nil {
		data = map[string]interface{}{}
	}
	if _, ok := data[wxmpPostprocessContextKeyCtx]; !ok {
		data[wxmpPostprocessContextKeyCtx] = context.Background()
	}
	run, _ := data[wxmpPostprocessRunKey].(*wxmpPostprocessRun)
	logWXMPFlow(flow.ID, "started", run, nil)
	if run != nil {
		run.logger.Info().Msgf("Postprocess.wxmp: flow=%s input=%s", flow.ID, formatWXMPContextSnapshot(data))
	}

	flowEngine := flowengine.NewWorkflowEngine()
	flowEngine.SetFlowDefinitions(map[string]flowengine.FlowDefinition{
		flow.ID: flow,
	})
	instanceID, err := flowEngine.StartFlow(flow.ID, data)
	if err != nil {
		logWXMPFlow(flow.ID, "failed", run, err)
		if run != nil {
			logWXMPFlowNodeExecution(run, flowEngine, flow.ID, instanceID, err)
		}
		return err
	}
	logWXMPFlow(flow.ID, "succeeded", run, nil)
	if run != nil {
		logWXMPFlowNodeExecution(run, flowEngine, flow.ID, instanceID, nil)
	}
	return err
}

// AssembleFullHTML wraps body HTML into a complete standalone HTML document.
func AssembleFullHTML(title, bodyHTML string) string {
	return assembleWXMPFullHTML(title, bodyHTML, false)
}

// composeWXMPAlbumHTML wraps album body HTML into a complete HTML document with album layout.
func assembleAlbumFullHTML(title, bodyHTML string) string {
	return assembleWXMPFullHTML(title, bodyHTML, true)
}

func assembleWXMPFullHTML(title, bodyHTML string, isAlbum bool) string {
	var b strings.Builder
	var cssBuilder strings.Builder

	baseMaxWidth := 677
	if isAlbum {
		baseMaxWidth = 1024
	}
	cssBuilder.WriteString(`        html { height: 100%; }`)
	cssBuilder.WriteString(fmt.Sprintf(`
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.6;
            max-width: %dpx;
            margin: 0 auto;
            padding: 20px;
            color: #333;
        }
        h1 { font-size: 1.8em; margin-bottom: 0.5em; }
        img { max-width: 100%%; height: auto; }`, baseMaxWidth))
	if isAlbum {
		cssBuilder.WriteString(`
        img { display: block; margin-bottom: 20px; border-radius: 6px; }
        .album_desc { font-size: 1.1em; margin-bottom: 30px; color: #666; text-align: center; }
        .album_gallery { max-width: 677px; margin: 0 auto; }`)
	}

	b.WriteString(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>`)
	b.WriteString(escapeHTMLStr(title))
	b.WriteString(`</title>
    <style>
`)
	b.WriteString(cssBuilder.String())
	b.WriteString(`
    </style>
</head>
<body>`)
	b.WriteString(bodyHTML)
	b.WriteString(`</body>
</html>`)
	return b.String()
}

func imageResourcesByURL(resources []hermes.ResourceJob) map[string]*hermes.ResourceJob {
	result := make(map[string]*hermes.ResourceJob)
	for i := range resources {
		resource := &resources[i]
		if !isEmbeddableWXMPImage(resource) {
			continue
		}
		for _, endpoint := range resource.Endpoints {
			imageURL := normalizeImageURL(endpoint.URL)
			if imageURL == "" {
				continue
			}
			hash := md5.Sum([]byte(imageURL))
			result[hex.EncodeToString(hash[:])] = resource
		}

		// Keep compatibility with tasks created before endpoint-based matching:
		// their resource name is the MD5 of the source URL.
		filename := filepath.Base(resource.Name)
		ext := filepath.Ext(filename)
		filename = strings.TrimSuffix(filename, ext)
		filename = strings.TrimSuffix(filename, "_")
		if len(filename) == md5.Size*2 {
			result[filename] = resource
		}
	}
	return result
}

func isEmbeddableWXMPImage(resource *hermes.ResourceJob) bool {
	if resource == nil || strings.HasSuffix(resource.UniqueID, "_cover") {
		return false
	}
	kind := strings.ToLower(strings.TrimSpace(resource.Kind))
	return kind == "image" || strings.HasPrefix(kind, "image/")
}

// imageResourceToDataURI reads a downloaded image from its actual Hermes path.
// The temporary path intentionally has no extension, so MIME comes from Kind.
func imageResourceToDataURI(resource *hermes.ResourceJob) string {
	if resource == nil || strings.TrimSpace(resource.FilePath) == "" {
		return ""
	}
	data, err := os.ReadFile(resource.FilePath)
	if err != nil {
		return ""
	}
	mime := strings.ToLower(strings.TrimSpace(resource.Kind))
	if !strings.HasPrefix(mime, "image/") {
		mime = mimeByExtension(filepath.Ext(resource.FilePath))
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func wxmpResourceKey(resource *hermes.ResourceJob) string {
	if resource == nil {
		return ""
	}
	if resource.ID > 0 {
		return fmt.Sprintf("id:%d", resource.ID)
	}
	if resource.UniqueID != "" {
		return "unique_id:" + resource.UniqueID
	}
	if resource.FilePath != "" {
		return "file_path:" + resource.FilePath
	}
	return "name:" + resource.Name
}

// mimeByExtension maps file extensions to MIME types.
func mimeByExtension(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".bmp":
		return "image/bmp"
	default:
		return "image/jpeg"
	}
}

func mapKeysResource(m map[string]*hermes.ResourceJob) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// mapKeysBool returns the keys of a map[string]bool as a slice for debug logging.
func mapKeysBool(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func escapeHTMLStr(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
