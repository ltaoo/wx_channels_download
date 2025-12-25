package main

import (
	_ "embed"
	"fmt"

	"wx_channel/cmd"
	"wx_channel/config"
	"wx_channel/internal/api"
	"wx_channel/internal/interceptor"
	"wx_channel/pkg/certificate"
	"wx_channel/pkg/platform"
)

var AppVer = "25122607"

func main() {
	cfg, err := config.New()
	if err != nil {
		fmt.Printf("加载配置文件失败 %v", err.Error())
		return
	}
	interceptor.SetDefaultSettings(cfg)
	api.SetDefaultSettings(cfg)
	if err := cfg.LoadConfig(); err != nil {
		fmt.Printf("加载配置文件失败 %v", err.Error())
		return
	}
	interceptor_settings := interceptor.NewInterceptorSettings(cfg)
	if interceptor_settings.ProxySetSystem && platform.NeedAdminPermission() && !platform.IsAdmin() {
		if !platform.RequestAdminPermission() {
			fmt.Println("启动失败，请右键选择「以管理员身份运行」")
			return
		}
		return
	}
	if err := cmd.Execute(AppVer, certificate.DefaultCertFiles, cfg, interceptor_settings); err != nil {
		fmt.Printf("初始化失败 %v\n", err.Error())
	}
}
