package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"wx_channel/internal/api"
	"wx_channel/internal/manager"
	"wx_channel/internal/officialaccount"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var mp_cmd = &cobra.Command{
	Use:   "mp",
	Short: "公众号服务",
	Long:  "仅启用公众号相关功能",
	Run: func(cmd *cobra.Command, args []string) {
		command := cmd.Name()
		if command != "mp" {
			return
		}
		mp_command()
	},
}

func init() {
	root_cmd.AddCommand(mp_cmd)
}

func mp_command() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := Cfg
	fmt.Printf("\nv%v\n", cfg.Version)
	fmt.Printf("问题反馈 https://github.com/ltaoo/wx_channels_download/issues\n\n")

	if cfg.FullPath != "" {
		fmt.Printf("配置文件 %s\n", color.New(color.Underline).Sprint(cfg.FullPath))
	}
	mgr := manager.NewServerManager()
	api_cfg := api.NewAPIConfig(Cfg, true)
	mp_token_filepath, err := officialaccount.ValidateTokenFilepath(api_cfg.OfficialAccountTokenFilepath, cfg.RootDir)
	if mp_token_filepath != "" && err == nil {
		fmt.Printf("公众号授权凭证文件 %s\n", color.New(color.Underline).Sprint(mp_token_filepath))
	}
	l, err := net.Listen("tcp", api_cfg.Addr)
	if err != nil {
		color.Red(fmt.Sprintf("启动API服务失败，%s 被占用\n\n", api_cfg.Addr))
		os.Exit(0)
		return
	}
	l.Close()
	api_srv := api.NewAPIServer(api_cfg)
	mgr.RegisterServer(api_srv)
	cleanup := func() {
		fmt.Printf("\n正在关闭服务...\n")
		if err := mgr.StopServer("api"); err != nil {
			color.Red(fmt.Sprintf("⚠️ 关闭API服务失败: %v\n", err))
		}
		color.Green("服务已关闭")
	}
	if err := mgr.StartServer("api"); err != nil {
		color.Red(fmt.Sprintf("ERROR 启动API服务失败: %v\n", err.Error()))
		cleanup()
		os.Exit(0)
	}
	color.Green(fmt.Sprintf("API服务启动成功, 地址: %v", api_srv.Addr()))
	fmt.Println("\n按 Ctrl+C 退出...")
	<-ctx.Done()
	cleanup()
}
