package pages

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestDeployProjectCreatesConfiguredPagesDeployment(t *testing.T) {
	temp_dir := t.TempDir()
	write_test_file(t, filepath.Join(temp_dir, "index.html"), "<h1>Hub</h1>")
	write_test_file(t, filepath.Join(temp_dir, "assets", "app.js"), "console.log('hub')")
	worker_source := "export default { fetch() { return new Response('ok') } }"
	write_test_file(t, filepath.Join(temp_dir, "_worker.js"), worker_source)

	var request_mu sync.Mutex
	request_counts := make(map[string]int)
	var project_payload map[string]any
	var deployment_manifest map[string]string
	var uploaded_file_count int
	var worker_metadata map[string]any
	var uploaded_worker string
	var uploaded_routes string

	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		request_mu.Lock()
		request_counts[request.Method+" "+request.URL.Path]++
		request_mu.Unlock()

		switch request.Method + " " + request.URL.Path {
		case "GET /accounts/account/pages/projects/hub-admin":
			assert_authorization(t, request, "api-token")
			write_api_response(response_writer, http.StatusNotFound, false, nil)
		case "POST /accounts/account/pages/projects":
			assert_authorization(t, request, "api-token")
			decode_json_body(t, request, &project_payload)
			write_api_response(response_writer, http.StatusOK, true, map[string]any{
				"id":                "project-id",
				"name":              "hub-admin",
				"subdomain":         "hub-admin.pages.dev",
				"production_branch": "main",
			})
		case "GET /accounts/account/pages/projects/hub-admin/upload-token":
			assert_authorization(t, request, "api-token")
			write_api_response(response_writer, http.StatusOK, true, map[string]any{"jwt": "upload-jwt"})
		case "POST /pages/assets/check-missing":
			assert_authorization(t, request, "upload-jwt")
			var payload struct {
				Hashes []string `json:"hashes"`
			}
			decode_json_body(t, request, &payload)
			write_api_response(response_writer, http.StatusOK, true, payload.Hashes)
		case "POST /pages/assets/upload":
			assert_authorization(t, request, "upload-jwt")
			var payload []FilePayloadToUpload
			decode_json_body(t, request, &payload)
			request_mu.Lock()
			uploaded_file_count += len(payload)
			request_mu.Unlock()
			write_api_response(response_writer, http.StatusOK, true, map[string]any{})
		case "POST /pages/assets/upsert-hashes":
			assert_authorization(t, request, "upload-jwt")
			write_api_response(response_writer, http.StatusOK, true, map[string]any{})
		case "POST /accounts/account/pages/projects/hub-admin/deployments":
			assert_authorization(t, request, "api-token")
			if err := request.ParseMultipartForm(2 * 1024 * 1024); err != nil {
				t.Fatalf("parse deployment multipart: %v", err)
			}
			if branch := request.FormValue("branch"); branch != "main" {
				t.Fatalf("unexpected deployment branch %q", branch)
			}
			if err := json.Unmarshal([]byte(request.FormValue("manifest")), &deployment_manifest); err != nil {
				t.Fatalf("decode deployment manifest: %v", err)
			}
			bundle_file, bundle_header, err := request.FormFile("_worker.bundle")
			if err != nil {
				t.Fatalf("read worker bundle: %v", err)
			}
			defer bundle_file.Close()
			if content_type := bundle_header.Header.Get("Content-Type"); content_type != "application/octet-stream" {
				t.Fatalf("unexpected worker bundle content type %q", content_type)
			}
			bundle_content, err := io.ReadAll(bundle_file)
			if err != nil {
				t.Fatalf("read worker bundle: %v", err)
			}
			line_end := bytes.Index(bundle_content, []byte("\r\n"))
			if line_end < 3 {
				t.Fatalf("worker bundle does not start with a multipart boundary")
			}
			boundary := strings.TrimPrefix(string(bundle_content[:line_end]), "--")
			bundle_reader := multipart.NewReader(bytes.NewReader(bundle_content), boundary)
			for {
				part, part_err := bundle_reader.NextPart()
				if part_err == io.EOF {
					break
				}
				if part_err != nil {
					t.Fatalf("read worker bundle part: %v", part_err)
				}
				content, read_err := io.ReadAll(part)
				if read_err != nil {
					t.Fatalf("read worker bundle content: %v", read_err)
				}
				switch part.FormName() {
				case "metadata":
					if err := json.Unmarshal(content, &worker_metadata); err != nil {
						t.Fatalf("decode worker metadata: %v", err)
					}
				case "_worker.js":
					uploaded_worker = string(content)
				}
			}
			routes_file, _, err := request.FormFile("_routes.json")
			if err != nil {
				t.Fatalf("read routes file: %v", err)
			}
			defer routes_file.Close()
			routes, err := io.ReadAll(routes_file)
			if err != nil {
				t.Fatalf("read routes content: %v", err)
			}
			uploaded_routes = string(routes)
			write_api_response(response_writer, http.StatusOK, true, map[string]any{
				"id":          "deployment-id",
				"url":         "https://deployment.hub-admin.pages.dev",
				"environment": "production",
			})
		default:
			http.Error(response_writer, "unexpected endpoint", http.StatusNotFound)
		}
	}))
	defer server.Close()

	api_client := NewClient(ClientOptions{BaseURL: server.URL, HTTPClient: server.Client()})
	result, err := api_client.DeployProject(context.Background(), DeployOptions{
		AccountID:         "account",
		AuthToken:         "api-token",
		ProjectName:       "hub-admin",
		ProductionBranch:  "main",
		CompatibilityDate: "2026-08-19",
		Directory:         temp_dir,
		Secrets:           map[string]string{"HUB_ADMIN_TOKEN": "admin-secret"},
		Services:          map[string]ServiceBinding{"HUB": {Service: "hub-worker"}},
	})
	if err != nil {
		t.Fatalf("DeployProject returned error: %v", err)
	}
	if result.ProjectURL != "https://hub-admin.pages.dev" {
		t.Fatalf("unexpected project URL %q", result.ProjectURL)
	}
	if result.DeploymentID != "deployment-id" || result.Files != 2 {
		t.Fatalf("unexpected deployment result: %+v", result)
	}
	request_mu.Lock()
	actual_uploaded_file_count := uploaded_file_count
	request_mu.Unlock()
	if actual_uploaded_file_count != 2 {
		t.Fatalf("expected 2 uploaded assets, got %d", actual_uploaded_file_count)
	}
	if _, ok := deployment_manifest["/index.html"]; !ok {
		t.Fatalf("manifest missing /index.html: %#v", deployment_manifest)
	}
	if _, ok := deployment_manifest["/assets/app.js"]; !ok {
		t.Fatalf("manifest missing nested asset: %#v", deployment_manifest)
	}
	if uploaded_worker != worker_source {
		t.Fatalf("unexpected worker source %q", uploaded_worker)
	}
	if worker_metadata["main_module"] != "_worker.js" || worker_metadata["compatibility_date"] != "2026-08-19" {
		t.Fatalf("unexpected worker metadata: %#v", worker_metadata)
	}
	bindings := worker_metadata["bindings"].([]any)
	hub_binding := bindings[0].(map[string]any)
	if hub_binding["name"] != "HUB" || hub_binding["type"] != "service" || hub_binding["service"] != "hub-worker" {
		t.Fatalf("unexpected worker service binding: %#v", hub_binding)
	}
	if uploaded_routes != `{"version":1,"include":["/*"],"exclude":[]}` {
		t.Fatalf("unexpected routes: %s", uploaded_routes)
	}
	assert_project_configuration(t, project_payload)
	if request_counts["POST /accounts/account/pages/projects"] != 1 {
		t.Fatalf("project was not created exactly once: %#v", request_counts)
	}
}

