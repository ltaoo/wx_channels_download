package cmd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"wx_channel/frontend"
	"wx_channel/pkg/cloudflare/durableobjects"
	"wx_channel/pkg/cloudflare/worker"
)

const permission_hint = "提示: 请确保在 Cloudflare 后台为 Token 授予了足够的权限 (Workers:Edit, D1:Edit)"

const (
	hub_default_worker_name      = "wx-channels-hub"
	hub_compatibility_date       = "2026-05-03"
	hub_class_name               = "HubDurableObject"
	hub_binding_name             = "HUBS"
	hub_token_binding_name       = "HUB_TOKEN"
	hub_admin_token_binding_name = "HUB_ADMIN_TOKEN"
	hub_main_module              = "hub.js"
)

var deploy_cmd = &cobra.Command{
	Use:   "deploy",
	Short: "部署 Cloudflare Worker",
	Long:  "读取配置文件中的 Cloudflare 配置，通过 Cloudflare REST API 直接部署 Worker",
}

var deploy_mp_cmd = &cobra.Command{
	Use:   "mp",
	Short: "部署公众号 Worker",
	Long:  "部署公众号 RSS/API 相关的 Cloudflare Worker",
	Run: func(cmd *cobra.Command, args []string) {
		deploy_mp()
	},
}

var deploy_sph_cmd = &cobra.Command{
	Use:   "sph",
	Short: "部署视频号查询 Worker",
	Long:  "部署视频号视频信息查询的 Cloudflare Worker",
	Run: func(cmd *cobra.Command, args []string) {
		deploy_sph()
	},
}

var deploy_hub_cmd = &cobra.Command{
	Use:   "hub",
	Short: "部署 Durable Objects 任务 Hub",
	Long:  "通过 Cloudflare REST API 直接部署原生 JavaScript Durable Objects 任务 Hub",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		deploy_hub()
	},
}

func init() {
	deploy_cmd.AddCommand(deploy_mp_cmd, deploy_sph_cmd, deploy_hub_cmd)
	Register(deploy_cmd)
}

func deploy_hub() {
	pterm.DefaultSection.Println("开始部署 Durable Objects 任务 Hub (Go + JavaScript + REST API)")

	account_id := strings.TrimSpace(viper.GetString("cloudflare.accountId"))
	api_token := strings.TrimSpace(viper.GetString("cloudflare.apiToken"))
	worker_name := strings.TrimSpace(viper.GetString("hub.deploy.workerName"))
	hub_token := strings.TrimSpace(viper.GetString("hub.deploy.token"))
	admin_token := strings.TrimSpace(viper.GetString("hub.deploy.adminToken"))
	if worker_name == "" {
		worker_name = hub_default_worker_name
	}
	if account_id == "" || api_token == "" {
		pterm.Error.Println("错误: 未配置 cloudflare.accountId 或 cloudflare.apiToken")
		return
	}
	if hub_token == "" {
		pterm.Error.Println("错误: 未配置 hub.deploy.token；该值会作为 Worker 的 HUB_TOKEN secret")
		return
	}
	if admin_token == "" {
		pterm.Error.Println("错误: 未配置 hub.deploy.adminToken；该值用于保护 Hub 管理页面")
		return
	}
	if admin_token == hub_token {
		pterm.Error.Println("错误: hub.deploy.adminToken 不能与 hub.deploy.token 相同")
		return
	}

	spinner, _ := pterm.DefaultSpinner.Start("正在部署原生 JavaScript Hub...")
	script_content := []byte(frontend.HubWorkerJavaScript())
	deploy_context, cancel_deploy := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel_deploy()
	result, err := durableobjects.Deploy(deploy_context, durableobjects.DeployOptions{
		AccountID:         account_id,
		AuthToken:         api_token,
		WorkerName:        worker_name,
		ScriptContent:     script_content,
		CompatibilityDate: hub_compatibility_date,
		MainModule:        hub_main_module,
		DurableObjects: []durableobjects.DurableObject{
			{
				BindingName: hub_binding_name,
				ClassName:   hub_class_name,
				Storage:     "sqlite",
			},
		},
		Secrets: map[string]string{
			hub_token_binding_name:       hub_token,
			hub_admin_token_binding_name: admin_token,
		},
		EnableSubdomain: true,
	})
	if err != nil {
		spinner.Fail(fmt.Sprintf("部署失败: %v", err))
		pterm.Info.Println("提示: Cloudflare API Token 需要 Workers Scripts:Edit 权限")
		return
	}
	spinner.Success(fmt.Sprintf("Hub 部署成功，已编译 %d 字节 JavaScript", result.ScriptBytes))

	spinner, _ = pterm.DefaultSpinner.Start("正在获取 Worker 访问地址...")
	subdomain, err := get_workers_subdomain(account_id, api_token)
	worker_url := fmt.Sprintf("https://%s.<your-subdomain>.workers.dev", result.WorkerName)
	if err != nil {
		spinner.Warning(fmt.Sprintf("获取子域名失败: %v", err))
	} else {
		worker_url = fmt.Sprintf("https://%s.%s.workers.dev", result.WorkerName, subdomain)
		spinner.Success("获取访问地址成功")
	}

	pterm.Println()
	pterm.DefaultHeader.WithFullWidth().Println("Hub 部署摘要")
	table_data := [][]string{
		{"项目", "值"},
		{"Worker", result.WorkerName},
		{"Worker ID", result.WorkerID},
		{"URL", worker_url},
		{"健康检查", worker_url + "/health"},
		{"管理 API", worker_url + "/admin/api/overview"},
		{"WebSocket", worker_url + "/v1/hubs/<hub.id>/ws"},
	}
	pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(table_data).Render()
	pterm.Println()
	pterm.Info.Println("将上面的 URL 和 hub.deploy.token 写入需要连接该 Worker 的 hub.instances。")
	pterm.Info.Println("管理页面已拆分到 frontend/hub/admin，请按其中 README 使用 Cloudflare Pages 单独部署。")
}

