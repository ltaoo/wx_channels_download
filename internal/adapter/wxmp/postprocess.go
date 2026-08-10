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
	wxmp_postprocess_pipeline_name           = "wxmp_postprocess"
	wxmp_postprocess_run_key                 = "wxmp_postprocess_run"
	wxmp_postprocess_context_embedded_names  = "embedded_resource_names"
	wxmp_postprocess_context_output_file     = "output_file"
	wxmp_postprocess_context_final_html      = "final_html"
	wxmp_postprocess_context_key_ctx         = "context"
	wxmp_postprocess_flow_node_assemble_html = "assemble_html"
	wxmp_postprocess_flow_node_cleanup       = "cleanup_embedded_resources"
	wxmp_postprocess_flow_node_done          = "done"
)

type wxmp_postprocess_run struct {
	task         *hermes.TaskJob
	db           *gorm.DB
	logger       zerolog.Logger
	download_dir string
	task_name    string
	biz_type     int
	content_html string
}

// Postprocess assembles downloaded wxmp resources into the final HTML file.
func (a *OfficialAccountAdapter) Postprocess(ctx context.Context, info *hermes.TaskJob, deps adapter.PostprocessDeps) error {
	task_id := info.ID
	var meta struct {
		BizType int `json:"biz_type"`
	}
	if info.Metadata != nil {
		metadata_json, err := json.Marshal(info.Metadata)
		if err != nil {
			return fmt.Errorf("wxmp postprocess: encode task metadata: %w", err)
		}
		if err := json.Unmarshal(metadata_json, &meta); err != nil {
			return fmt.Errorf("wxmp postprocess: decode task metadata: %w", err)
		}
	}
	if meta.BizType != 1 {
		deps.Logger.Info().
			Int("task_id", task_id).
			Int("biz_type", meta.BizType).
			Msg("Postprocessor.wxmp: non-article task, skipping HTML image embedding")
		return nil
	}

	if deps.DB == nil {
		return fmt.Errorf("wxmp postprocess: database is nil")
	}

	var html_resource *hermes.ResourceJob
	for i := range info.Resources {
		resource := &info.Resources[i]
		kind := strings.ToLower(strings.TrimSpace(resource.Kind))
		if (kind == "html" || kind == "text/html") && resource.FilePath != "" {
			html_resource = resource
			break
		}
	}
	if html_resource == nil {
		return fmt.Errorf("wxmp postprocess: article task %d has no downloaded HTML resource", task_id)
	}
	content_html, err := os.ReadFile(html_resource.FilePath)
	if err != nil {
		return fmt.Errorf("wxmp postprocess: read HTML resource %s: %w", html_resource.FilePath, err)
	}

	data := map[string]interface{}{
		wxmp_postprocess_run_key: &wxmp_postprocess_run{
			task:         info,
			db:           deps.DB,
			logger:       deps.Logger,
			download_dir: deps.BasePath,
			task_name:    info.Name,
			biz_type:     meta.BizType,
			content_html: string(content_html),
		},
		wxmp_postprocess_context_key_ctx:     ctx,
		wxmp_postprocess_context_output_file: html_resource.FilePath,
	}

	if err := run_wxmp_postprocess_flow(wxmp_postprocess_flow, data); err != nil {
		return err
	}
	return nil
}

