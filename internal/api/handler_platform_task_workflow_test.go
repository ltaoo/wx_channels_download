package api

import (
	"encoding/json"
	"testing"

	"github.com/gin-gonic/gin"

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