func deploy_mp() {
	pterm.DefaultSection.Println("开始部署 Cloudflare Worker (REST API)")

	// 1. Get configuration
	account_id := viper.GetString("cloudflare.accountId")
	api_token := viper.GetString("cloudflare.apiToken")
	worker_name := viper.GetString("cloudflare.workerName")
	d1_database_id := viper.GetString("cloudflare.d1Id")
	d1_database_name := viper.GetString("cloudflare.d1Name") // New: support lookup/create by name
	admin_token := viper.GetString("cloudflare.adminToken")
	refresh_token := viper.GetString("cloudflare.refreshToken")
	remote_server_hostname := viper.GetString("mp.remoteServer.hostname")

	if api_token == "" || account_id == "" {
		pterm.Error.Println("错误: 未配置 Cloudflare Auth Token 或 Account ID")
		return
	}

	// 1.2 Use Database Name to find or create if configured
	if d1_database_name != "" {
		spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("正在根据名称查找 D1 数据库: '%s' ...", d1_database_name))
		id, err := find_d1_database_by_name(account_id, api_token, d1_database_name)
		if err != nil {
			spinner.Fail(fmt.Sprintf("查找数据库失败: %v", err))
			pterm.Info.Println(permission_hint)
			return
		}
		if id != "" {
			d1_database_id = id
			spinner.Success(fmt.Sprintf("找到现有的 D1 数据库: %s (ID: %s)", d1_database_name, d1_database_id))
		} else {
			spinner.UpdateText(fmt.Sprintf("数据库 '%s' 不存在，正在创建...", d1_database_name))
			new_id, err := create_d1_database(account_id, api_token, d1_database_name)
			if err != nil {
				spinner.Fail(fmt.Sprintf("创建数据库失败: %v", err))
				pterm.Info.Println(permission_hint)
				return
			}
			d1_database_id = new_id
			spinner.Success(fmt.Sprintf("成功创建 D1 数据库: %s (ID: %s)", d1_database_name, d1_database_id))
		}
	}

	if d1_database_id == "" {
		pterm.Error.Println("错误: 未配置 D1 Database ID (cloudflare.d1.databaseId) 且无法通过名称找到或创建")
		return
	}

	// 1.5 Execute database initialization (via API directly)
	spinner, _ := pterm.DefaultSpinner.Start("正在验证 D1 数据库连接...")
	if err := verify_d1_database(account_id, api_token, d1_database_id); err != nil {
		spinner.Fail(fmt.Sprintf("D1 数据库验证失败: %v", err))
		pterm.Info.Println(permission_hint)
		return
	}
	spinner.Success("D1 数据库连接验证成功")

	worker_dir := filepath.Join(Cfg.RootDir, "pkg", "scraper", "wxmp", "worker")

	// 1.6 Execute database migration
	spinner, _ = pterm.DefaultSpinner.Start("正在检查并执行数据库迁移...")

	if err := run_migrations(account_id, api_token, d1_database_id, filepath.Join(worker_dir, "migrations")); err != nil {
		spinner.Warning(fmt.Sprintf("数据库迁移失败: %v", err))
	} else {
		spinner.Success("数据库迁移完成")
	}

	script_path := filepath.Join(worker_dir, "index.js")
	script_content, err := os.ReadFile(script_path)
	if err != nil {
		pterm.Error.Printf("读取 Worker 脚本失败: %v\n", err)
		return
	}

	// 3. Build deployment parameters
	deploy_body := worker.DeployBody{
		AccountID:         account_id,
		AuthToken:         api_token,
		WorkerName:        worker_name,
		ScriptContent:     script_content,
		CompatibilityDate: "2024-01-01",
		Bindings: []worker.Binding{
			{Type: "d1", Name: "DB", ID: d1_database_id},
			{Type: "plain_text", Name: "ADMIN_TOKEN", Text: admin_token},
			{Type: "plain_text", Name: "REFRESH_TOKEN", Text: refresh_token},
			{Type: "plain_text", Name: "REMOTE_SERVER", Text: remote_server_hostname},
		},
	}

	// 4. Execute deployment
	// Truncate Account ID to prevent spinner rendering issues from terminal line wrapping
	short_account_id := account_id
	if len(short_account_id) > 6 {
		short_account_id = short_account_id[:6] + "..."
	}
	spinner, _ = pterm.DefaultSpinner.Start(fmt.Sprintf("正在部署到 Cloudflare (Worker: %s)...", worker_name))
	_, err = worker.Deploy(deploy_body)
	if err != nil {
		spinner.Fail(fmt.Sprintf("部署失败: %v", err))
		pterm.Info.Println(permission_hint)
		return
	}
	spinner.Success("部署成功!")

	// 5. Get subdomain and output access URL
	spinner, _ = pterm.DefaultSpinner.Start("正在获取 Worker 访问地址...")
	subdomain, err := get_workers_subdomain(account_id, api_token)
	worker_url := ""
	if err != nil {
		spinner.Warning(fmt.Sprintf("获取子域名失败: %v", err))
		worker_url = fmt.Sprintf("https://%s.<your-subdomain>.workers.dev", worker_name)
	} else {
		worker_url = fmt.Sprintf("https://%s.%s.workers.dev", worker_name, subdomain)
		spinner.Success("获取访问地址成功")
	}

	pterm.Println()
	pterm.DefaultHeader.WithFullWidth().Println("部署摘要")

	panels := pterm.Panels{
		{{Data: pterm.DefaultBox.WithTitle("Worker Info").Sprint(
			pterm.Sprintf("%s: %s\n%s: %s",
				pterm.Bold.Sprint("Worker Name"), pterm.Cyan(worker_name),
				// pterm.Bold.Sprint("Worker ID"), pterm.Cyan(worker_id),
				pterm.Bold.Sprint("URL"), pterm.LightGreen(worker_url),
			),
		)}},
	}
	pterm.DefaultPanel.WithPanels(panels).Render()

	pterm.Println()
	pterm.DefaultHeader.WithFullWidth().Println("可用 API 列表")

	table_data := [][]string{
		{"Method", "Path", "Description"},
		{"GET", "/api/mp/list", "获取公众号列表"},
		{"GET", "/api/mp/msg/list", "获取公众号消息列表"},
		{"POST", "/api/mp/refresh", "刷新/同步公众号信息 (需要 Refresh Token)"},
		{"POST", "/admin/token/add", "添加访问 Token (需要 Admin Token)"},
		{"POST", "/admin/token/delete", "删除访问 Token (需要 Admin Token)"},
		{"GET", "/rss/mp", "RSS 订阅地址 (参数: biz)"},
	}

	pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(table_data).Render()

	pterm.Println()
	pterm.DefaultSection.WithStyle(pterm.NewStyle(pterm.FgGreen)).Println("✅ 部署成功! 请访问上面的 URL 使用服务")
}

