package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	contentdownload "wx_channel/pkg/contentplatform/download"
	contentoa "wx_channel/pkg/contentplatform/officialaccount"
	officialaccountpkg "wx_channel/pkg/officialaccount"
)

func TestPlatformWorkflowUserConfirmationNode(t *testing.T) {
	run := newPlatformWorkflowRun("https://example.com/video", nil)
	interaction := &platformWorkflowInteraction{
		Kind:           "confirmation",
		Required:       true,
		Title:          "确认下载内容",
		SubmitLabel:    "开始下载",
		ResumeEndpoint: "/api/task/pipeline/resume",
		Output:         map[string]any{"body_html": "<p>body</p>"},
	}

	run.startNode("pause_after_probe", "user_confirmation")
	run.waitForUserConfirmation("pause_after_probe", interaction)

	if run.Status != "paused" {
		t.Fatalf("workflow status = %q, want paused", run.Status)
	}
	if run.CurrentNode != "pause_after_probe" {
		t.Fatalf("current node = %q, want pause_after_probe", run.CurrentNode)
	}
	if got := run.currentInteractionView(); got == nil || got["kind"] != "confirmation" || got["required"] != true {
		t.Fatalf("interaction = %#v, want required confirmation", got)
	}
	if len(run.Nodes) != 1 {
		t.Fatalf("nodes len = %d, want 1", len(run.Nodes))
	}
	if got := run.Nodes[0]; got.Type != "user_confirmation" || got.Status != "waiting_user" {
		t.Fatalf("node = %#v, want user_confirmation waiting_user", got)
	}
	if len(run.Nodes[0].Output) != 0 {
		t.Fatalf("waiting confirmation node output = %#v, want empty", run.Nodes[0].Output)
	}

	run.resumeAfterProbe(map[string]any{"variant_id": "html"})
	if run.Status != "running" {
		t.Fatalf("workflow status after resume = %q, want running", run.Status)
	}
	if run.Nodes[0].Status != "completed" {
		t.Fatalf("confirmation node status after resume = %q, want completed", run.Nodes[0].Status)
	}
	if got := run.Nodes[0].Output["variant_id"]; got != "html" {
		t.Fatalf("confirmation node output variant_id = %#v", got)
	}
	if got := run.Nodes[len(run.Nodes)-1]; got.ID != "resume_after_probe" || got.Type != "resume" {
		t.Fatalf("last node = %#v, want resume_after_probe resume", got)
	}
}

func TestPlatformWorkflowPublicViewsDoNotDuplicateOutput(t *testing.T) {
	probe := &contentdownload.Probe{
		ID:       "run_1",
		Platform: "zhihu",
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform: "zhihu",
			Title:    "title",
			ID:       "answer_1",
		}, map[string]any{"answer": "payload"}, nil, map[string]any{"body_html": "<p>body</p>"}),
		Internal: map[string]any{"page": map[string]any{"content": "<p>body</p>"}},
	}
	probeView := platformProbeView(probe)
	if _, ok := probeView["output"]; ok {
		t.Fatalf("probe view should not include output: %#v", probeView)
	}
	if _, ok := probeView["metadata"]; ok {
		t.Fatalf("probe view should not include metadata: %#v", probeView)
	}
	contentView, ok := probeView["content"].(gin.H)
	if !ok {
		t.Fatalf("probe view content = %#v", probeView["content"])
	}
	if _, ok := contentView["data"]; ok {
		t.Fatalf("probe view content should not include data: %#v", contentView)
	}
	output := platformProbeOutput(probe)
	if output["body_html"] != "<p>body</p>" {
		t.Fatalf("probe output body_html = %#v", output["body_html"])
	}
	if _, ok := output["content"]; ok {
		t.Fatalf("probe output should not include content: %#v", output)
	}

	run := newPlatformWorkflowRun("https://example.com", nil)
	run.Output = platformProbeOutput(probe)
	run.startNode("pause_after_probe", "user_confirmation")
	run.waitForUserConfirmation("pause_after_probe", &platformWorkflowInteraction{
		Kind:           "confirmation",
		Required:       true,
		Title:          "确认下载内容",
		SubmitLabel:    "开始下载",
		ResumeEndpoint: "/api/task/pipeline/resume",
		Output:         run.Output,
	})
	snapshot := run.snapshot()
	if _, ok := snapshot["output"]; ok {
		t.Fatalf("workflow snapshot should not include output: %#v", snapshot)
	}
	nodes, ok := snapshot["nodes"].([]gin.H)
	if !ok || len(nodes) != 1 {
		t.Fatalf("nodes = %#v", snapshot["nodes"])
	}
	if _, ok := nodes[0]["output"]; ok {
		t.Fatalf("workflow node should not include output: %#v", nodes[0])
	}
	interaction, ok := nodes[0]["interaction"].(gin.H)
	if !ok {
		t.Fatalf("node interaction = %#v", nodes[0]["interaction"])
	}
	if _, ok := interaction["output"]; ok {
		t.Fatalf("interaction summary should not include output: %#v", interaction)
	}
	if _, ok := interaction["form"]; !ok {
		t.Fatalf("interaction summary should include form: %#v", interaction)
	}
}