func TestEnsureProjectPreservesWranglerConfigHashes(t *testing.T) {
	var patch_payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodGet:
			write_api_response(response_writer, http.StatusOK, true, map[string]any{
				"id": "project-id",
				"deployment_configs": map[string]any{
					"production": map[string]any{"wrangler_config_hash": "production-hash"},
					"preview":    map[string]any{"wrangler_config_hash": "preview-hash"},
				},
			})
		case http.MethodPatch:
			decode_json_body(t, request, &patch_payload)
			write_api_response(response_writer, http.StatusOK, true, map[string]any{"id": "project-id"})
		default:
			http.Error(response_writer, "unexpected method", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()
	api_client := NewClient(ClientOptions{BaseURL: server.URL, HTTPClient: server.Client()})
	if _, err := api_client.EnsureProject(context.Background(), EnsureProjectOptions{
		AccountID:        "account",
		AuthToken:        "api-token",
		ProjectName:      "hub-admin",
		ProductionBranch: "main",
	}); err != nil {
		t.Fatalf("EnsureProject returned error: %v", err)
	}
	configs := patch_payload["deployment_configs"].(map[string]any)
	production := configs["production"].(map[string]any)
	preview := configs["preview"].(map[string]any)
	if production["wrangler_config_hash"] != "production-hash" {
		t.Fatalf("production config hash was not preserved: %#v", production)
	}
	if preview["wrangler_config_hash"] != "preview-hash" {
		t.Fatalf("preview config hash was not preserved: %#v", preview)
	}
}

func assert_project_configuration(t *testing.T, payload map[string]any) {
	t.Helper()
	configs := payload["deployment_configs"].(map[string]any)
	for _, environment_name := range []string{"production", "preview"} {
		environment := configs[environment_name].(map[string]any)
		variables := environment["env_vars"].(map[string]any)
		admin_variable := variables["HUB_ADMIN_TOKEN"].(map[string]any)
		if admin_variable["type"] != "secret_text" || admin_variable["value"] != "admin-secret" {
			t.Fatalf("unexpected %s secret: %#v", environment_name, admin_variable)
		}
		services := environment["services"].(map[string]any)
		hub_service := services["HUB"].(map[string]any)
		if hub_service["service"] != "hub-worker" {
			t.Fatalf("unexpected %s service: %#v", environment_name, hub_service)
		}
	}
}

func assert_authorization(t *testing.T, request *http.Request, expected_token string) {
	t.Helper()
	if actual := request.Header.Get("Authorization"); actual != "Bearer "+expected_token {
		t.Fatalf("unexpected authorization header %q", actual)
	}
}

func decode_json_body(t *testing.T, request *http.Request, value any) {
	t.Helper()
	if err := json.NewDecoder(request.Body).Decode(value); err != nil {
		t.Fatalf("decode request JSON: %v", err)
	}
}

func write_api_response(response_writer http.ResponseWriter, status_code int, success bool, result any) {
	response_writer.Header().Set("Content-Type", "application/json")
	response_writer.WriteHeader(status_code)
	_ = json.NewEncoder(response_writer).Encode(map[string]any{
		"success": success,
		"errors":  []any{},
		"result":  result,
	})
}

func write_test_file(t *testing.T, file_path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file_path), 0o755); err != nil {
		t.Fatalf("create test directory: %v", err)
	}
	if err := os.WriteFile(file_path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}

func TestPagesURL(t *testing.T) {
	test_cases := map[string]string{
		"hub.pages.dev":          "https://hub.pages.dev",
		"https://hub.pages.dev/": "https://hub.pages.dev",
		"":                       "",
	}
	for input, expected := range test_cases {
		if actual := pages_url(strings.TrimSpace(input)); actual != expected {
			t.Fatalf("pages_url(%q) = %q, want %q", input, actual, expected)
		}
	}
}
