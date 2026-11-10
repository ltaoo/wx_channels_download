package services

import (
	"fmt"
	"testing"

	"wx_channel/internal/database/model"
)

func TestAutomationClearRuns(t *testing.T) {
	service := new_trigger_test_service(t)
	runs := make([]model.FlowRunRecord, 0, 30)
	for index := 0; index < 25; index++ {
		runs = append(runs, model.FlowRunRecord{ID: fmt.Sprintf("finished-%d", index), FlowID: "target", Status: []string{
			model.FlowRunStatusCompleted, model.FlowRunStatusFailed, model.FlowRunStatusCancelled,
		}[index%3]})
	}
	for _, status := range []string{model.FlowRunStatusQueued, model.FlowRunStatusRunning, model.FlowRunStatusWaiting} {
		runs = append(runs, model.FlowRunRecord{ID: status, FlowID: "target", Status: status})
	}
	runs = append(runs, model.FlowRunRecord{ID: "other", FlowID: "other-flow", Status: model.FlowRunStatusCompleted})
	if err := service.db.Create(&runs).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := service.ClearRuns("  "); err == nil {
		t.Fatal("missing flow_id must not clear any records")
	}
	cleared, err := service.ClearRuns(" target ")
	if err != nil || cleared != 25 {
		t.Fatalf("clear all pages: cleared=%d, err=%v", cleared, err)
	}
	remaining, total, err := service.ListRuns(ListRunsInput{FlowID: "target"})
	if err != nil || total != 3 || len(remaining) != 3 {
		t.Fatalf("active runs were removed: %+v, total=%d, err=%v", remaining, total, err)
	}
	for _, run := range remaining {
		if _, err := service.GetRun(run.ID); err != nil {
			t.Fatalf("active run unavailable: %v", err)
		}
	}
	if _, err := service.GetRun("finished-0"); err == nil {
		t.Fatal("cleared record still accessible")
	}
	if _, err := service.GetRun("other"); err != nil {
		t.Fatalf("another flow was affected: %v", err)
	}
	var saved model.FlowRunRecord
	if err := service.db.Where("id = ?", "finished-0").First(&saved).Error; err != nil || saved.DeletedAt == nil {
		t.Fatalf("record was not soft-deleted: %+v, err=%v", saved, err)
	}
	if cleared, err := service.ClearRuns("target"); err != nil || cleared != 0 {
		t.Fatalf("repeated clear failed: cleared=%d, err=%v", cleared, err)
	}
}
