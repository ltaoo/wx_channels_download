package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestNormalizeChannelsProfileAgentURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "host only defaults to wss and profile path",
			raw:  "example.com",
			want: "wss://example.com/ws/channels/profile-agent",
		},
		{
			name: "https becomes wss",
			raw:  "https://example.com",
			want: "wss://example.com/ws/channels/profile-agent",
		},
		{
			name: "http becomes ws and keeps custom path",
			raw:  "http://example.com/custom-agent?node=mac",
			want: "ws://example.com/custom-agent?node=mac",
		},
		{
			name: "ws root path becomes profile path",
			raw:  "ws://example.com/",
			want: "ws://example.com/ws/channels/profile-agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalize_channels_profile_agent_url(tt.raw)
			if err != nil {
				t.Fatalf("normalize URL: %v", err)
			}
			if got != tt.want {
				t.Fatalf("URL = %q want %q", got, tt.want)
			}
		})
	}
}

func TestProfileAgentRunnerHandleRequestFetchesLocalProfile(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/channels/feed/profile" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("oid") != "oid-1" || r.URL.Query().Get("nid") != "nid-1" {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		write_apiok(w, json.RawMessage(`{
			"errCode": 0,
			"errMsg": "",
			"data": {
				"object": {
					"id": "oid-1",
					"objectNonceId": "nid-1"
				}
			}
		}`))
	}))
	defer local.Close()

	runner := &channels_profile_agent_runner{
		local_base_url: local.URL,
		http_client:    local.Client(),
	}
	resp := runner.handle_request(channels_profile_agent_request{
		ID:   "req-1",
		Type: channels_profile_agent_fetch_profile,
		OID:  "oid-1",
		UID:  "nid-1",
	})
	if !resp.OK {
		t.Fatalf("agent response error: %s", resp.Error)
	}
	var profile channels_feed_profile
	if err := json.Unmarshal(resp.Data, &profile); err != nil {
		t.Fatalf("parse agent data: %v", err)
	}
	if string(profile.Data.Object) == "" {
		t.Fatalf("agent returned empty profile object")
	}
}

func TestHandleChannelsFeedProfileForceAgentRequiresConnectedClient(t *testing.T) {
	target_called := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target_called = true
		write_apiok(w, json.RawMessage(valid_channels_profile_json()))
	}))
	defer target.Close()

	data_dir := t.TempDir()
	server := &migration_server{
		default_data_dir: data_dir,
		target_base_url:  target.URL,
		profile_agent:    new_channels_profile_agent_hub("abc"),
	}
	req := httptest.NewRequest(http.MethodGet, "/api/channels/feed/profile?oid=oid-1&uid=nid-1&force_agent=1&data_dir="+url.QueryEscape(data_dir), nil)
	rr := httptest.NewRecorder()
	server.handle_channels_feed_profile(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if target_called {
		t.Fatalf("target HTTP fallback was called despite force_agent=1")
	}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp.Code != http.StatusBadGateway || !strings.Contains(resp.Msg, "profile agent is unavailable") {
		t.Fatalf("response = %+v want code=%d and profile agent unavailable", resp, http.StatusBadGateway)
	}
}

func TestHandleChannelsFeedProfileReportsHTTPSourceWhenNotForced(t *testing.T) {
	target_called := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target_called = true
		if r.URL.Query().Get("oid") != "oid-1" || r.URL.Query().Get("nid") != "nid-1" {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		write_apiok(w, json.RawMessage(valid_channels_profile_json()))
	}))
	defer target.Close()

	data_dir := t.TempDir()
	server := &migration_server{
		default_data_dir: data_dir,
		target_base_url:  target.URL,
		profile_agent:    new_channels_profile_agent_hub("abc"),
	}
	req := httptest.NewRequest(http.MethodGet, "/api/channels/feed/profile?oid=oid-1&uid=nid-1&data_dir="+url.QueryEscape(data_dir), nil)
	rr := httptest.NewRecorder()
	server.handle_channels_feed_profile(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !target_called {
		t.Fatalf("target HTTP fallback was not called")
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Source string `json:"source"`
			Cached bool   `json:"cached"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp.Code != 0 || resp.Data.Source != "http" || resp.Data.Cached {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestHandleChannelsProfileAgentStatusReportsAuthRequired(t *testing.T) {
	server := &migration_server{
		profile_agent: new_channels_profile_agent_hub("abc"),
	}
	req := httptest.NewRequest(http.MethodGet, "/api/channels/profile-agent/status", nil)
	rr := httptest.NewRecorder()
	server.handle_channels_profile_agent_status(rr, req)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Connected    bool `json:"connected"`
			Clients      int  `json:"clients"`
			AuthRequired bool `json:"auth_required"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp.Code != 0 || resp.Data.Connected || resp.Data.Clients != 0 || !resp.Data.AuthRequired {
		t.Fatalf("unexpected status: %+v", resp)
	}
}

func valid_channels_profile_json() string {
	return `{
		"errCode": 0,
		"errMsg": "",
		"data": {
			"object": {
				"id": "oid-1",
				"objectNonceId": "nid-1"
			}
		}
	}`
}