func deploy_sph() {
	pterm.DefaultSection.Println("开始部署 视频号查询 Worker (REST API)")

	account_id := viper.GetString("cloudflare.accountId")
	api_token := viper.GetString("cloudflare.apiToken")
	worker_name := viper.GetString("cloudflare.sphWorkerName")
	sph_cookie := viper.GetString("cloudflare.sphCookie")
	sph_credential := viper.GetString("cloudflare.sphCredential")

	if api_token == "" || account_id == "" {
		pterm.Error.Println("错误: 未配置 Cloudflare Auth Token 或 Account ID")
		return
	}

	if worker_name == "" {
		pterm.Error.Println("错误: 未配置 cloudflare.sphWorkerName")
		return
	}

	if strings.TrimSpace(sph_credential) == "" {
		pterm.Error.Println("错误: 未配置 cloudflare.sphCredential")
		return
	}

	sph_dir := filepath.Join(Cfg.RootDir, "pkg", "scraper", "wxchannels", "worker")

	// Read worker.js
	script_path := filepath.Join(sph_dir, "worker.js")
	script_content, err := os.ReadFile(script_path)
	if err != nil {
		pterm.Error.Printf("读取 worker.js 失败: %v\n", err)
		return
	}

	// Read index.html
	html_path := filepath.Join(sph_dir, "index.html")
	html_content, err := os.ReadFile(html_path)
	if err != nil {
		pterm.Error.Printf("读取 index.html 失败: %v\n", err)
		return
	}

	// Read icon.png and convert to base64, deploy as JS module
	icon_path := filepath.Join(Cfg.RootDir, "build", "icon.png")
	icon_bytes, err := os.ReadFile(icon_path)
	if err != nil {
		pterm.Error.Printf("读取 icon.png 失败: %v\n", err)
		return
	}
	icon_base64 := base64.StdEncoding.EncodeToString(icon_bytes)

	// Build deployment parameters (sph worker does not need D1 or extra bindings)
	deploy_body := worker.DeployBody{
		AccountID:         account_id,
		AuthToken:         api_token,
		WorkerName:        worker_name,
		ScriptContent:     script_content,
		CompatibilityDate: "2024-01-01",
		MainModule:        "worker.js",
		Bindings: []worker.Binding{
			{Type: "plain_text", Name: "COOKIE", Text: sph_cookie},
			{Type: "plain_text", Name: "ACCESS_CREDENTIAL", Text: sph_credential},
		},
		AdditionalFiles: map[string][]byte{
			"index.html": html_content,
			"icon.js":    []byte(fmt.Sprintf(`export default "%s";`, icon_base64)),
		},
	}

	short_account_id := account_id
	if len(short_account_id) > 6 {
		short_account_id = short_account_id[:6] + "..."
	}

	spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("正在部署到 Cloudflare (Worker: %s)...", worker_name))
	_, err = worker.Deploy(deploy_body)
	if err != nil {
		spinner.Fail(fmt.Sprintf("部署失败: %v", err))
		return
	}
	spinner.Success("部署成功!")

	// Get subdomain and output access URL
	spinner, _ = pterm.DefaultSpinner.Start("正在获取 Worker 访问地址...")
	subdomain, err := get_workers_subdomain(account_id, api_token)
	worker_url := ""
	if err != nil {
		spinner.Warning(fmt.Sprintf("获取子域名失败: %v", err))
		worker_url = fmt.Sprintf("https://%s.<your-subdomain>.workers.dev", worker_name)
	} else {
		worker_url = fmt.Sprintf("https://%s.%s.workers.dev", worker_name, subdomain)
		spinner.Success("获取访问地址成功")
	}

	pterm.Println()
	pterm.DefaultHeader.WithFullWidth().Println("部署摘要")

	panels := pterm.Panels{
		{{Data: pterm.DefaultBox.WithTitle("Worker Info").Sprint(
			pterm.Sprintf("%s: %s\n%s: %s",
				pterm.Bold.Sprint("Worker Name"), pterm.Cyan(worker_name),
				pterm.Bold.Sprint("URL"), pterm.LightGreen(worker_url),
			),
		)}},
	}
	pterm.DefaultPanel.WithPanels(panels).Render()

	pterm.Println()
	pterm.DefaultHeader.WithFullWidth().Println("可用 API")

	table_data := [][]string{
		{"Method", "Path", "Description"},
		{"GET", "/", "视频号视频信息查询页面"},
		{"POST", "/api/fetch_video_profile", "获取视频号视频信息"},
	}

	pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(table_data).Render()

	pterm.Println()
	pterm.DefaultSection.WithStyle(pterm.NewStyle(pterm.FgGreen)).Println("部署成功! 请访问上面的 URL 使用服务")
}

