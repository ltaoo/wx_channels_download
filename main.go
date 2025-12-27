package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	"wx_channel/cmd"
	"wx_channel/config"
	"wx_channel/internal/api"
	"wx_channel/internal/interceptor"
	"wx_channel/pkg/certificate"
	"wx_channel/pkg/platform"
)

var AppVer = "251226"
var Mode = "debug"

func main() {
	if Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	cfg, err := config.New()
	if err != nil {
		fmt.Printf("加载配置文件失败 %v", err.Error())
		return
	}
	interceptor.RegisterSettings(cfg)
	api.RegisterSettings(cfg)
	if err := cfg.LoadConfig(); err != nil {
		fmt.Printf("加载配置文件失败 %v", err.Error())
		return
	}
	if viper.GetBool("proxy.system") && platform.NeedAdminPermission() && !platform.IsAdmin() {
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
