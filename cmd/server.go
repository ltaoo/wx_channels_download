package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"wx_channel/internal/application"
	"wx_channel/internal/config"
	"wx_channel/pkg/system"
)

var status_quiet bool

var server_cmd = &cobra.Command{
	Use:   "server",
	Short: "运行服务器",
	RunE: func(cmd *cobra.Command, args []string) error {
		if start_transferred {
			return nil
		}
		return run_server_with_pidfile()
	},
}

var server_status_cmd = &cobra.Command{
	Use:   "status",
	Short: "查看服务器状态",
	Run: func(cmd *cobra.Command, args []string) {
		apply_server_config_defaults()
		if ok := status_command(); !ok {
			os.Exit(1)
		}
	},
}

var server_stop_cmd = &cobra.Command{
	Use:   "stop",
	Short: "停止服务器",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !stop_server() {
			return fmt.Errorf("停止服务器失败")
		}
		return nil
	},
}

var server_restart_cmd = &cobra.Command{
	Use:   "restart",
	Short: "重启服务器",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !stop_server() {
			return fmt.Errorf("停止服务器失败")
		}
		if start_transferred {
			return nil
		}
		return run_server_with_pidfile()
	},
}

func init() {
	server_status_cmd.Flags().BoolVarP(&status_quiet, "quiet", "q", false, "仅返回退出码")
	server_cmd.AddCommand(server_status_cmd, server_stop_cmd, server_restart_cmd)
	root_cmd.AddCommand(server_cmd)
}

func apply_server_config_defaults() {
	if Cfg == nil {
		return
	}
	Cfg.Update("proxy.skipInstallRootCert", true)
	Cfg.Update("proxy.enabled", false)
	Cfg.Update("proxy.system", false)
}

func run_server() error {
	apply_server_config_defaults()
	return application.Start(Cfg)
}

func run_server_with_pidfile() error {
	if pid, err := read_server_pidfile(); err == nil && pid > 0 {
		if system.IsProcessRunning(pid) {
			return fmt.Errorf("服务器已在运行, PID: %d", pid)
		}
		_ = remove_server_pidfile()
	}
	if err := write_server_pidfile(os.Getpid()); err != nil {
		color.Red(fmt.Sprintf("ERROR 写入 PID 文件失败: %v\n", err))
	} else {
		defer remove_server_pidfile()
	}
	return run_server()
}

func stop_server() bool {
	pid, err := read_server_pidfile()
	if err != nil || pid == 0 {
		color.Red("未发现服务器进程")
		return true
	}
	if !system.IsProcessRunning(pid) {
		_ = remove_server_pidfile()
		color.Green("进程已停止")
		return true
	}
	if err := system.TerminateProcess(pid); err != nil {
		color.Red(fmt.Sprintf("ERROR 停止服务器失败: %v\n", err))
		return false
	}

	expire := time.After(8 * time.Second)
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if !system.IsProcessRunning(pid) {
				_ = remove_server_pidfile()
				color.Green("服务已关闭")
				return true
			}
		case <-expire:
			color.Red("关闭超时")
			return false
		}
	}
}

func server_pidfile_path() string {
	return filepath.Join(server_state_dir(), "wx_video_download.pid")
}

func server_state_dir() string {
	if Cfg != nil {
		if dir := strings.TrimSpace(Cfg.WorkDir); dir != "" {
			return dir
		}
		if dir := strings.TrimSpace(Cfg.RootDir); dir != "" {
			return dir
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

func write_server_pidfile(pid int) error {
	if err := os.MkdirAll(server_state_dir(), 0755); err != nil {
		return err
	}
	return os.WriteFile(server_pidfile_path(), []byte(strconv.Itoa(pid)), 0644)
}

func read_server_pidfile() (int, error) {
	data, err := os.ReadFile(server_pidfile_path())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func remove_server_pidfile() error {
	if _, err := os.Stat(server_pidfile_path()); err == nil {
		return os.Remove(server_pidfile_path())
	}
	return nil
}

type command_status struct {
	Version string          `json:"version"`
	PID     int             `json:"pid"`
	Running bool            `json:"running"`
	PIDFile string          `json:"pidFile"`
	API     endpoint_status `json:"api"`
	Proxy   endpoint_status `json:"proxy"`
}

type endpoint_status struct {
	Enabled   bool   `json:"enabled"`
	Addr      string `json:"addr"`
	Listening bool   `json:"listening"`
}

func status_command() bool {
	s := command_status{
		Version: Version,
		PIDFile: server_pidfile_path(),
	}

	pid, err := read_server_pidfile()
	if err == nil && pid > 0 {
		s.PID = pid
		s.Running = system.IsProcessRunning(pid)
		if !s.Running {
			_ = remove_server_pidfile()
		}
	}

	api_host := Cfg.GetString("api.hostname")
	api_port := Cfg.GetInt("api.port")
	s.API.Enabled = true
	s.API.Addr = listen_addr(api_host, api_port)
	s.API.Listening = check_port(api_host, api_port)

	proxy_enabled := Cfg.GetBool("proxy.enabled")
	proxy_host := Cfg.GetString("proxy.hostname")
	proxy_port := Cfg.GetInt("proxy.port")
	s.Proxy.Enabled = proxy_enabled
	s.Proxy.Addr = listen_addr(proxy_host, proxy_port)
	if proxy_enabled {
		s.Proxy.Listening = check_port(proxy_host, proxy_port)
	}

	ok := s.Running && s.API.Listening && (!s.Proxy.Enabled || s.Proxy.Listening)
	if status_quiet {
		return ok
	}

	b, _ := json.MarshalIndent(s, "", "  ")
	fmt.Println(string(b))
	fmt.Println()
	color.Cyan("版本: %s", s.Version)
	if s.Running {
		color.Green("进程: 运行中 (PID: %d)", s.PID)
	} else {
		color.Red("进程: 未运行")
	}
	if s.API.Listening {
		color.Green("API服务: 运行中 (%s)", s.API.Addr)
	} else {
		color.Red("API服务: 未运行 (%s)", s.API.Addr)
	}
	if !s.Proxy.Enabled {
		color.Yellow("代理服务: 未启用")
	} else if s.Proxy.Listening {
		color.Green("代理服务: 运行中 (%s)", s.Proxy.Addr)
	} else {
		color.Red("代理服务: 未运行 (%s)", s.Proxy.Addr)
	}
	return ok
}

func listen_addr(host string, port int) string {
	host = strings.TrimSpace(host)
	if host == "" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func dial_addr(host string, port int) string {
	return net.JoinHostPort(config.APIClientHostname(host), strconv.Itoa(port))
}

func check_port(host string, port int) bool {
	conn, err := net.DialTimeout("tcp", dial_addr(host, port), 800*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
