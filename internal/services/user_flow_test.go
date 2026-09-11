package services

import (
	"testing"

	"wx_channel/pkg/flowengine/engine"
)

func new_user_flow_service(t *testing.T) *AutomationService {
	t.Helper()
	return new_verify_automation_service(t)
}

func TestCreateUserFlowProducesStartOnlyDefinition(t *testing.T) {
	service := new_user_flow_service(t)
	flow, err := service.CreateUserFlow(CreateUserFlowInput{
		Name: "测试流程",
		TriggerType: "Event",
		EventKey: "test.event",
		ContextSchema: []engine.FieldSchema{{Key: "url", Type: "string", Required: true}},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	definition, err := decode_user_flow_definition(flow.Definition)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(definition.Nodes) != 1 {
		t.Fatalf("expected single start node, got %d nodes", len(definition.Nodes))
	}
	start, ok := definition.Nodes["start"]
	if !ok || start.Type != "StartNode" {
		t.Fatalf("expected StartNode at 'start', got %+v", definition.Nodes)
	}
	if definition.StartNodeID != "start" {
		t.Fatalf("unexpected start node id: %s", definition.StartNodeID)
	}
	if definition.Nodes[definition.StartNodeID].Type != "StartNode" {
		t.Fatal("start node must be a StartNode")
	}
	if service.flow_engine.FlowDefinitions[flow.ID].ID != flow.ID {
		t.Fatal("created flow was not registered into the engine")
	}
}

func TestCreateUserFlowRequiresEventKeyForEventTrigger(t *testing.T) {
	service := new_user_flow_service(t)
	if _, err := service.CreateUserFlow(CreateUserFlowInput{Name: "x", TriggerType: "Event"}); err == nil {
		t.Fatal("expected event trigger without event_key to fail")
	}
}

func TestUpdateUserFlowAppendsNodeAndEdge(t *testing.T) {
	service := new_user_flow_service(t)
	flow, err := service.CreateUserFlow(CreateUserFlowInput{Name: "编辑流程"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	updated, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{
		Nodes: []UserFlowNodeInput{
			{ID: "start", Type: "StartNode", Name: "开始", NextIDs: []string{"calc"}},
			{ID: "calc", Type: "ExprNode", Name: "计算", Config: map[string]interface{}{
				"expression": "1 + 1",
			}, NextIDs: []string{"end"}},
			{ID: "end", Type: "EndNode", Name: "结束"},
		},
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	definition, err := decode_user_flow_definition(updated.Definition)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(definition.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(definition.Nodes))
	}
	if len(definition.Nodes["start"].NextNodes) != 1 || definition.Nodes["start"].NextNodes[0].TargetID != "calc" {
		t.Fatalf("start node edge missing: %+v", definition.Nodes["start"])
	}
	if service.flow_engine.FlowDefinitions[flow.ID].Nodes["calc"].ID != "calc" {
		t.Fatal("updated definition was not re-registered into the engine")
	}

	payload, err := service.UserFlowVisualization(flow.ID)
	if err != nil {
		t.Fatalf("visualization failed: %v", err)
	}
	if len(payload.Flows) != 1 || len(payload.Flows[0].Nodes) != 3 {
		t.Fatalf("unexpected visualization payload: %+v", payload)
	}
}

func TestUpdateUserFlowRejectsUnknownTargetAndType(t *testing.T) {
	service := new_user_flow_service(t)
	flow, _ := service.CreateUserFlow(CreateUserFlowInput{Name: "校验流程"})
	if _, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{
		Nodes: []UserFlowNodeInput{
			{ID: "start", Type: "StartNode", Name: "开始", NextIDs: []string{"ghost"}},
		},
	}); err == nil {
		t.Fatal("expected dangling edge to fail")
	}
	if _, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{
		Nodes: []UserFlowNodeInput{
			{ID: "start", Type: "FuncNode", Name: "开始"},
		},
	}); err == nil {
		t.Fatal("expected FuncNode to be rejected")
	}
}

func TestDeleteUserFlowDetachesEngine(t *testing.T) {
	service := new_user_flow_service(t)
	flow, _ := service.CreateUserFlow(CreateUserFlowInput{Name: "删除流程"})
	if err := service.DeleteUserFlow(flow.ID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if _, ok := service.flow_engine.FlowDefinitions[flow.ID]; ok {
		t.Fatal("deleted flow still registered in the engine")
	}
	if _, err := service.GetUserFlow(flow.ID); err == nil {
		t.Fatal("deleted flow still readable")
	}
}