func wxmp_postprocess_run_from_context(data map[string]interface{}) (*wxmp_postprocess_run, error) {
	run, _ := data[wxmp_postprocess_run_key].(*wxmp_postprocess_run)
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
func assemble_html_node(values map[string]interface{}) (interface{}, error) {
	return run_wxmp_node(values, wxmp_postprocess_flow_node_assemble_html, func(run *wxmp_postprocess_run) (interface{}, error) {
		return assemble_html_node_internal(values, run)
	})
}

func assemble_html_node_internal(values map[string]interface{}, run *wxmp_postprocess_run) (interface{}, error) {
	if run.download_dir == "" {
		return nil, fmt.Errorf("缺少 download_dir")
	}
	task_name := strings.TrimSpace(run.task_name)
	if task_name == "" {
		task_name = "article"
	}
	task_id := run.task.ID
	biz_type := run.biz_type

	logf := func(msg string, args ...interface{}) {
		run.logger.Info().Int("task_id", task_id).Msgf(msg, args...)
	}
	warnf := func(msg string, args ...interface{}) {
		run.logger.Warn().Int("task_id", task_id).Msgf(msg, args...)
	}

	logf("assemble_html: download_dir=%q task_name=%q biz_type=%d", run.download_dir, task_name, biz_type)

	if biz_type != 1 {
		return nil, fmt.Errorf("wxmp postprocess: unsupported biz_type %d", biz_type)
	}
	if run.content_html == "" {
		return nil, fmt.Errorf("缺少 content_html")
	}

	embedded_resources := make(map[string]bool)
	md5_to_resource := image_resources_by_url(run.task.Resources)
	logf("assemble_html: article mode, mapped %d downloaded image resources from task", len(md5_to_resource))

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(run.content_html))
	if err != nil {
		return nil, fmt.Errorf("解析 content HTML 失败: %w", err)
	}

	total_imgs := 0
	matched_imgs := 0
	data_uri_imgs := 0
	remote_fallback_imgs := 0
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		total_imgs++
		img_url := s.AttrOr("data-src", "")
		src_attr := "data-src"
		if img_url == "" {
			img_url = s.AttrOr("src", "")
			src_attr = "src"
		}
		img_url = normalize_image_url(img_url)
		if img_url == "" {
			warnf("assemble_html: img[%d] no valid URL (both data-src and src are empty)", i)
			return
		}

		hash := md5.Sum([]byte(img_url))
		hash_str := hex.EncodeToString(hash[:])
		logf("assemble_html: img[%d] URL=%q attr=%s → hash=%s", i, img_url, src_attr, hash_str)

		if resource, ok := md5_to_resource[hash_str]; ok {
			matched_imgs++
			data_uri := image_resource_to_data_uri(resource)

			if data_uri != "" {
				s.SetAttr("src", data_uri)
				data_uri_imgs++
				embedded_resources[wxmp_resource_key(resource)] = true
				logf("assemble_html: img[%d] base64 inline success (resource_id=%d file=%s size=%d)",
					i, resource.ID, resource.FilePath, len(data_uri))
			} else {
				s.SetAttr("src", img_url)
				remote_fallback_imgs++
				warnf("assemble_html: img[%d] base64 read failed, falling back to remote URL (resource_id=%d path=%s)",
					i, resource.ID, resource.FilePath)
			}
			s.RemoveAttr("data-src")
		} else {
			warnf("assemble_html: img[%d] no local resource matched (URL=%q hash=%q, md5_to_resource keys=%v)",
				i, img_url, hash_str, map_keys_resource(md5_to_resource))
		}
	})

	logf("assemble_html: stats total_imgs=%d matched=%d data_uri=%d remote_fallback=%d unmatched=%d embedded_resources=%v",
		total_imgs, matched_imgs, data_uri_imgs, remote_fallback_imgs, total_imgs-matched_imgs, map_keys_bool(embedded_resources))

	body_html, err := doc.Find("body").Html()
	if err != nil {
		body_html = run.content_html
	}
	values[wxmp_postprocess_context_embedded_names] = embedded_resources

	full_html := AssembleFullHTML(task_name, body_html)

	// Ensure output file has .html extension.
	// Sanitize task name to replace "/" which can appear in titles (e.g. "zh-CN/zh-TW")
	// and would break filepath.Join by creating unintended subdirectories.
	var output_path string
	if out_file, _ := values[wxmp_postprocess_context_output_file].(string); out_file != "" {
		output_path = out_file
	} else {
		safe_name := strings.ReplaceAll(task_name, "/", "-")
		if !strings.HasSuffix(safe_name, ".html") {
			safe_name += ".html"
		}
		output_path = filepath.Join(run.download_dir, safe_name)
	}

	dir := filepath.Dir(output_path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}
	if err := os.WriteFile(output_path, []byte(full_html), 0644); err != nil {
		return nil, fmt.Errorf("写入 HTML 文件失败: %w", err)
	}
	for i := range run.task.Resources {
		resource := &run.task.Resources[i]
		if resource.FilePath != "" && filepath.Clean(resource.FilePath) == filepath.Clean(output_path) {
			resource.Kind = "text/html"
			resource.Size = int64(len(full_html))
			break
		}
	}

	values[wxmp_postprocess_context_final_html] = output_path
	return nil, nil
}

