package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"wx_channel/pkg/cloudflare/worker"
)

var sph_deploy_cmd = &cobra.Command{
	Use:   "sph_deploy",
	Short: "部署视频号查询 Cloudflare Worker",
	Long:  "读取配置文件中的 Cloudflare 配置，将 sph 目录下的 index.html 和 worker.js 部署到 Cloudflare Worker",
	Run: func(cmd *cobra.Command, args []string) {
		sph_deploy()
	},
}

func init() {
	Register(sph_deploy_cmd)
}

func sph_deploy() {
	pterm.DefaultSection.Println("开始部署 视频号查询 Worker (REST API)")

	account_id := viper.GetString("cloudflare.accountId")
	api_token := viper.GetString("cloudflare.apiToken")
	worker_name := viper.GetString("cloudflare.sphWorkerName")
	sph_cookie := viper.GetString("cloudflare.sphCookie")

	if api_token == "" || account_id == "" {
		pterm.Error.Println("错误: 未配置 Cloudflare Auth Token 或 Account ID")
		return
	}

	if worker_name == "" {
		pterm.Error.Println("错误: 未配置 cloudflare.sphWorkerName")
		return
	}

	sph_dir := filepath.Join(Cfg.RootDir, "internal", "api", "sph")

	// 读取 worker.js
	script_path := filepath.Join(sph_dir, "worker.js")
	script_content, err := os.ReadFile(script_path)
	if err != nil {
		pterm.Error.Printf("读取 worker.js 失败: %v\n", err)
		return
	}

	// 读取 index.html
	html_path := filepath.Join(sph_dir, "index.html")
	html_content, err := os.ReadFile(html_path)
	if err != nil {
		pterm.Error.Printf("读取 index.html 失败: %v\n", err)
		return
	}

	// 读取 icon.png 并转 base64，作为 JS 模块部署
	icon_path := filepath.Join(Cfg.RootDir, "build", "icon.png")
	icon_bytes, err := os.ReadFile(icon_path)
	if err != nil {
		pterm.Error.Printf("读取 icon.png 失败: %v\n", err)
		return
	}
	icon_base64 := base64.StdEncoding.EncodeToString(icon_bytes)

	// 构造部署参数 (sph worker 不需要 D1 和额外绑定)
	deploy_body := worker.DeployBody{
		AccountID:         account_id,
		AuthToken:         api_token,
		WorkerName:        worker_name,
		ScriptContent:     script_content,
		CompatibilityDate: "2024-01-01",
		MainModule:        "worker.js",
		Bindings: []worker.Binding{
			{Type: "plain_text", Name: "COOKIE", Text: sph_cookie},
		},
		AdditionalFiles: map[string][]byte{
			"index.html": html_content,
			"icon.js":    []byte(fmt.Sprintf(`export default "%s";`, icon_base64)),
		},
	}

	shortAccountID := account_id
	if len(shortAccountID) > 6 {
		shortAccountID = shortAccountID[:6] + "..."
	}

	spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("正在部署到 Cloudflare (Worker: %s)...", worker_name))
	_, err = worker.Deploy(deploy_body)
	if err != nil {
		spinner.Fail(fmt.Sprintf("部署失败: %v", err))
		return
	}
	spinner.Success("部署成功!")

	// 获取子域名并输出访问地址
	spinner, _ = pterm.DefaultSpinner.Start("正在获取 Worker 访问地址...")
	subdomain, err := get_workers_subdomain(account_id, api_token)
	workerUrl := ""
	if err != nil {
		spinner.Warning(fmt.Sprintf("获取子域名失败: %v", err))
		workerUrl = fmt.Sprintf("https://%s.<your-subdomain>.workers.dev", worker_name)
	} else {
		workerUrl = fmt.Sprintf("https://%s.%s.workers.dev", worker_name, subdomain)
		spinner.Success("获取访问地址成功")
	}

	pterm.Println()
	pterm.DefaultHeader.WithFullWidth().Println("部署摘要")

	panels := pterm.Panels{
		{{Data: pterm.DefaultBox.WithTitle("Worker Info").Sprint(
			pterm.Sprintf("%s: %s\n%s: %s",
				pterm.Bold.Sprint("Worker Name"), pterm.Cyan(worker_name),
				pterm.Bold.Sprint("URL"), pterm.LightGreen(workerUrl),
			),
		)}},
	}
	pterm.DefaultPanel.WithPanels(panels).Render()

	pterm.Println()
	pterm.DefaultHeader.WithFullWidth().Println("可用 API")

	tableData := [][]string{
		{"Method", "Path", "Description"},
		{"GET", "/", "视频号视频信息查询页面"},
		{"POST", "/api/fetch_video_profile", "获取视频号视频信息"},
	}

	pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(tableData).Render()

	pterm.Println()
	pterm.DefaultSection.WithStyle(pterm.NewStyle(pterm.FgGreen)).Println("部署成功! 请访问上面的 URL 使用服务")
}