func TestPlatformOfficialAccountProbeViewsOmitDuplicateArticleBody(t *testing.T) {
	bodyHTML := "<p>unique official account body that must not be duplicated</p>"
	pageHTML := "<html><body>raw official account page</body></html>"
	pageJSON := map[string]any{"title": "article", "page_type": float64(2)}
	probe := &contentdownload.Probe{
		ID:        "run_oa",
		Platform:  contentoa.PlatformID,
		SourceURL: "https://mp.weixin.qq.com/s/demo",
		ContentID: "demo",
		Content: contentoa.NewArticleContentEnvelope(
			contentdownload.ContentSummary{
				Platform:  contentoa.PlatformID,
				Type:      contentoa.ContentTypeArticle,
				ID:        "demo",
				Title:     "article",
				Author:    "author",
				SourceURL: "https://mp.weixin.qq.com/s/demo",
				CoverURL:  "https://example.com/cover.jpg",
			},
			&officialaccountpkg.WechatOfficialArticle{
				Title:          "article",
				AuthorNickname: "author",
				AuthorID:       "author-id",
				Content:        bodyHTML,
				Images:         []string{"https://example.com/cover.jpg"},
			},
			contentoa.ArticleMetadata{ArticleID: "demo", AuthorID: "author-id"},
			contentoa.ArticleOutput{
				Format:      contentoa.OutputFormatHTML,
				ContentType: contentoa.ContentTypeArticle,
				ArticleID:   "demo",
				Title:       "article",
				SourceURL:   "https://mp.weixin.qq.com/s/demo",
				BodyHTML:    bodyHTML,
			},
		),
		Internal: map[string]any{
			"pagejson": pageJSON,
			"pagehtml": pageHTML,
		},
	}

	output := platformProbeOutput(probe)
	if output["body_html"] != bodyHTML {
		t.Fatalf("official account output body_html = %#v", output["body_html"])
	}
	if _, ok := output["content"]; ok {
		t.Fatalf("official account output should not include content envelope: %#v", output)
	}
	if output["content_type"] != contentoa.ContentTypeArticle {
		t.Fatalf("content_type = %#v", output["content_type"])
	}

	responseView := gin.H{
		"content":  platformProbeContent(probe),
		"probe":    platformProbeView(probe),
		"output":   output,
		"pagejson": platformProbePageJSON(probe),
		"pagehtml": platformProbePageHTML(probe),
	}
	data, err := json.Marshal(responseView)
	if err != nil {
		t.Fatalf("marshal response view: %v", err)
	}
	decoded := map[string]any{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal response view: %v", err)
	}
	if got := countStringValue(decoded, bodyHTML); got != 1 {
		t.Fatalf("response view body html count = %d, want 1: %s", got, data)
	}
	if got := countStringValue(decoded, pageHTML); got != 1 {
		t.Fatalf("response view page html count = %d, want 1: %s", got, data)
	}
	pageJSONView, ok := decoded["pagejson"].(map[string]any)
	if !ok || pageJSONView["title"] != "article" || pageJSONView["page_type"] != float64(2) {
		t.Fatalf("pagejson = %#v", decoded["pagejson"])
	}
}

func countStringValue(value any, target string) int {
	switch v := value.(type) {
	case string:
		if v == target {
			return 1
		}
	case []any:
		total := 0
		for _, item := range v {
			total += countStringValue(item, target)
		}
		return total
	case map[string]any:
		total := 0
		for _, item := range v {
			total += countStringValue(item, target)
		}
		return total
	}
	return 0
}

