package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"wx_channel/internal/config"
	"wx_channel/internal/manager"
	"wx_channel/pkg/browsermgr"
)

func TestAdminToolsRoutes(t *testing.T) {
	browserMgr, err := browsermgr.New(browsermgr.Config{WorkDir: t.TempDir()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv := &AdminServer{
		cfg:        &config.Config{RootDir: ".", Mode: "test"},
		controller: fakeAdminController{},
		browserMgr: browserMgr,
	}
	handler := srv.routes()

	statusReq := httptest.NewRequest(http.MethodGet, "/api/v1/daemons/status?name=default", nil)
	statusRec := httptest.NewRecorder()
	handler.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("daemon status code = %d body=%s", statusRec.Code, statusRec.Body.String())
	}
	var status map[string]any
	if err := json.Unmarshal(statusRec.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status["name"] != "api" || status["running"] != true {
		t.Fatalf("daemon status = %#v", status)
	}

	appsReq := httptest.NewRequest(http.MethodGet, "/api/v1/apps?all=true", nil)
	appsRec := httptest.NewRecorder()
	handler.ServeHTTP(appsRec, appsReq)
	if appsRec.Code != http.StatusOK {
		t.Fatalf("apps code = %d body=%s", appsRec.Code, appsRec.Body.String())
	}
	var apps map[string]any
	if err := json.Unmarshal(appsRec.Body.Bytes(), &apps); err != nil {
		t.Fatal(err)
	}
	if _, ok := apps["apps"]; !ok {
		t.Fatalf("apps response missing apps: %#v", apps)
	}

	sandboxesReq := httptest.NewRequest(http.MethodGet, "/api/v1/sandboxes", nil)
	sandboxesRec := httptest.NewRecorder()
	handler.ServeHTTP(sandboxesRec, sandboxesReq)
	if sandboxesRec.Code != http.StatusOK {
		t.Fatalf("sandboxes code = %d body=%s", sandboxesRec.Code, sandboxesRec.Body.String())
	}
	var sandboxesResp struct {
		Code int   `json:"code"`
		Data []any `json:"data"`
	}
	if err := json.Unmarshal(sandboxesRec.Body.Bytes(), &sandboxesResp); err != nil {
		t.Fatal(err)
	}
	if sandboxesResp.Code != 0 || len(sandboxesResp.Data) != 0 {
		t.Fatalf("sandboxes response = %#v", sandboxesResp)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sandboxes", bytes.NewBufferString(`{"kind":"local","alias":"local","cdp_url":"http://127.0.0.1:9222"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("create sandbox code = %d body=%s", createRec.Code, createRec.Body.String())
	}
	var createResp struct {
		Code int `json:"code"`
		Data struct {
			ID       string `json:"id"`
			Alias    string `json:"alias"`
			Kind     string `json:"kind"`
			Status   string `json:"status"`
			Endpoint struct {
				CDPURL string `json:"cdp_url"`
			} `json:"endpoint"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &createResp); err != nil {
		t.Fatal(err)
	}
	if createResp.Code != 0 || createResp.Data.ID == "" || createResp.Data.Endpoint.CDPURL != "http://127.0.0.1:9222" {
		t.Fatalf("create sandbox response = %#v", createResp)
	}
}

type fakeAdminController struct{}

func (fakeAdminController) ListServices() []ServiceSnapshot {
	return []ServiceSnapshot{{Name: "api", Title: "API", Addr: "127.0.0.1:2020", Status: manager.StatusRunning}}
}

func (fakeAdminController) StartService(name string) error { return nil }

func (fakeAdminController) StopService(name string) error { return nil }
