package main

import (
	_ "embed"
	"fmt"

	"wx_channel/cmd"
	"wx_channel/config"
	"wx_channel/pkg/platform"
)

var RootCertificateName = "SunnyNet"
var AppVer = "251122"

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("加载配置文件失败 %v", err.Error())
		return
	}
	if cfg.ProxySystem && platform.NeedAdminPermission() && !platform.IsAdmin() {
		if !platform.RequestAdminPermission() {
			fmt.Println("启动失败，请右键选择「以管理员身份运行」")
			return
		}
		return
	}
	if err := cmd.Execute(AppVer, RootCertificateName, channel_files, files, cfg); err != nil {
		fmt.Printf("初始化失败 %v", err.Error())
		fmt.Printf("按 Ctrl+C 退出...\n")
		select {}
	}
}
