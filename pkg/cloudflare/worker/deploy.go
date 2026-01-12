package worker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
)

// DeployBody 定义 Worker 部署所需的参数
type DeployBody struct {
	AccountID         string
	AuthToken         string
	WorkerName        string
	ScriptContent     []byte
	CompatibilityDate string
	Bindings          []Binding
}

// Metadata 定义 Worker 部署所需的元数据
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

// DeployResult 定义部署结果
type DeployResult struct {
	Success bool  `json:"success"`
	Errors  []any `json:"errors"`
	Result  struct {
		ID string `json:"id"`
	} `json:"result"`
}

// Deploy 执行 Cloudflare Worker 部署
func Deploy(deployBody DeployBody) (string, error) {
	// 构造 Metadata
	metadata := Metadata{
		MainModule:        "index.js",
		CompatibilityDate: deployBody.CompatibilityDate,
		Bindings:          deployBody.Bindings,
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("构造 metadata 失败: %v", err)
	}

	// 构造 Multipart 请求
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Part 1: Metadata
	// 注意: Cloudflare API 要求 metadata 部分必须有 Content-Type: application/json
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="metadata"`)
	h.Set("Content-Type", "application/json")
	part, err := writer.CreatePart(h)
	if err != nil {
		return "", fmt.Errorf("创建 multipart metadata 失败: %v", err)
	}
	part.Write(metadataJSON)

	// Part 2: index.js
	// 注意: 必须标记为 application/javascript+module (ES Module)
	h2 := make(textproto.MIMEHeader)
	h2.Set("Content-Disposition", `form-data; name="index.js"; filename="index.js"`)
	h2.Set("Content-Type", "application/javascript+module")
	part2, err := writer.CreatePart(h2)
	if err != nil {
		return "", fmt.Errorf("创建 multipart script 失败: %v", err)
	}
	part2.Write(deployBody.ScriptContent)

	writer.Close()

	// 发送请求
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

	// 解析响应以确认 success
	var result DeployResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %v, body: %s", err, string(respBody))
	}

	if !result.Success {
		return "", fmt.Errorf("部署失败 (API Error): %s", string(respBody))
	}

	// 部署成功后，确保 workers.dev 子域名已启用
	if err := enableSubdomain(deployBody.AccountID, deployBody.AuthToken, deployBody.WorkerName); err != nil {
		// fmt.Printf("警告: 启用 workers.dev 子域名失败: %v\n", err)
	}

	return result.Result.ID, nil
}

// enableSubdomain 确保 Worker 的 workers.dev 子域名路由已启用
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
