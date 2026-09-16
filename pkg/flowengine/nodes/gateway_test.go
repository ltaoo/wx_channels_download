package nodes

import (
	"reflect"
	"testing"

	"wx_channel/pkg/flowengine/engine"
)

func TestGatewayNodeRoutesJSONDecodedRules(t *testing.T) {
	// JSON-decoded configs produce rules as []interface{} of
	// map[string]interface{}; the node must not panic and must route.
	node := &GatewayNode{
		Id: "check_live",
		Config: map[string]interface{}{
			"gateway_type": "Exclusive",
			"is_joining":   false,
			"rules": []interface{}{
				map[string]interface{}{
					"condition": "len(videos.data.object) > 0 && videos.data.object[0].liveInfo != nil",
					"target_id": "download_live",
				},
				map[string]interface{}{
					"condition": "true",
					"target_id": "end",
				},
			},
		},
	}
	ctx := &engine.ProcessContext{
		Data: map[string]interface{}{
			"videos": map[string]interface{}{
				"data": map[string]interface{}{
					"object": []interface{}{
						map[string]interface{}{
							"id":       "1",
							"liveInfo": map[string]interface{}{"liveId": "42"},
						},
					},
				},
			},
		},
	}
	ok, next, err := node.Execute(ctx)
	if err != nil {
		t.Fatalf("execute gateway: %v", err)
	}
	if !ok {
		t.Fatal("expected gateway to route successfully")
	}
	if !reflect.DeepEqual(next, []string{"download_live"}) {
		t.Fatalf("unexpected next nodes: %#v", next)
	}
}

func TestGatewayNodeFallsBackToElseRule(t *testing.T) {
	node := &GatewayNode{
		Id: "check_live",
		Config: map[string]interface{}{
			"gateway_type": "Exclusive",
			"is_joining":   false,
			"rules": []map[string]interface{}{
				{"condition": "len(videos.data.object) > 0 && videos.data.object[0].liveInfo != nil", "target_id": "download_live"},
				{"condition": "true", "target_id": "end"},
			},
		},
	}
	ctx := &engine.ProcessContext{
		Data: map[string]interface{}{
			"videos": map[string]interface{}{
				"data": map[string]interface{}{
					"object": []interface{}{
						map[string]interface{}{"id": "1"},
					},
				},
			},
		},
	}
	_, next, err := node.Execute(ctx)
	if err != nil {
		t.Fatalf("execute gateway: %v", err)
	}
	if !reflect.DeepEqual(next, []string{"end"}) {
		t.Fatalf("unexpected next nodes: %#v", next)
	}
}