func TestPlatformProbeFormAddsJSONDefault(t *testing.T) {
	probe := &contentdownload.Probe{
		ID:        "run_1",
		Platform:  "zhihu",
		SourceURL: "https://example.com/content",
		ContentID: "content_1",
		Content:   contentdownload.NewContent(contentdownload.ContentSummary{Platform: "zhihu", Type: "answer", ID: "content_1", Title: "title"}, map[string]any{"id": "content_1"}, nil, nil),
		Variants:  []contentdownload.Variant{{ID: "html", Type: "html", Label: "HTML", Suffix: ".html"}},
		Defaults:  contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
		Warnings:  []string{"warning"},
	}

	platformProbeAddJSONDefault(probe)
	form := platformProbeForm(probe)
	if len(form) == 0 {
		t.Fatalf("form is empty")
	}
	if got := form[0]["default"]; got != platformJSONVariantID {
		t.Fatalf("variant default = %#v, want json", got)
	}
	options, ok := form[0]["options"].([]contentdownload.Variant)
	if !ok {
		t.Fatalf("options = %T, want []contentdownload.Variant", form[0]["options"])
	}
	foundJSON := false
	for _, option := range options {
		if option.ID == platformJSONVariantID && option.Type == "json" && option.Label == "JSON" && option.Suffix == ".json" {
			foundJSON = true
		}
	}
	if !foundJSON {
		t.Fatalf("options should include JSON variant: %#v", options)
	}

	resolved := platformJSONResolvedRequest("https://example.com/content", probe, contentdownload.Options{})
	if resolved.Suffix != ".json" || resolved.Download.Protocol != "inline_json" {
		t.Fatalf("json resolved = %#v", resolved)
	}
	if got := resolved.Metadata["variant_id"]; got != platformJSONVariantID {
		t.Fatalf("json variant_id metadata = %#v", got)
	}
}

func TestPlatformProbeFormCanDisableJSONVariant(t *testing.T) {
	probe := &contentdownload.Probe{
		ID:        "run_html_only",
		Platform:  "html_only",
		SourceURL: "https://example.com/page",
		ContentID: "page_1",
		Content:   contentdownload.NewContent(contentdownload.ContentSummary{Platform: "html_only", Type: "article", ID: "page_1", Title: "title"}, map[string]any{"id": "page_1"}, nil, nil),
		Variants:  []contentdownload.Variant{{ID: "html", Type: "html", Label: "HTML", Suffix: ".html"}},
		Defaults:  contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
		Internal:  map[string]any{contentdownload.InternalKeyDisableJSONVariant: true},
	}

	platformProbeAddJSONDefault(probe)
	if probe.Defaults.VariantID != "html" || probe.Defaults.Suffix != ".html" {
		t.Fatalf("defaults changed despite disabled json: %#v", probe.Defaults)
	}
	form := platformProbeForm(probe)
	options, ok := form[0]["options"].([]contentdownload.Variant)
	if !ok {
		t.Fatalf("options = %T, want []contentdownload.Variant", form[0]["options"])
	}
	for _, option := range options {
		if option.ID == platformJSONVariantID {
			t.Fatalf("json variant should be disabled: %#v", options)
		}
	}
}

