package iqiyi

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestDashResponseClassifiesPlaybackStatusBeforeProgram(t *testing.T) {
	for _, row := range []struct {
		name   string
		body   string
		kind   string
		status int
	}{
		{"reported region denial", `{"code":"A00000","data":{"tvid":100077400,"st":502,"program":[]}}`, "region", 502},
		{"string region status", `{"code":"A00000","data":{"tvid":"100077400","st":"502","program":[]}}`, "region", 502},
		{"region api code", `{"code":"A00111"}`, "region", 0},
		{"copyright", `{"code":"A00000","data":{"tvid":100077400,"st":401,"program":[]}}`, "copyright", 401},
		{"drm", `{"code":"A00000","data":{"tvid":100077400,"st":512,"program":[]}}`, "drm", 512},
		{"expired login", `{"code":"A00000","data":{"tvid":100077400,"st":800,"program":[]}}`, "authorization", 800},
		{"network", `{"code":"A00000","data":{"tvid":100077400,"st":801,"program":[]}}`, "network", 801},
		{"empty track", `{"code":"A00000","data":{"tvid":100077400,"st":707,"program":[]}}`, "unavailable", 707},
		{"empty program", `{"code":"A00000","data":{"tvid":100077400,"program":[]}}`, "unavailable", 0},
		{"invalid program", `{"code":"A00000","data":{"tvid":100077400,"program":"invalid"}}`, "invalid_response", 0},
		{"wrong episode", `{"code":"A00000","data":{"tvid":999999,"st":101,"program":[]}}`, "identity", 0},
		{"invalid json", `invalid`, "invalid_response", 0},
		{"invalid status", `{"code":"A00000","data":{"tvid":100077400,"st":"bad"}}`, "invalid_response", 0},
		{"invalid api code", `{"code":"https://private.example/?Cookie=secret","msg":"private response"}`, "invalid_response", 0},
	} {
		t.Run(row.name, func(t *testing.T) {
			_, err := read_dash_response(row.body, "100077400")
			var playback_error *PlaybackError
			if !errors.As(err, &playback_error) || playback_error.Kind != row.kind || playback_error.Status != row.status {
				t.Fatalf("wrong playback classification: %+v", err)
			}
			if strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "secret") {
				t.Fatal("error leaked arbitrary upstream data")
			}
			if row.status == 502 && (!strings.Contains(err.Error(), "地区") || strings.Contains(err.Error(), "分集不匹配")) {
				t.Fatalf("region denial mislabeled: %v", err)
			}
		})
	}
}

func TestDashResponseHonorsPreviewAndAuthorization(t *testing.T) {
	for _, field := range []string{"boss", "boss_ts"} {
		for _, row := range []struct {
			name string
			boss any
			paid int
			kind string
		}{
			{"full", map[string]any{"code": "A00000", "data": map[string]any{"prv": 0}}, 1, ""},
			{"preview", map[string]any{"code": "A00000", "data": map[string]any{"prv": 1}}, 1, "preview"},
			{"no entitlement", map[string]any{"code": "Q00505"}, 1, "authorization"},
			{"missing entitlement", nil, 1, "authorization"},
			{"private error", map[string]any{"code": "Cookie=private-token"}, 1, "authorization"},
		} {
			t.Run(field+"/"+row.name, func(t *testing.T) {
				var payload map[string]any
				json.Unmarshal([]byte(iqiyi_dash_body("100077400", "#EXTM3U", 2)), &payload)
				data := payload["data"].(map[string]any)
				data["st"], data[field] = 101, row.boss
				data["content"] = map[string]any{"bossStatus": row.paid}
				body, _ := json.Marshal(payload)
				videos, err := read_dash_response(string(body), "100077400")
				if row.kind == "" {
					if err != nil || len(videos) != 2 {
						t.Fatalf("full playback rejected: %v", err)
					}
					return
				}
				var playback_error *PlaybackError
				if !errors.As(err, &playback_error) || playback_error.Kind != row.kind {
					t.Fatalf("authorization classification: %v", err)
				}
				if strings.Contains(err.Error(), "private-token") {
					t.Fatal("leaked authorization response")
				}
			})
		}
	}
}