func verify_d1_database(account_id, auth_token, database_id string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/d1/database/%s", account_id, database_id)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("create request failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+auth_token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func run_migrations(account_id, auth_token, database_id, migrations_dir string) error {
	// 1. Ensure migrations table exists
	_, err := query_d1(account_id, auth_token, database_id, `CREATE TABLE IF NOT EXISTS d1_migrations (
		id INTEGER PRIMARY KEY,
		name TEXT,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`, nil)
	if err != nil {
		return fmt.Errorf("failed to ensure migrations table: %v", err)
	}

	// 2. Get applied migrations
	resp, err := query_d1(account_id, auth_token, database_id, "SELECT id FROM d1_migrations", nil)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %v", err)
	}

	applied := make(map[int]bool)
	if len(resp.Result) > 0 && len(resp.Result[0].Results) > 0 {
		for _, row := range resp.Result[0].Results {
			if id, ok := row["id"].(float64); ok {
				applied[int(id)] = true
			}
		}
	}

	// 3. Read migration files
	files, err := os.ReadDir(migrations_dir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %v", err)
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".sql" {
			continue
		}

		// Simple parsing: name should start with ID (e.g., 0001_init.sql)
		var id int
		if _, err := fmt.Sscanf(file.Name(), "%d_", &id); err != nil {
			// fmt.Printf("Skipping invalid migration file: %s\n", file.Name())
			continue
		}

		if applied[id] {
			continue
		}

		// fmt.Printf("Applying migration: %s\n", file.Name())
		content, err := os.ReadFile(filepath.Join(migrations_dir, file.Name()))
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %v", file.Name(), err)
		}

		// Execute migration and record it in a single batch (transaction)
		// We append the INSERT statement to ensure atomicity.
		full_sql := string(content) + fmt.Sprintf("\nINSERT INTO d1_migrations (id, name) VALUES (%d, '%s');", id, file.Name())

		if _, err := query_d1(account_id, auth_token, database_id, full_sql, nil); err != nil {
			return fmt.Errorf("failed to execute migration %s: %v", file.Name(), err)
		}
		// fmt.Printf("Migration %s applied successfully\n", file.Name())
	}

	return nil
}

