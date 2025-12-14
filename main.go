package main

import (
	_ "embed"
	"fmt"

	"wx_channel/cmd"
	"wx_channel/internal/interceptor"
	"wx_channel/pkg/certificate"
	"wx_channel/pkg/platform"
)

var AppVer = "251213"

func main() {
	cfg, err := interceptor.NewInterceptorSettings()
	if err != nil {
		fmt.Printf("加载配置文件失败 %v", err.Error())
		return
	}
	if cfg.ProxySetSystem && platform.NeedAdminPermission() && !platform.IsAdmin() {
		if !platform.RequestAdminPermission() {
			fmt.Println("启动失败，请右键选择「以管理员身份运行」")
			return
		}
		return
	}
	if err := cmd.Execute(AppVer, certificate.DefaultCertFiles, cfg); err != nil {
		fmt.Printf("初始化失败 %v\n", err.Error())
	}
}