func TestPlatformWorkflowResumeCreatesAccountAndContentRecords(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/workflow.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.DownloadTask{}, &model.Account{}, &model.Content{}, &model.ContentAccount{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	client := &APIClient{
		db:  db,
		cfg: &APIConfig{DownloadDir: t.TempDir()},
	}
	run := newPlatformWorkflowRun("https://example.com/content/1", nil)
	run.Probe = &contentdownload.Probe{
		ID:           run.ID,
		Platform:     "test_platform",
		SourceURL:    run.URL,
		CanonicalURL: "https://example.com/canonical/1",
		ContentID:    "content_1",
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:        "test_platform",
			Type:            "article",
			ID:              "content_1",
			Title:           "content title",
			Description:     "content description",
			Author:          "author nickname",
			URL:             "https://example.com/canonical/1",
			SourceURL:       "https://example.com/content/1",
			AuthorNickname:  "author nickname",
			AuthorAvatarURL: "https://example.com/avatar.jpg",
			CoverURL:        "https://example.com/cover.jpg",
			Duration:        12,
		}, map[string]any{"id": "content_1"}, map[string]any{
			"author_id":       "author_1",
			"author_username": "author_name",
		}, map[string]any{"body_html": "<p>body</p>"}),
		Variants: []contentdownload.Variant{{ID: platformJSONVariantID, Type: "json", Label: "JSON", Suffix: ".json"}},
		Defaults: contentdownload.Defaults{VariantID: platformJSONVariantID, Suffix: ".json"},
	}
	run.startNode("pause_after_probe", "user_confirmation")
	run.waitForUserConfirmation("pause_after_probe", platformProbeConfirmation(run.Probe, nil))
	platformWorkflowRuns.Store(run.ID, run)
	defer platformWorkflowRuns.Delete(run.ID)

	taskID, err := client.startPlatformDownloadTask(context.Background(), platformCreateTaskBody{
		URL:       run.URL,
		RunID:     run.ID,
		VariantID: platformJSONVariantID,
		Options:   contentdownload.Options{VariantID: platformJSONVariantID},
	})
	if err != nil {
		t.Fatalf("startPlatformDownloadTask() error = %v", err)
	}
	if taskID == "" {
		t.Fatal("task id is empty")
	}

	assertWorkflowNodeCompleted(t, run, "create_account")
	assertWorkflowNodeCompleted(t, run, "create_content")
	if run.Resolved == nil || run.Resolved.Pipeline == nil {
		t.Fatal("resolved pipeline is empty")
	}
	if got := platformPipelineNodeIDByType(run.Resolved.Pipeline, "create_account"); got != "create_account" {
		t.Fatalf("create_account plan node = %q", got)
	}
	if got := platformPipelineNodeIDByType(run.Resolved.Pipeline, "create_content"); got != "create_content" {
		t.Fatalf("create_content plan node = %q", got)
	}

	var account model.Account
	if err := db.Where("platform_id = ? AND external_id = ?", "test_platform", "author_1").First(&account).Error; err != nil {
		t.Fatalf("load account: %v", err)
	}
	if account.Username != "author_name" || account.Nickname != "author nickname" {
		t.Fatalf("unexpected account: %#v", account)
	}

	var downloadTask model.DownloadTask
	if err := db.Where("task_id = ?", taskID).First(&downloadTask).Error; err != nil {
		t.Fatalf("load download task: %v", err)
	}

	var content model.Content
	if err := db.Where("platform_id = ? AND external_id = ?", "test_platform", "content_1").First(&content).Error; err != nil {
		t.Fatalf("load content: %v", err)
	}
	if content.Title != "content title" || content.ContentType != "article" || content.DownloadTaskId == nil || *content.DownloadTaskId != downloadTask.Id {
		t.Fatalf("unexpected content: %#v", content)
	}

	var link model.ContentAccount
	if err := db.Where("content_id = ? AND account_id = ?", content.Id, account.Id).First(&link).Error; err != nil {
		t.Fatalf("load content account link: %v", err)
	}
	if link.Role != "owner" {
		t.Fatalf("unexpected content account link: %#v", link)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		run.mu.Lock()
		status := run.Status
		run.mu.Unlock()
		if status == "completed" || status == "failed" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestPlatformPlanMetadataNodesUseComposableNodeIDs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/workflow.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Account{}, &model.Content{}, &model.ContentAccount{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	client := &APIClient{db: db}
	run := newPlatformWorkflowRun("https://example.com/content/2", nil)
	resolved := &contentdownload.ResolvedRequest{
		Platform:  "test_platform",
		ContentID: "content_2",
		Title:     "content title",
		Suffix:    ".json",
		Download:  contentdownload.DownloadSpec{URL: "inline-json://test_platform/content_2", Protocol: "inline_json"},
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:       "test_platform",
			Type:           "article",
			ID:             "content_2",
			Title:          "content title",
			AuthorNickname: "author nickname",
		}, map[string]any{}, map[string]any{
			"author_id": "author_2",
		}, nil),
		Pipeline: &contentdownload.PipelinePlan{
			Platform: "test_platform",
			Nodes: []contentdownload.PipelineNode{
				{ID: "content_upsert", Type: "create_content", Stage: "prepare", DependsOn: []string{"account_upsert"}},
				{ID: "account_upsert", Type: "create_account", Stage: "prepare"},
			},
		},
	}
	platformEnsureMetadataPipelineNodes(resolved)
	if len(resolved.Pipeline.Nodes) != 2 {
		t.Fatalf("custom pipeline nodes should not be duplicated: %#v", resolved.Pipeline.Nodes)
	}
	run.Resolved = resolved
	err = client.runPlatformPlanNodeTypes(context.Background(), run, &platformPlanExecution{
		Resolved:       resolved,
		DownloadTaskID: 10,
	}, map[string]bool{"create_account": true, "create_content": true})
	if err != nil {
		t.Fatalf("runPlatformPlanNodeTypes() error = %v", err)
	}
	assertWorkflowNodeCompleted(t, run, "account_upsert")
	assertWorkflowNodeCompleted(t, run, "content_upsert")
	if len(run.Nodes) < 2 || run.Nodes[0].ID != "account_upsert" || run.Nodes[1].ID != "content_upsert" {
		t.Fatalf("workflow node order = %#v", run.Nodes)
	}
}

func assertWorkflowNodeCompleted(t *testing.T, run *platformWorkflowRun, id string) {
	t.Helper()
	run.mu.Lock()
	defer run.mu.Unlock()
	for _, node := range run.Nodes {
		if node.ID == id {
			if node.Status != "completed" {
				t.Fatalf("node %s status = %q, want completed", id, node.Status)
			}
			return
		}
	}
	t.Fatalf("node %s not found in %#v", id, run.Nodes)
}
