package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"wx_channel/pkg/system"
)

var status_cmd = &cobra.Command{
	Use:   "status",
	Short: "查看服务状态",
	Run: func(cmd *cobra.Command, args []string) {
		status_command()
	},
}

func init() {
	root_cmd.AddCommand(status_cmd)
}

type ServiceStatus struct {
	Version string `json:"version"`
	PID     int    `json:"pid"`
	Running bool   `json:"running"`
	API     struct {
		Addr      string `json:"addr"`
		Listening bool   `json:"listening"`
	} `json:"api"`
	Proxy struct {
		Addr      string `json:"addr"`
		Listening bool   `json:"listening"`
	} `json:"proxy"`
}

func status_command() {
	s := ServiceStatus{Version: Version}

	pid, err := read_wx_pidfile()
	if err != nil || pid == 0 {
		s.PID = 0
		s.Running = false
	} else {
		s.PID = pid
		s.Running = system.IsProcessRunning(pid)
		if !s.Running {
			_ = remove_wx_pidfile()
		}
	}

	apiHost := viper.GetString("api.hostname")
	apiPort := viper.GetInt("api.port")
	if apiHost == "" {
		apiHost = "127.0.0.1"
	}
	if apiPort == 0 {
		apiPort = 2710
	}
	s.API.Addr = fmt.Sprintf("%s:%d", apiHost, apiPort)
	s.API.Listening = checkPort(s.API.Addr)

	proxyHost := viper.GetString("proxy.hostname")
	proxyPort := viper.GetInt("proxy.port")
	if proxyHost == "" {
		proxyHost = "127.0.0.1"
	}
	if proxyPort == 0 {
		proxyPort = 2023
	}
	s.Proxy.Addr = fmt.Sprintf("%s:%d", proxyHost, proxyPort)
	s.Proxy.Listening = checkPort(s.Proxy.Addr)

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
		color.Red("API服务: 未运行")
	}
	if s.Proxy.Listening {
		color.Green("代理服务: 运行中 (%s)", s.Proxy.Addr)
	} else {
		color.Red("代理服务: 未运行")
	}
}

func checkPort(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
