package services

import (
	"strings"
	"sync"
	"testing"

	"wx_channel/internal/database/model"
)

func TestAutomationActiveRunBlocksEveryTriggerOfSameFlow(t *testing.T) {
	for _, status := range []string{"QUEUED", "RUNNING", "WAITING"} {
		t.Run(status, func(t *testing.T) {
			service := new_trigger_test_service(t)
			t.Cleanup(service.Stop)
			flow := trigger_test_flow(t, service)
			other_flow := trigger_test_flow(t, service)
			rule, err := service.CreateSchedule(CreateScheduleInput{Name: "second trigger", FlowID: flow.ID, CronExpr: "@daily"})
			if err != nil {
				t.Fatal(err)
			}
			active := model.FlowRunRecord{ID: "active", FlowID: flow.ID, ScheduleID: "first-trigger", TriggerType: "Cron", Status: status}
			if err := service.db.Create(&active).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := service.TriggerFlowDirect(flow.ID, nil); err == nil || !strings.Contains(err.Error(), "已有进行中的执行") {
				t.Fatalf("manual trigger must be blocked: %v", err)
			}
			if _, err := service.TriggerSchedule(rule.ID); err == nil {
				t.Fatal("another trigger of the same flow was allowed")
			}
			copied, err := service.CopyUserFlow(flow.ID)
			if err != nil || copied.ActiveRunID != "" {
				t.Fatalf("copied flow inherited the source run: %+v, %v", copied, err)
			}
			loaded, err := service.GetUserFlow(flow.ID)
			if err != nil || loaded.ActiveRunID != active.ID {
				t.Fatalf("detail must expose active run: %+v, %v", loaded, err)
			}
			flows, err := service.ListUserFlows()
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range flows {
				if item.ID == flow.ID && item.ActiveRunID != active.ID || item.ID == other_flow.ID && item.ActiveRunID != "" {
					t.Fatalf("wrong active run in list: %+v", item)
				}
			}
			if _, err := service.TriggerFlowDirect(other_flow.ID, nil); err != nil {
				t.Fatalf("unrelated flow was blocked: %v", err)
			}
			service.wg.Wait()
			for _, ended_status := range []string{"COMPLETED", "FAILED", "CANCELLED"} {
				if err := service.db.Model(&active).Update("status", ended_status).Error; err != nil {
					t.Fatal(err)
				}
				if _, err := service.TriggerFlowDirect(flow.ID, nil); err != nil {
					t.Fatalf("%s run must allow another trigger: %v", ended_status, err)
				}
				service.wg.Wait()
			}
			loaded, err = service.GetUserFlow(flow.ID)
			if err != nil || loaded.ActiveRunID != "" {
				t.Fatalf("finished flow still active: %+v, %v", loaded, err)
			}
		})
	}
}

func TestAutomationConcurrentTriggersClaimOnlyOneRun(t *testing.T) {
	service := new_trigger_test_service(t)
	flow := trigger_test_flow(t, service)
	start := make(chan struct{})
	results := make(chan error, 16)
	var workers sync.WaitGroup
	for index := 0; index < cap(results); index++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			<-start
			schedule := model.FlowSchedule{FlowID: flow.ID}
			if index%2 == 0 {
				schedule.ID = "cron-trigger"
			}
			_, err := service.prepare_run(schedule, model.FlowRunTriggerManual, "", "", "")
			results <- err
		}(index)
	}
	close(start)
	workers.Wait()
	close(results)
	accepted := 0
	for err := range results {
		if err == nil {
			accepted++
		} else if !strings.Contains(err.Error(), "已有进行中的执行") {
			t.Fatal(err)
		}
	}
	var count int64
	if err := service.db.Model(&model.FlowRunRecord{}).Where("flow_id = ?", flow.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if accepted != 1 || count != 1 {
		t.Fatalf("accepted %d requests, created %d runs", accepted, count)
	}
}
