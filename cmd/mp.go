package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
	"wx_channel/internal/api"
	"wx_channel/internal/manager"
	"wx_channel/internal/officialaccount"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	mp_daemon       bool
	mp_daemon_child bool
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
		if mp_daemon {
			mp_start_daemon()
			return
		}
		mp_command()
	},
}

func init() {
	mp_cmd.Flags().BoolVarP(&mp_daemon, "daemon", "d", false, "以守护进程运行")
	mp_cmd.Flags().BoolVar(&mp_daemon_child, "daemon-child", false, "内部参数")
	_ = mp_cmd.Flags().MarkHidden("daemon-child")

	root_cmd.AddCommand(mp_cmd)
	mp_cmd.AddCommand(mp_status_cmd)
	mp_cmd.AddCommand(mp_stop_cmd)
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
	if mp_daemon_child {
		_ = write_mp_pidfile(os.Getpid())
		defer func() {
			_ = remove_mp_pidfile()
		}()
	}
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
		return
	}
	color.Green(fmt.Sprintf("API服务启动成功, 地址: %v", api_srv.Addr()))
	fmt.Println("\n按 Ctrl+C 退出...")
	<-ctx.Done()
	cleanup()
}

func mp_start_daemon() {
	exe, err := os.Executable()
	if err != nil {
		color.Red(fmt.Sprintf("ERROR 获取可执行文件失败: %v\n", err))
		return
	}
	log_fp := filepath.Join(Cfg.RootDir, "mp.log")
	log_file, err := os.OpenFile(log_fp, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		color.Red(fmt.Sprintf("ERROR 打开日志文件失败: %v\n", err))
		return
	}
	defer log_file.Close()
	c := exec.Command(exe, "mp", "--daemon-child")
	c.Stdout = log_file
	c.Stderr = log_file
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := c.Start(); err != nil {
		color.Red(fmt.Sprintf("ERROR 启动守护进程失败: %v\n", err))
		return
	}
	if err := write_mp_pidfile(c.Process.Pid); err != nil {
		color.Red(fmt.Sprintf("ERROR 写入PID文件失败: %v\n", err))
	} else {
		color.Green(fmt.Sprintf("守护进程已启动, PID: %d", c.Process.Pid))
	}
	fmt.Printf("日志文件 %s\n", color.New(color.Underline).Sprint(log_fp))
}

func mp_pidfile_path() string {
	return filepath.Join(Cfg.RootDir, "mp.pid")
}

func write_mp_pidfile(pid int) error {
	fp := mp_pidfile_path()
	return os.WriteFile(fp, []byte(strconv.Itoa(pid)), 0644)
}

func read_mp_pidfile() (int, error) {
	fp := mp_pidfile_path()
	data, err := os.ReadFile(fp)
	if err != nil {
		return 0, err
	}
	p, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, err
	}
	return p, nil
}

func remove_mp_pidfile() error {
	fp := mp_pidfile_path()
	if _, err := os.Stat(fp); err == nil {
		return os.Remove(fp)
	}
	return nil
}

var mp_status_cmd = &cobra.Command{
	Use:   "status",
	Short: "查看公众号服务状态",
	Run: func(cmd *cobra.Command, args []string) {
		api_cfg := api.NewAPIConfig(Cfg, true)
		pid, err := read_mp_pidfile()
		if err != nil || pid == 0 {
			color.Red("未发现守护进程")
			return
		}
		running := process_running(pid)
		if !running {
			color.Red(fmt.Sprintf("进程未运行, PID: %d", pid))
			_ = remove_mp_pidfile()
			return
		}
		ok := port_listening(api_cfg.Addr)
		type Status struct {
			PID       int    `json:"pid"`
			Addr      string `json:"addr"`
			Listening bool   `json:"listening"`
		}
		s := Status{PID: pid, Addr: api_cfg.Addr, Listening: ok}
		b, _ := json.Marshal(s)
		fmt.Println(string(b))
	},
}

var mp_stop_cmd = &cobra.Command{
	Use:   "stop",
	Short: "停止公众号服务",
	Run: func(cmd *cobra.Command, args []string) {
		pid, err := read_mp_pidfile()
		if err != nil || pid == 0 {
			color.Red("未发现守护进程")
			return
		}
		if !process_running(pid) {
			color.Green("进程已停止")
			_ = remove_mp_pidfile()
			return
		}
		_ = syscall.Kill(pid, syscall.SIGTERM)
		expire := time.After(8 * time.Second)
		tick := time.NewTicker(200 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				if !process_running(pid) {
					_ = remove_mp_pidfile()
					color.Green("服务已关闭")
					return
				}
			case <-expire:
				color.Red("关闭超时")
				return
			}
		}
	},
}

func process_running(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, syscall.Signal(0))
	return err == nil
}

func port_listening(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 800*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
