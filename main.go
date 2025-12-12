package main

import (
	_ "embed"
	"fmt"

	"wx_channel/cmd"
	"wx_channel/config"
	"wx_channel/internal/interceptor"
	"wx_channel/pkg/platform"
)

//go:embed certs/SunnyRoot.cer
var cert_file []byte

//go:embed certs/private.key
var private_key_file []byte

var server_cert = &interceptor.ServerCert{
	Name:           "SunnyNet",
	CertFile:       cert_file,
	PrivateKeyFile: private_key_file,
}
var AppVer = "251211_18"

func main() {
	cfg, err := config.LoadConfig()
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
	if err := cmd.Execute(AppVer, interceptor.Assets, server_cert, cfg); err != nil {
		fmt.Printf("初始化失败 %v\n", err.Error())
	}
}