// GetWXMPPostprocessFlowVisualization returns a read-only flow graph payload for
// frontend visualization.
func GetWXMPPostprocessFlowVisualization(flow_id string) (*WXMPPostprocessFlowVisualizationPayload, error) {
	payload, err := flowengine.BuildFlowVisualizationPayload([]flowengine.FlowDefinition{wxmp_postprocess_flow}, flow_id, flowengine.FlowVisualizationOptions{
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
	ID:   wxmp_postprocess_flow_node_assemble_html,
	Name: "assemble_html",
	Type: "FuncNode",
	InputSchema: []flowengine.FieldSchema{
		{Key: wxmp_postprocess_run_key, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
			{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
				{Key: "ID", Type: "int", Required: true},
				{Key: "Name", Type: "string", Required: false},
				{Key: "UniqueID", Type: "string", Required: false},
				{Key: "DownloadDir", Type: "string", Required: false},
				{Key: "FilenameTemplate", Type: "string", Required: false},
				{Key: "Platform", Type: "string", Required: false},
				{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
					{Key: "type", Type: "any", Required: false},
					{Key: "suffix", Type: "string", Required: false},
				}},
			}},
			{Key: "db", Type: "any", Required: false},
			{Key: "logger", Type: "any", Required: false},
			{Key: "download_dir", Type: "string", Required: false},
			{Key: "task_name", Type: "string", Required: false},
			{Key: "biz_type", Type: "int", Required: false},
			{Key: "content_html", Type: "string", Required: false},
		}},
		{Key: wxmp_postprocess_context_output_file, Type: "string", Required: false},
	},
	OutputSchema: []flowengine.FieldSchema{
		{Key: wxmp_postprocess_context_final_html, Type: "string", Required: false},
		{Key: wxmp_postprocess_context_embedded_names, Type: "object", Required: false},
	},
	Config: map[string]interface{}{
		"func": assemble_html_node,
	},
	NextNodes: []flowengine.TargetNode{{TargetID: wxmp_postprocess_flow_node_cleanup}},
}

func cleanup_embedded_image_resources_node(values map[string]interface{}) (interface{}, error) {
	return run_wxmp_node(values, wxmp_postprocess_flow_node_cleanup, func(run *wxmp_postprocess_run) (interface{}, error) {
		warnf := func(msg string, args ...interface{}) {
			run.logger.Warn().Int("task_id", run.task.ID).Msgf(msg, args...)
		}

		embedded_resources, _ := values[wxmp_postprocess_context_embedded_names].(map[string]bool)
		if len(embedded_resources) == 0 {
			return nil, nil
		}

		kept_resources := make([]hermes.ResourceJob, 0, len(run.task.Resources))
		for _, r := range run.task.Resources {
			if embedded_resources[wxmp_resource_key(&r)] {
				if err := os.Remove(r.FilePath); err != nil && !os.IsNotExist(err) {
					warnf("Postprocessor.wxmp: task_id=%d remove image %s: %v", run.task.ID, r.FilePath, err)
				}
				if err := run.db.Where("id = ? AND task_id = ?", r.ID, run.task.ID).Delete(&model.DownloadResource{}).Error; err != nil {
					warnf("Postprocessor.wxmp: task_id=%d remove image record id=%d: %v", run.task.ID, r.ID, err)
				}
				continue
			}
			kept_resources = append(kept_resources, r)
		}

		run.task.Resources = kept_resources
		return nil, nil
	})
}