// Helper structs for D1 API response
type D1Response struct {
	Result []struct {
		Meta struct {
			ChangedDB bool    `json:"changed_db"`
			Changes   int     `json:"changes"`
			Duration  float64 `json:"duration"`
		} `json:"meta"`
		Results []map[string]any `json:"results"`
		Success bool             `json:"success"`
	} `json:"result"`
	Success bool `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

func query_d1(account_id, auth_token, database_id, sql_str string, params []any) (*D1Response, error) {
	req_body := map[string]any{
		"sql":    sql_str,
		"params": params,
	}
	if req_body["params"] == nil {
		req_body["params"] = []any{}
	}

	json_body, err := json.Marshal(req_body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/d1/database/%s/query", account_id, database_id)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(json_body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+auth_token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		var error_resp D1Response
		if json_err := json.Unmarshal(body, &error_resp); json_err == nil && len(error_resp.Errors) > 0 {
			var sb strings.Builder
			for _, e := range error_resp.Errors {
				sb.WriteString(fmt.Sprintf("[%d] %s; ", e.Code, e.Message))
				if e.Code == 7500 {
					sb.WriteString(" (提示: Token 缺少 'D1:Edit' 权限，请在 Cloudflare 后台为 Token 添加 Account->Workers D1->Edit 权限)")
				}
				if strings.Contains(e.Message, "SQLITE_AUTH") {
					sb.WriteString(fmt.Sprintf(" (Hint: Check if Token has 'D1:Edit' permission, and AccountID '%s' matches DatabaseID '%s'. Also ensure you are using a Token, not Global Key)", account_id, database_id))
				}
			}
			return nil, fmt.Errorf("D1 API error (Status %d): %s", resp.StatusCode, sb.String())
		}
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var d1_resp D1Response
	if err := json.Unmarshal(body, &d1_resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v, body: %s", err, string(body))
	}

	if !d1_resp.Success {
		var sb strings.Builder
		for _, e := range d1_resp.Errors {
			sb.WriteString(fmt.Sprintf("[%d] %s; ", e.Code, e.Message))
			if strings.Contains(e.Message, "SQLITE_AUTH") {
				sb.WriteString(fmt.Sprintf(" (Hint: Check if Token has 'D1:Edit' permission, and AccountID '%s' matches DatabaseID '%s')", account_id, database_id))
			}
		}
		return nil, fmt.Errorf("D1 API error: %s", sb.String())
	}

	return &d1_resp, nil
}

// Helper to list/find D1 database by name
func find_d1_database_by_name(account_id, auth_token, name string) (string, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/d1/database", account_id)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create request failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+auth_token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var list_resp struct {
		Result []struct {
			UUID string `json:"uuid"`
			Name string `json:"name"`
		} `json:"result"`
		Success bool `json:"success"`
	}

	if err := json.Unmarshal(body, &list_resp); err != nil {
		return "", fmt.Errorf("unmarshal failed: %v", err)
	}

	if !list_resp.Success {
		return "", fmt.Errorf("api returned success=false")
	}

	for _, db := range list_resp.Result {
		if db.Name == name {
			return db.UUID, nil
		}
	}

	return "", nil
}

// Helper to create D1 database
func create_d1_database(account_id, auth_token, name string) (string, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/d1/database", account_id)
	req_body := map[string]string{"name": name}
	json_body, _ := json.Marshal(req_body)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(json_body))
	if err != nil {
		return "", fmt.Errorf("create request failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+auth_token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var create_resp struct {
		Result struct {
			UUID string `json:"uuid"`
			Name string `json:"name"`
		} `json:"result"`
		Success bool `json:"success"`
	}

	if err := json.Unmarshal(body, &create_resp); err != nil {
		return "", fmt.Errorf("unmarshal failed: %v", err)
	}

	if !create_resp.Success {
		return "", fmt.Errorf("api returned success=false")
	}

	return create_resp.Result.UUID, nil
}

func get_workers_subdomain(account_id, auth_token string) (string, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/workers/subdomain", account_id)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create request failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+auth_token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var subdomain_resp struct {
		Result struct {
			Subdomain string `json:"subdomain"`
		} `json:"result"`
		Success bool `json:"success"`
	}

	if err := json.Unmarshal(body, &subdomain_resp); err != nil {
		return "", fmt.Errorf("unmarshal failed: %v", err)
	}

	if !subdomain_resp.Success {
		return "", fmt.Errorf("api returned success=false")
	}

	return subdomain_resp.Result.Subdomain, nil
}
