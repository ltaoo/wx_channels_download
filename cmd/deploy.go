package cmd

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"wx_channel/internal/workers/hub"
	"wx_channel/internal/workers/mp"
	"wx_channel/internal/workers/sph"
)

var deploy_cmd = &cobra.Command{
	Use:   "deploy",
	Short: "部署 Cloudflare Worker 和 Pages",
	Long:  "读取配置文件中的 Cloudflare 配置，通过 Cloudflare REST API 部署 Worker 和 Pages",
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
	Short: "部署 Durable Objects 任务 Hub 和管理页面",
	Long:  "通过 Cloudflare REST API 部署原生 JavaScript Durable Objects Worker 和 Pages 管理页面",
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

	spinner, _ := pterm.DefaultSpinner.Start("正在部署 Hub Worker 和 Pages...")
	result, err := hub.Deploy(context.Background(), hub.DeployOptions{
		AccountID:        viper.GetString("cloudflare.accountId"),
		AuthToken:        viper.GetString("cloudflare.apiToken"),
		WorkerName:       viper.GetString("hub.deploy.workerName"),
		PagesProjectName: viper.GetString("hub.deploy.pagesProjectName"),
		HubToken:         viper.GetString("hub.deploy.token"),
		AdminToken:       viper.GetString("hub.deploy.adminToken"),
		RepositoryDir:    Cfg.RootDir,
		Progress: func(progress hub.DeployProgress) {
			spinner.UpdateText(progress.Message)
		},
	})
	if err != nil {
		spinner.Fail(err.Error())
		if result != nil && result.WorkerID != "" {
			pterm.Info.Println("提示: Hub Worker 已部署；请根据上面的 Pages 构建或 API 错误处理后重试")
		} else {
			pterm.Info.Println("提示: Cloudflare API Token 需要 Workers Scripts:Edit 和 Pages:Edit 权限")
		}
		return
	}
	spinner.Success(fmt.Sprintf(
		"Hub Worker 和 Pages 部署成功：%d 字节 JavaScript，%d 个静态文件",
		result.ScriptBytes,
		result.PagesFiles,
	))
	if result.WorkerURLWarning != "" {
		pterm.Warning.Println(result.WorkerURLWarning)
	}

	pterm.Println()
	pterm.DefaultHeader.WithFullWidth().Println("Hub 部署摘要")
	table_data := [][]string{
		{"项目", "值"},
		{"Worker", result.WorkerName},
		{"Worker ID", result.WorkerID},
		{"URL", result.WorkerURL},
		{"健康检查", result.WorkerURL + "/health"},
		{"Pages 项目", result.PagesProjectName},
		{"管理页面", result.PagesURL},
		{"Pages Deployment", result.PagesDeploymentID},
		{"管理 API", result.PagesURL + "/admin/api/overview"},
		{"WebSocket", result.WorkerURL + "/v1/connect"},
	}
	pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(table_data).Render()
	pterm.Println()
	pterm.Info.Println("将上面的 URL 和 hub.deploy.token 写入每台设备的 hub.url 和 hub.token。")
	pterm.Info.Println("管理页面使用用户名 admin 和 hub.deploy.adminToken 登录。")
}

func deploy_mp() {
	pterm.DefaultSection.Println("开始部署公众号 Worker (REST API)")

	spinner, _ := pterm.DefaultSpinner.Start("正在准备 D1 并部署公众号 Worker...")
	result, err := mp.Deploy(context.Background(), mp.DeployOptions{
		AccountID:            viper.GetString("cloudflare.accountId"),
		AuthToken:            viper.GetString("cloudflare.apiToken"),
		WorkerName:           viper.GetString("cloudflare.workerName"),
		D1DatabaseID:         viper.GetString("cloudflare.d1Id"),
		D1DatabaseName:       viper.GetString("cloudflare.d1Name"),
		AdminToken:           viper.GetString("cloudflare.adminToken"),
		RefreshToken:         viper.GetString("cloudflare.refreshToken"),
		RemoteServerHostname: viper.GetString("mp.remoteServer.hostname"),
		Progress: func(progress mp.DeployProgress) {
			spinner.UpdateText(progress.Message)
		},
	})
	if err != nil {
		spinner.Fail(err.Error())
		pterm.Info.Println("提示: Cloudflare API Token 需要 Workers Scripts:Edit 和 D1:Edit 权限")
		return
	}
	spinner.Success(fmt.Sprintf("部署成功，已上传 %d 字节 JavaScript", result.ScriptBytes))
	if result.MigrationWarning != "" {
		pterm.Warning.Println(result.MigrationWarning)
	}
	if result.WorkerURLWarning != "" {
		pterm.Warning.Println(result.WorkerURLWarning)
	}

	pterm.Println()
	pterm.DefaultHeader.WithFullWidth().Println("部署摘要")
	table_data := [][]string{
		{"项目", "值"},
		{"Worker", result.WorkerName},
		{"Worker ID", result.WorkerID},
		{"URL", result.WorkerURL},
		{"D1 Database", result.D1DatabaseID},
	}
	pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(table_data).Render()

	pterm.Println()
	pterm.DefaultHeader.WithFullWidth().Println("可用 API 列表")
	table_data = [][]string{
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
	pterm.DefaultSection.WithStyle(pterm.NewStyle(pterm.FgGreen)).Println("部署成功! 请访问上面的 URL 使用服务")
}

func deploy_sph() {
	pterm.DefaultSection.Println("开始部署 视频号查询 Worker (REST API)")

	spinner, _ := pterm.DefaultSpinner.Start("正在部署视频号查询 Worker...")
	result, err := sph.Deploy(context.Background(), sph.DeployOptions{
		AccountID:     viper.GetString("cloudflare.accountId"),
		AuthToken:     viper.GetString("cloudflare.apiToken"),
		WorkerName:    viper.GetString("cloudflare.sphWorkerName"),
		Cookie:        viper.GetString("cloudflare.sphCookie"),
		Credential:    viper.GetString("cloudflare.sphCredential"),
		RepositoryDir: Cfg.RootDir,
		Progress: func(progress sph.DeployProgress) {
			spinner.UpdateText(progress.Message)
		},
	})
	if err != nil {
		spinner.Fail(err.Error())
		pterm.Info.Println("提示: Cloudflare API Token 需要 Workers Scripts:Edit 权限")
		return
	}
	spinner.Success(fmt.Sprintf("部署成功，已上传 %d 字节 JavaScript", result.ScriptBytes))
	if result.WorkerURLWarning != "" {
		pterm.Warning.Println(result.WorkerURLWarning)
	}

	pterm.Println()
	pterm.DefaultHeader.WithFullWidth().Println("部署摘要")
	table_data := [][]string{
		{"项目", "值"},
		{"Worker", result.WorkerName},
		{"Worker ID", result.WorkerID},
		{"URL", result.WorkerURL},
	}
	pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(table_data).Render()

	pterm.Println()
	pterm.DefaultHeader.WithFullWidth().Println("可用 API")
	table_data = [][]string{
		{"Method", "Path", "Description"},
		{"GET", "/", "视频号视频信息查询页面"},
		{"POST", "/api/fetch_video_profile", "获取视频号视频信息"},
	}
	pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(table_data).Render()

	pterm.Println()
	pterm.DefaultSection.WithStyle(pterm.NewStyle(pterm.FgGreen)).Println("部署成功! 请访问上面的 URL 使用服务")
}
