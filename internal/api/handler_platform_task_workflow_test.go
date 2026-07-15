package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	contentshuba69 "wx_channel/pkg/contentplatform/69shuba"
	contentdownload "wx_channel/pkg/contentplatform/download"
	contentoa "wx_channel/pkg/contentplatform/officialaccount"
	officialaccountpkg "wx_channel/internal/webcontent/officialaccount"
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

func TestPlatformWorkflowPersistenceRoundTrip(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/workflow-persist.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.PlatformWorkflowRun{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	client := &APIClient{db: db}
	run := client.attachPlatformWorkflowPersistence(newPlatformWorkflowRunWithID("https://example.com/video", map[string]any{"source": "test"}, "run_persist_test"))
	platformWorkflowRuns.Store(run.ID, run)
	defer platformWorkflowRuns.Delete(run.ID)

	run.startNode("pause_after_probe", "user_confirmation")
	run.waitForUserConfirmation("pause_after_probe", &platformWorkflowInteraction{
		Kind:           "confirmation",
		Required:       true,
		Title:          "确认下载内容",
		SubmitLabel:    "开始下载",
		ResumeEndpoint: "/api/task/pipeline/resume",
		Form:           []gin.H{{"name": "variant_id", "default": "video"}},
	})
	run.resumeAfterProbe(map[string]any{
		"variant_id": "video",
		"suffix":     ".mp4",
		"filename":   "demo",
		"options": map[string]any{
			"variant_id": "video",
			"suffix":     ".mp4",
			"filename":   "demo",
		},
	})

	platformWorkflowRuns.Delete(run.ID)
	loaded := client.lookupPlatformWorkflow(run.ID)
	if loaded == nil {
		t.Fatal("loaded workflow is nil")
	}
	if loaded.ID != run.ID || loaded.URL != run.URL || loaded.Status != "running" {
		t.Fatalf("loaded workflow = %#v", loaded)
	}
	if len(loaded.Nodes) != len(run.Nodes) {
		t.Fatalf("loaded nodes len = %d, want %d", len(loaded.Nodes), len(run.Nodes))
	}
	body, ok := platformWorkflowCreateBodyFromSelection(loaded)
	if !ok {
		t.Fatal("selection was not restored")
	}
	if body.RunID != run.ID || body.URL != run.URL {
		t.Fatalf("body identity = %#v", body)
	}
	if body.Options.VariantID != "video" || body.Options.Suffix != ".mp4" || body.Options.Filename != "demo" {
		t.Fatalf("restored options = %#v", body.Options)
	}
}

func TestPlatformProbeContentIncludesAuthorHomepageURL(t *testing.T) {
	content := contentdownload.NewContent(contentdownload.ContentSummary{
		Platform:        "youtube",
		Type:            "video",
		ID:              "video_1",
		Title:           "title",
		AuthorNickname:  "author",
		AuthorAvatarURL: "https://example.com/avatar.jpg",
	}, map[string]any{"id": "video_1"}, map[string]any{
		"channel_url": "https://www.youtube.com/@author",
	}, nil)
	view := platformContentView(content)
	if view["author_homepage_url"] != "https://www.youtube.com/@author" {
		t.Fatalf("author_homepage_url = %#v", view["author_homepage_url"])
	}
	if view["author_avatar_url"] != "https://example.com/avatar.jpg" {
		t.Fatalf("author_avatar_url = %#v", view["author_avatar_url"])
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

func TestPlatformWorkflowWebsocketPublishesNodeEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &APIClient{engine: gin.New()}
	client.engine.GET("/admin", client.handlePlatformWorkflowWebsocket)
	server := httptest.NewServer(client.engine)
	defer server.Close()

	run := newPlatformWorkflowRunWithID("https://example.com/video", nil, "run_ws_test")
	platformWorkflowRuns.Store(run.ID, run)
	defer platformWorkflowRuns.Delete(run.ID)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/admin?run_id=" + run.ID
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial workflow ws: %v", err)
	}
	defer conn.Close()

	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if snapshot["type"] != "pipeline_workflow" || snapshot["event"] != "snapshot" {
		t.Fatalf("snapshot message = %#v", snapshot)
	}

	run.startNode("probe", "probe")
	_, raw, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("read node event: %v", err)
	}
	var event map[string]any
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatalf("decode event: %v", err)
	}
	if event["event"] != "node_started" || event["run_id"] != run.ID {
		t.Fatalf("node event = %#v", event)
	}
	data, ok := event["data"].(map[string]any)
	if !ok {
		t.Fatalf("event data = %#v", event["data"])
	}
	node, ok := data["node"].(map[string]any)
	if !ok || node["id"] != "probe" || node["status"] != "running" {
		t.Fatalf("event node = %#v", data["node"])
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

func TestPlatformProbeFormKeeps69ShubaNovelDefault(t *testing.T) {
	probe := &contentdownload.Probe{
		ID:        "run_69shuba",
		Platform:  contentshuba69.PlatformID,
		SourceURL: "https://www.69shuba.com/book/34567.htm",
		ContentID: "34567",
		Content: contentdownload.NewContent(
			contentdownload.ContentSummary{Platform: contentshuba69.PlatformID, Type: "novel", ID: "34567", Title: "book"},
			map[string]any{"id": "34567"},
			nil,
			nil,
		),
		Variants: []contentdownload.Variant{{ID: "html", Type: "archive", Label: "整本 HTML 文件夹"}},
		Defaults: contentdownload.Defaults{VariantID: "html"},
	}

	platformProbeAddJSONDefault(probe)
	form := platformProbeForm(probe)
	if got := form[0]["default"]; got != "html" {
		t.Fatalf("variant default = %#v, want 69shuba archive html", got)
	}
	if probe.Defaults.VariantID != "html" || probe.Defaults.Suffix != "" {
		t.Fatalf("defaults = %#v, want 69shuba archive html", probe.Defaults)
	}
	options, ok := form[0]["options"].([]contentdownload.Variant)
	if !ok {
		t.Fatalf("options = %T, want []contentdownload.Variant", form[0]["options"])
	}
	foundJSON := false
	for _, option := range options {
		if option.ID == platformJSONVariantID {
			foundJSON = true
		}
	}
	if !foundJSON {
		t.Fatalf("options should still include JSON variant: %#v", options)
	}
}

func TestShuba69DownloadTaskRecordsUseGenericContainerAndCDPFileNodes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.DownloadTask{}, &model.Content{}, &model.Account{}, &model.ContentAccount{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	client := &APIClient{
		db:  db,
		cfg: &APIConfig{Shuba69Fetcher: "cdp"},
	}
	task := &contentdownload.Task{
		ID:       "task_69shuba_34567",
		FilePath: "/tmp/downloads/book",
		Resolved: &contentdownload.ResolvedRequest{
			Platform:     contentshuba69.PlatformID,
			SourceURL:    "https://www.69shuba.com/book/34567.htm",
			CanonicalURL: "https://www.69shuba.com/book/34567/",
			ContentID:    "34567",
			Title:        "book",
			Filename:     "book",
			Download: contentdownload.DownloadSpec{
				URL:      "69shuba-archive://34567",
				Protocol: contentshuba69.ArchiveProtocol,
			},
			Metadata: map[string]any{"full_catalog_url": "https://www.69shuba.com/book/34567/"},
			Content: contentdownload.NewContent(
				contentdownload.ContentSummary{Platform: contentshuba69.PlatformID, Type: "novel", ID: "34567", Title: "book"},
				&contentshuba69.Novel{
					Title: "book",
					Chapters: []contentshuba69.Chapter{
						{Index: 1, Title: "chapter 1", URL: "https://www.69shuba.com/txt/34567/1001"},
					},
				},
				nil,
				nil,
			),
		},
		Files: []contentdownload.FileNode{
			{
				Name: "source", Path: "source", Type: contentdownload.FileNodeTypeDir, Status: contentdownload.FileNodeStatusDone,
				Children: []contentdownload.FileNode{
					{Name: "book.html", Path: "source/book.html", Type: contentdownload.FileNodeTypeFile, Status: contentdownload.FileNodeStatusDone, Size: 10},
					{Name: "full_catalog.html", Path: "source/full_catalog.html", Type: contentdownload.FileNodeTypeFile, Status: contentdownload.FileNodeStatusDone, Size: 20},
				},
			},
			{
				Name: "chapters", Path: "chapters", Type: contentdownload.FileNodeTypeDir, Status: contentdownload.FileNodeStatusPending,
				Children: []contentdownload.FileNode{
					{Name: "chapter 1.html", Path: "chapters/chapter 1.html", Type: contentdownload.FileNodeTypeFile, Status: contentdownload.FileNodeStatusPending},
				},
			},
		},
	}

	parent, err := client.createPlatformDownloadTaskRecord(task)
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}
	if parent.NodeType != downloadNodeTypeContainer || parent.Engine != "" {
		t.Fatalf("parent node/engine = %q/%q, want container/empty", parent.NodeType, parent.Engine)
	}
	if parent.Protocol != "" || parent.URL != "" || parent.SourceURI != "" || parent.OutputPath != "" {
		t.Fatalf("parent source fields = protocol %q url %q source %q output %q, want empty", parent.Protocol, parent.URL, parent.SourceURI, parent.OutputPath)
	}

	client.syncPlatformDownloadTaskChildren(parent.Id, task)
	var children []model.DownloadTask
	if err := db.Where("parent_id = ?", parent.Id).Order("idx ASC").Find(&children).Error; err != nil {
		t.Fatalf("query children: %v", err)
	}
	if len(children) != 3 {
		t.Fatalf("children len = %d, want 3", len(children))
	}
	chapter := children[2]
	if chapter.NodeType != downloadNodeTypeFile || chapter.Engine != downloadEngineCDP || chapter.Protocol != "file" {
		t.Fatalf("chapter node/engine/protocol = %q/%q/%q", chapter.NodeType, chapter.Engine, chapter.Protocol)
	}
	if chapter.SourceURI != "https://www.69shuba.com/txt/34567/1001" || chapter.URL != chapter.SourceURI {
		t.Fatalf("chapter source = url %q source_uri %q", chapter.URL, chapter.SourceURI)
	}
	if chapter.OutputPath != "" {
		t.Fatalf("chapter output_path = %q, want empty", chapter.OutputPath)
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(chapter.Metadata), &meta); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if meta["role"] != "chapter" || meta["chapter_url"] != chapter.SourceURI {
		t.Fatalf("chapter metadata = %#v", meta)
	}

	var chapterContent model.Content
	if err := db.First(&chapterContent, "download_task_id = ?", chapter.Id).Error; err != nil {
		t.Fatalf("query chapter content: %v", err)
	}
	if chapterContent.ContentType != "html" || chapterContent.ExternalId != "34567#file:chapters/chapter 1.html" {
		t.Fatalf("chapter content type/external_id = %q/%q", chapterContent.ContentType, chapterContent.ExternalId)
	}
	if chapterContent.DownloadPath != "/tmp/downloads/book/chapters/chapter 1.html" {
		t.Fatalf("chapter content download_path = %q", chapterContent.DownloadPath)
	}
	var contentMeta map[string]any
	if err := json.Unmarshal([]byte(chapterContent.Metadata), &contentMeta); err != nil {
		t.Fatalf("decode content metadata: %v", err)
	}
	if contentMeta["output_format"] != "html" || contentMeta["tree_path"] != "chapters/chapter 1.html" {
		t.Fatalf("chapter content metadata = %#v", contentMeta)
	}
	var contentCount int64
	if err := db.Model(&model.Content{}).Count(&contentCount).Error; err != nil {
		t.Fatalf("count content: %v", err)
	}
	if contentCount != 3 {
		t.Fatalf("content count = %d, want 3", contentCount)
	}
}

