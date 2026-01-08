package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"wx_channel/pkg/cloudflare/worker"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "部署 Cloudflare Worker",
	Long:  "读取配置文件中的 Cloudflare 配置，通过 Cloudflare REST API 直接部署 Worker",
	Run: func(cmd *cobra.Command, args []string) {
		deploy()
	},
}

func init() {
	Register(deployCmd)
}

func deploy() {
	fmt.Println(color.GreenString("开始部署 Cloudflare Worker (REST API)..."))

	// 1. 获取配置
	accountID := viper.GetString("cloudflare.accountId")
	authToken := viper.GetString("cloudflare.authToken")
	workerName := viper.GetString("cloudflare.worker_name")
	d1DatabaseID := viper.GetString("cloudflare.d1.database_id")
	refreshToken := viper.GetString("cloudflare.refreshToken")
	remoteServer := viper.GetString("mp.remoteServer.hostname")

	if authToken == "" || accountID == "" {
		fmt.Println(color.RedString("错误: 未配置 Cloudflare Auth Token 或 Account ID"))
		return
	}
	if d1DatabaseID == "" {
		fmt.Println(color.RedString("错误: 未配置 D1 Database ID (cloudflare.d1.database_id)"))
		return
	}

	// 1.5 执行数据库初始化 (直接调用 API)
	fmt.Println(color.GreenString("正在验证 D1 数据库连接..."))
	if err := verifyD1Database(accountID, authToken, d1DatabaseID); err != nil {
		fmt.Println(color.RedString("D1 数据库验证失败: %v", err))
		return
	}

	workerDir := filepath.Join(Cfg.RootDir, "internal", "officialaccount", "worker")

	// 1.6 执行数据库迁移
	fmt.Println(color.GreenString("正在检查并执行数据库迁移..."))

	if err := runMigrations(accountID, authToken, d1DatabaseID, filepath.Join(workerDir, "migrations")); err != nil {
		fmt.Println(color.RedString("数据库迁移失败: %v", err))
	}
	scriptPath := filepath.Join(workerDir, "index.js")
	scriptContent, err := os.ReadFile(scriptPath)
	if err != nil {
		fmt.Println(color.RedString("读取 Worker 脚本失败: %v", err))
		return
	}

	// 3. 构造部署参数
	deployBody := worker.DeployBody{
		AccountID:         accountID,
		AuthToken:         authToken,
		WorkerName:        workerName,
		ScriptContent:     scriptContent,
		CompatibilityDate: "2024-01-01",
		Bindings: []worker.Binding{
			{Type: "d1", Name: "DB", ID: d1DatabaseID},
			{Type: "plain_text", Name: "AUTH_TOKEN", Text: authToken},
			{Type: "plain_text", Name: "REFRESH_TOKEN", Text: refreshToken},
			{Type: "plain_text", Name: "REMOTE_SERVER", Text: remoteServer},
		},
	}

	// 4. 执行部署
	fmt.Printf("正在部署到 Cloudflare (Account: %s, Worker: %s)...\n", accountID, workerName)
	workerID, err := worker.Deploy(deployBody)
	if err != nil {
		fmt.Println(color.RedString("部署失败: %v", err))
		return
	}

	fmt.Println(color.GreenString("部署成功!"))
	fmt.Printf("Worker ID: %s\n", workerID)
}

func verifyD1Database(accountID, authToken, databaseID string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/d1/database/%s", accountID, databaseID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("create request failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+authToken)
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

func runMigrations(accountID, authToken, databaseID, migrationsDir string) error {
	// 1. Ensure migrations table exists
	_, err := queryD1(accountID, authToken, databaseID, `CREATE TABLE IF NOT EXISTS d1_migrations (
		id INTEGER PRIMARY KEY,
		name TEXT,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`, nil)
	if err != nil {
		return fmt.Errorf("failed to ensure migrations table: %v", err)
	}

	// 2. Get applied migrations
	resp, err := queryD1(accountID, authToken, databaseID, "SELECT id FROM d1_migrations", nil)
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
	files, err := os.ReadDir(migrationsDir)
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
			fmt.Printf("Skipping invalid migration file: %s\n", file.Name())
			continue
		}

		if applied[id] {
			continue
		}

		fmt.Printf("Applying migration: %s\n", file.Name())
		content, err := os.ReadFile(filepath.Join(migrationsDir, file.Name()))
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %v", file.Name(), err)
		}

		// Execute migration and record it in a single batch (transaction)
		// We append the INSERT statement to ensure atomicity.
		fullSQL := string(content) + fmt.Sprintf("\nINSERT INTO d1_migrations (id, name) VALUES (%d, '%s');", id, file.Name())

		if _, err := queryD1(accountID, authToken, databaseID, fullSQL, nil); err != nil {
			return fmt.Errorf("failed to execute migration %s: %v", file.Name(), err)
		}
		fmt.Printf("Migration %s applied successfully\n", file.Name())
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

func queryD1(accountID, authToken, databaseID, sqlStr string, params []any) (*D1Response, error) {
	reqBody := map[string]any{
		"sql":    sqlStr,
		"params": params,
	}
	if reqBody["params"] == nil {
		reqBody["params"] = []any{}
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/d1/database/%s/query", accountID, databaseID)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+authToken)
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
		var errorResp D1Response
		if jsonErr := json.Unmarshal(body, &errorResp); jsonErr == nil && len(errorResp.Errors) > 0 {
			var sb strings.Builder
			for _, e := range errorResp.Errors {
				sb.WriteString(fmt.Sprintf("[%d] %s; ", e.Code, e.Message))
				if e.Code == 7500 {
					sb.WriteString(" (提示: Token 缺少 'D1:Edit' 权限，请在 Cloudflare 后台为 Token 添加 Account->Workers D1->Edit 权限)")
				}
				if strings.Contains(e.Message, "SQLITE_AUTH") {
					sb.WriteString(fmt.Sprintf(" (Hint: Check if Token has 'D1:Edit' permission, and AccountID '%s' matches DatabaseID '%s'. Also ensure you are using a Token, not Global Key)", accountID, databaseID))
				}
			}
			return nil, fmt.Errorf("D1 API error (Status %d): %s", resp.StatusCode, sb.String())
		}
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var d1Resp D1Response
	if err := json.Unmarshal(body, &d1Resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v, body: %s", err, string(body))
	}

	if !d1Resp.Success {
		var sb strings.Builder
		for _, e := range d1Resp.Errors {
			sb.WriteString(fmt.Sprintf("[%d] %s; ", e.Code, e.Message))
			if strings.Contains(e.Message, "SQLITE_AUTH") {
				sb.WriteString(fmt.Sprintf(" (Hint: Check if Token has 'D1:Edit' permission, and AccountID '%s' matches DatabaseID '%s')", accountID, databaseID))
			}
		}
		return nil, fmt.Errorf("D1 API error: %s", sb.String())
	}

	return &d1Resp, nil
}