var CleanupEmbeddedImageResourcesNode = flowengine.NodeDefinition{
	ID:   wxmp_postprocess_flow_node_cleanup,
	Name: "cleanup_embedded_resources",
	Type: "FuncNode",
	InputSchema: []flowengine.FieldSchema{
		{Key: wxmp_postprocess_run_key, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
			{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
				{Key: "ID", Type: "int", Required: true},
				{Key: "Name", Type: "string", Required: false},
				{Key: "UniqueID", Type: "string", Required: false},
				{Key: "DownloadDir", Type: "string", Required: false},
				{Key: "FilenameTemplate", Type: "string", Required: false},
				{Key: "Platform", Type: "string", Required: false},
				{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
					{Key: "type", Type: "any", Required: false},
					{Key: "suffix", Type: "string", Required: false},
				}},
			}},
			{Key: "db", Type: "any", Required: false},
			{Key: "logger", Type: "any", Required: false},
			{Key: "download_dir", Type: "string", Required: false},
			{Key: "task_name", Type: "string", Required: false},
			{Key: "biz_type", Type: "int", Required: false},
			{Key: "content_html", Type: "string", Required: false},
		}},
		{Key: wxmp_postprocess_context_embedded_names, Type: "object", Required: false},
	},
	// No explicit output field; this node updates run.task.Resources in place.
	OutputSchema: []flowengine.FieldSchema{},
	Config: map[string]interface{}{
		"func": cleanup_embedded_image_resources_node,
	},
	NextNodes: []flowengine.TargetNode{{TargetID: wxmp_postprocess_flow_node_done}},
}

var wxmp_postprocess_flow = flowengine.FlowDefinition{
	ID:   wxmp_postprocess_pipeline_name,
	Name: wxmp_postprocess_pipeline_name,
	ContextSchema: []flowengine.FieldSchema{
		{Key: wxmp_postprocess_run_key, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
			{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
				{Key: "ID", Type: "int", Required: true},
				{Key: "Name", Type: "string", Required: false},
				{Key: "UniqueID", Type: "string", Required: false},
				{Key: "DownloadDir", Type: "string", Required: false},
				{Key: "FilenameTemplate", Type: "string", Required: false},
				{Key: "Platform", Type: "string", Required: false},
				{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
					{Key: "type", Type: "any", Required: false},
					{Key: "suffix", Type: "string", Required: false},
				}},
			}},
			{Key: "db", Type: "any", Required: false},
			{Key: "logger", Type: "any", Required: false},
			{Key: "download_dir", Type: "string", Required: false},
			{Key: "task_name", Type: "string", Required: false},
			{Key: "biz_type", Type: "int", Required: false},
			{Key: "content_html", Type: "string", Required: false},
		}},
		{Key: wxmp_postprocess_context_key_ctx, Type: "object", Required: false},
		{Key: wxmp_postprocess_context_output_file, Type: "string", Required: false},
	},
	StartNodeID: wxmp_postprocess_flow_node_assemble_html,
	Nodes: map[string]flowengine.NodeDefinition{
		wxmp_postprocess_flow_node_assemble_html: AssembleHTMLNode,
		wxmp_postprocess_flow_node_cleanup:       CleanupEmbeddedImageResourcesNode,
		wxmp_postprocess_flow_node_done: {
			ID:          wxmp_postprocess_flow_node_done,
			Name:        "done",
			Type:        "EndNode",
			InputSchema: []flowengine.FieldSchema{},
			OutputSchema: []flowengine.FieldSchema{
				{Key: wxmp_postprocess_context_final_html, Type: "string", Required: false},
			},
		},
	},
}

func run_wxmp_node(values map[string]interface{}, node_id string, fn func(*wxmp_postprocess_run) (interface{}, error)) (interface{}, error) {
	run, err := wxmp_postprocess_run_from_context(values)
	if err != nil {
		return nil, err
	}
	task_id := 0
	if run.task != nil {
		task_id = run.task.ID
	}
	input := format_wxmp_context_snapshot(values)
	run.logger.Info().Int("task_id", task_id).Msgf("Postprocess.wxmp: node=%s status=started input=%s", node_id, input)

	output, err := fn(run)
	if err != nil {
		run.logger.Info().Int("task_id", task_id).Msgf("Postprocess.wxmp: node=%s status=failed input=%s output=%s error=%v",
			node_id, input, format_wxmp_context_snapshot(values), err)
		return nil, err
	}

	run.logger.Info().Int("task_id", task_id).Msgf("Postprocess.wxmp: node=%s status=succeeded input=%s output=%s",
		node_id, input, format_wxmp_context_snapshot(values))
	return output, nil
}

