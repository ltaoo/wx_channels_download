package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"wx_channel/internal/testui"
)

var (
	testPort   int
	testDir    string
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "启动单测管理服务",
	Long: `启动一个 Web 服务，展示项目中的所有 Go 单元测试，
支持在页面中执行测试并实时查看结果。

默认监听 127.0.0.1:2024，仅本地可访问。`,
	Run: func(cmd *cobra.Command, args []string) {
		runTestUI()
	},
}

func init() {
	testCmd.Flags().IntVarP(&testPort, "port", "p", 2024, "监听端口")
	testCmd.Flags().StringVarP(&testDir, "dir", "d", "", "项目根目录（默认自动检测 go.mod）")
	root_cmd.AddCommand(testCmd)
}

func runTestUI() {
	// Resolve module directory
	modDir := testDir
	if modDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			color.Red(fmt.Sprintf("获取当前目录失败: %v\n", err))
			os.Exit(1)
		}
		modDir = testui.ResolveModDir(cwd)
	}

	// Verify go.mod exists
	if _, err := os.Stat(modDir + "/go.mod"); os.IsNotExist(err) {
		color.Red(fmt.Sprintf("未找到 go.mod，请确认在 Go 项目目录中运行或在 --dir 中指定\n"))
		os.Exit(1)
	}

	addr := fmt.Sprintf("127.0.0.1:%d", testPort)

	// Create the test UI server
	srv := testui.NewServer(modDir, addr)
	httpSrv := &http.Server{
		Addr:    addr,
		Handler: srv.Handler(),
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		fmt.Printf("\n🧪 单测管理服务启动成功\n")
		fmt.Printf("   地址: %s\n", color.New(color.FgGreen, color.Underline).Sprintf("http://%s", addr))
		fmt.Printf("   项目: %s\n", modDir)
		fmt.Printf("   按 Ctrl+C 停止服务\n\n")

		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			color.Red(fmt.Sprintf("服务启动失败: %v\n", err))
			os.Exit(1)
		}
	}()

	<-quit
	fmt.Println("\n正在关闭服务...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		color.Red(fmt.Sprintf("关闭服务失败: %v\n", err))
	}
	srv.Close()
	color.Green("服务已关闭")
}
