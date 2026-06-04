package api

import (
	"testing"

	"github.com/gin-gonic/gin"

	contentdownload "wx_channel/pkg/contentplatform/download"
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
	if contentdownload.ContentTitle(output["content"]) != "title" {
		t.Fatalf("probe output content = %#v", output["content"])
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