func log_wxmp_flow(flow_id string, status string, run *wxmp_postprocess_run, err error) {
	if run == nil {
		return
	}
	task_id := 0
	if run.task != nil {
		task_id = run.task.ID
	}
	if err != nil {
		run.logger.Info().Int("task_id", task_id).Msgf("Postprocess.wxmp: flow=%s status=%s error=%v", flow_id, status, err)
		return
	}
	run.logger.Info().Int("task_id", task_id).Msgf("Postprocess.wxmp: flow=%s status=%s", flow_id, status)
}

func log_wxmp_flow_node_execution(run *wxmp_postprocess_run, flow_engine *flowengine.FlowEngine, flow_id, instance_id string, err error) {
	if run == nil || flow_engine == nil || instance_id == "" {
		return
	}

	record, has_record := flow_engine.GetRunSnapshot(instance_id)
	if !has_record {
		if err != nil {
			run.logger.Info().Msgf("Postprocess.wxmp: flow=%s node_execution_summary unavailable: %v", flow_id, err)
		} else {
			run.logger.Info().Msgf("Postprocess.wxmp: flow=%s node_execution_summary unavailable", flow_id)
		}
		return
	}

	_, node_states := flow_engine.GetRunContext(instance_id)
	node_ids := make([]string, 0, len(record.NodeAttempts))
	for node_id := range record.NodeAttempts {
		node_ids = append(node_ids, node_id)
	}
	for node_id := range node_states {
		exists := false
		for _, existing_id := range node_ids {
			if existing_id == node_id {
				exists = true
				break
			}
		}
		if !exists {
			node_ids = append(node_ids, node_id)
		}
	}
	if len(node_ids) == 0 {
		run.logger.Info().Msgf("Postprocess.wxmp: flow=%s node_execution_summary: no nodes were executed", flow_id)
		return
	}

	sort.Strings(node_ids)
	for _, node_id := range node_ids {
		state := "unknown"
		attempts := record.NodeAttempts[node_id]
		if attempts == 0 {
			state = "skipped"
		}
		if len(node_states) > 0 {
			if node_state, ok := node_states[node_id]; ok {
				switch node_state {
				case flowengine.StateCompleted:
					state = "succeeded"
				case flowengine.StateFailed:
					state = "failed"
				case flowengine.StateRunning, flowengine.StateRetrying:
					state = "running"
				case flowengine.StatePending:
					state = "pending"
				default:
					state = string(node_state)
				}
			}
		}
		if state == "unknown" && attempts > 0 && record.Status == flowengine.RunStatusFailed && record.CurrentNode == node_id {
			state = "failed"
		}
		if state == "unknown" && attempts > 0 && record.Status == flowengine.RunStatusCompleted {
			state = "succeeded"
		}

		task_suffix := ""
		if attempts > 0 {
			task_suffix = fmt.Sprintf(" attempts=%d", attempts)
		}
		run.logger.Info().Msgf("Postprocess.wxmp: flow=%s node=%s status=%s%s", flow_id, node_id, state, task_suffix)
	}
}

func format_wxmp_context_snapshot(values map[string]interface{}) string {
	if values == nil {
		return "{}"
	}

	snapshot := make(map[string]interface{}, len(values))
	for key, value := range values {
		if key == wxmp_postprocess_run_key || key == wxmp_postprocess_context_key_ctx {
			continue
		}
		snapshot[key] = value
	}

	if run, ok := values[wxmp_postprocess_run_key].(*wxmp_postprocess_run); ok && run != nil {
		task_id := 0
		resource_count := 0
		if run.task != nil {
			task_id = run.task.ID
			resource_count = len(run.task.Resources)
		}
		snapshot["task_id"] = task_id
		snapshot["task_name"] = run.task_name
		snapshot["download_dir"] = run.download_dir
		snapshot["resource_count"] = resource_count
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
		b.WriteString(format_wxmp_context_value(snapshot[key]))
	}
	b.WriteString("}")
	return b.String()
}

