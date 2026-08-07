package worker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strings"
)

// DeployBody defines the parameters required for Worker deployment.
type DeployBody struct {
	AccountID         string
	AuthToken         string
	WorkerName        string
	ScriptContent     []byte
	CompatibilityDate string
	Bindings          []Binding
	MainModule        string            // ES module entry point, defaults to "index.js"
	AdditionalFiles   map[string][]byte // extra files to include (e.g. index.html)
}

// Metadata defines the metadata required for Worker deployment.
type Metadata struct {
	MainModule        string    `json:"main_module"`
	CompatibilityDate string    `json:"compatibility_date"`
	Bindings          []Binding `json:"bindings"`
}

type Binding struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	NamespaceID string `json:"namespace_id,omitempty"` // For kv_namespace
	Text        string `json:"text,omitempty"`         // For plain_text
	ID          string `json:"id,omitempty"`           // For d1
}

// DeployResult defines the deployment result.
type DeployResult struct {
	Success bool  `json:"success"`
	Errors  []any `json:"errors"`
	Result  struct {
		ID string `json:"id"`
	} `json:"result"`
}

func detectContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".html", ".htm":
		return "application/octet-stream"
	case ".js", ".mjs":
		return "application/javascript+module"
	case ".css":
		return "text/css"
	case ".json":
		return "application/json"
	case ".txt":
		return "text/plain"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".wasm":
		return "application/wasm"
	default:
		return "application/octet-stream"
	}
}

// Deploy executes a Cloudflare Worker deployment.
func Deploy(deployBody DeployBody) (string, error) {
	mainModule := deployBody.MainModule
	if mainModule == "" {
		mainModule = "index.js"
	}

	// Build Metadata
	metadata := Metadata{
		MainModule:        mainModule,
		CompatibilityDate: deployBody.CompatibilityDate,
		Bindings:          deployBody.Bindings,
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("构造 metadata 失败: %v", err)
	}

	// Build multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Part 1: Metadata
	// NOTE: Cloudflare API requires the metadata part to have Content-Type: application/json
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="metadata"`)
	h.Set("Content-Type", "application/json")
	part, err := writer.CreatePart(h)
	if err != nil {
		return "", fmt.Errorf("创建 multipart metadata 失败: %v", err)
	}
	part.Write(metadataJSON)

	// Part 2: main module (ES Module)
	h2 := make(textproto.MIMEHeader)
	h2.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, mainModule, mainModule))
	h2.Set("Content-Type", "application/javascript+module")
	part2, err := writer.CreatePart(h2)
	if err != nil {
		return "", fmt.Errorf("创建 multipart script 失败: %v", err)
	}
	part2.Write(deployBody.ScriptContent)

	// Part 3+: additional files
	for filename, content := range deployBody.AdditionalFiles {
		hf := make(textproto.MIMEHeader)
		hf.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, filename, filename))
		hf.Set("Content-Type", detectContentType(filename))
		pf, err := writer.CreatePart(hf)
		if err != nil {
			return "", fmt.Errorf("创建 multipart file %s 失败: %v", filename, err)
		}
		pf.Write(content)
	}

	writer.Close()

	// Send request
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/workers/scripts/%s", deployBody.AccountID, deployBody.WorkerName)
	req, err := http.NewRequest("PUT", url, body)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+deployBody.AuthToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("部署失败 (Status: %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response to confirm success
	var result DeployResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %v, body: %s", err, string(respBody))
	}

	if !result.Success {
		return "", fmt.Errorf("部署失败 (API Error): %s", string(respBody))
	}

	// After successful deployment, ensure the workers.dev subdomain is enabled
	if err := enableSubdomain(deployBody.AccountID, deployBody.AuthToken, deployBody.WorkerName); err != nil {
		// fmt.Printf("Warning: failed to enable workers.dev subdomain: %v\n", err)
	}

	return result.Result.ID, nil
}

// enableSubdomain ensures the Worker's workers.dev subdomain route is enabled
// PUT /accounts/{account_id}/workers/scripts/{script_name}/subdomain
func enableSubdomain(accountID, authToken, workerName string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/workers/scripts/%s/subdomain", accountID, workerName)

	// Request body: {"enabled": true}
	reqBody := []byte(`{"enabled": true}`)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+authToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