func TestPlatformResolvedContentUsesActualJSONOutput(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.DownloadTask{}, &model.Content{}, &model.Account{}, &model.ContentAccount{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	client := &APIClient{db: db}
	resolved := &contentdownload.ResolvedRequest{
		Platform:  "zhihu",
		SourceURL: "https://www.zhihu.com/question/1/answer/2",
		ContentID: "answer_2",
		Title:     "answer title",
		Filename:  "answer title",
		Suffix:    ".json",
		Download: contentdownload.DownloadSpec{
			URL:      "inline-json://zhihu/answer_2",
			Protocol: "inline_json",
		},
		Labels: map[string]string{
			"id":           "answer_2",
			"content_type": "answer",
			"suffix":       ".json",
		},
		Metadata: map[string]any{
			"variant_id": platformJSONVariantID,
		},
		Content: contentdownload.NewContent(
			contentdownload.ContentSummary{Platform: "zhihu", Type: "answer", ID: "answer_2", Title: "answer title"},
			map[string]any{"id": "answer_2"},
			nil,
			nil,
		),
	}
	rec := model.DownloadTask{
		TaskId:     "task_json",
		Status:     4,
		ExternalId: "answer_2",
		URL:        "inline-json://zhihu/answer_2",
		Title:      "answer title",
		Filename:   "answer title.json",
		MimeType:   "application/json",
		Size:       123,
		Filepath:   "/tmp/downloads/answer title.json",
		Timestamps: model.Timestamps{
			CreatedAt: 100,
			UpdatedAt: 200,
		},
	}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	content, err := client.upsertPlatformResolvedContent(resolved, rec.Id, nil)
	if err != nil {
		t.Fatalf("upsert content: %v", err)
	}
	if content == nil {
		t.Fatal("content is nil")
	}
	if content.ContentType != "json" || content.ExternalId != "answer_2#json" {
		t.Fatalf("content type/external_id = %q/%q", content.ContentType, content.ExternalId)
	}
	if content.DownloadPath != "/tmp/downloads/answer title.json" || content.DownloadStatus != 4 {
		t.Fatalf("download path/status = %q/%d", content.DownloadPath, content.DownloadStatus)
	}
	var stored model.Content
	if err := db.First(&stored, "external_id = ?", "answer_2#json").Error; err != nil {
		t.Fatalf("query content: %v", err)
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(stored.Metadata), &meta); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if meta["output_format"] != "json" || meta["mime_type"] != "application/json" || meta["source_content_type"] != "answer" {
		t.Fatalf("metadata = %#v", meta)
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
	if err := db.Where("platform_id = ? AND external_id = ?", "test_platform", "content_1#json").First(&content).Error; err != nil {
		t.Fatalf("load content: %v", err)
	}
	if content.Title != "content title" || content.ContentType != "json" || content.DownloadTaskId == nil || *content.DownloadTaskId != downloadTask.Id {
		t.Fatalf("unexpected content: %#v", content)
	}
	var contentMeta map[string]any
	if err := json.Unmarshal([]byte(content.Metadata), &contentMeta); err != nil {
		t.Fatalf("decode content metadata: %v", err)
	}
	if contentMeta["output_format"] != "json" || contentMeta["source_content_type"] != "article" {
		t.Fatalf("unexpected content metadata: %#v", contentMeta)
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