func format_wxmp_context_value(value interface{}) string {
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

func run_wxmp_postprocess_flow(flow flowengine.FlowDefinition, data map[string]interface{}) error {
	if data == nil {
		data = map[string]interface{}{}
	}
	if _, ok := data[wxmp_postprocess_context_key_ctx]; !ok {
		data[wxmp_postprocess_context_key_ctx] = context.Background()
	}
	run, _ := data[wxmp_postprocess_run_key].(*wxmp_postprocess_run)
	log_wxmp_flow(flow.ID, "started", run, nil)
	if run != nil {
		run.logger.Info().Msgf("Postprocess.wxmp: flow=%s input=%s", flow.ID, format_wxmp_context_snapshot(data))
	}

	flow_engine := flowengine.NewWorkflowEngine()
	flow_engine.SetFlowDefinitions(map[string]flowengine.FlowDefinition{
		flow.ID: flow,
	})
	instance_id, err := flow_engine.StartFlow(flow.ID, data)
	if err != nil {
		log_wxmp_flow(flow.ID, "failed", run, err)
		if run != nil {
			log_wxmp_flow_node_execution(run, flow_engine, flow.ID, instance_id, err)
		}
		return err
	}
	log_wxmp_flow(flow.ID, "succeeded", run, nil)
	if run != nil {
		log_wxmp_flow_node_execution(run, flow_engine, flow.ID, instance_id, nil)
	}
	return err
}

// AssembleFullHTML wraps body HTML into a complete standalone HTML document.
func AssembleFullHTML(title, body_html string) string {
	return assemble_wxmp_full_html(title, body_html, false)
}

// assemble_album_full_html wraps album body HTML into a complete HTML document with album layout.
func assemble_album_full_html(title, body_html string) string {
	return assemble_wxmp_full_html(title, body_html, true)
}

func assemble_wxmp_full_html(title, body_html string, is_album bool) string {
	var b strings.Builder
	var css_builder strings.Builder

	base_max_width := 677
	if is_album {
		base_max_width = 1024
	}
	css_builder.WriteString(`        html { height: 100%; }`)
	css_builder.WriteString(fmt.Sprintf(`
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.6;
            max-width: %dpx;
            margin: 0 auto;
            padding: 20px;
            color: #333;
        }
        h1 { font-size: 1.8em; margin-bottom: 0.5em; }
        img { max-width: 100%%; height: auto; }`, base_max_width))
	if is_album {
		css_builder.WriteString(`
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
	b.WriteString(escape_html_str(title))
	b.WriteString(`</title>
    <style>
`)
	b.WriteString(css_builder.String())
	b.WriteString(`
    </style>
</head>
<body>`)
	b.WriteString(body_html)
	b.WriteString(`</body>
</html>`)
	return b.String()
}

func image_resources_by_url(resources []hermes.ResourceJob) map[string]*hermes.ResourceJob {
	result := make(map[string]*hermes.ResourceJob)
	for i := range resources {
		resource := &resources[i]
		if !is_embeddable_wxmp_image(resource) {
			continue
		}
		for _, endpoint := range resource.Endpoints {
			image_url := normalize_image_url(endpoint.URL)
			if image_url == "" {
				continue
			}
			hash := md5.Sum([]byte(image_url))
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

func is_embeddable_wxmp_image(resource *hermes.ResourceJob) bool {
	if resource == nil || strings.HasSuffix(resource.UniqueID, "_cover") {
		return false
	}
	kind := strings.ToLower(strings.TrimSpace(resource.Kind))
	return kind == "image" || strings.HasPrefix(kind, "image/")
}

// image_resource_to_data_uri reads a downloaded image from its actual Hermes path.
// The temporary path intentionally has no extension, so MIME comes from Kind.
func image_resource_to_data_uri(resource *hermes.ResourceJob) string {
	if resource == nil || strings.TrimSpace(resource.FilePath) == "" {
		return ""
	}
	data, err := os.ReadFile(resource.FilePath)
	if err != nil {
		return ""
	}
	mime := strings.ToLower(strings.TrimSpace(resource.Kind))
	if !strings.HasPrefix(mime, "image/") {
		mime = mime_by_extension(filepath.Ext(resource.FilePath))
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func wxmp_resource_key(resource *hermes.ResourceJob) string {
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

// mime_by_extension maps file extensions to MIME types.
func mime_by_extension(ext string) string {
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

func map_keys_resource(m map[string]*hermes.ResourceJob) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// map_keys_bool returns the keys of a map[string]bool as a slice for debug logging.
func map_keys_bool(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func escape_html_str(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
