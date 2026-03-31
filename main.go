package main

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"wx_channel/cmd"
	"wx_channel/internal/config"
)

var AppVer = "260330"
var Mode = "debug"

func main() {
	if Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	cfg := config.New(AppVer, Mode)
	if err := cmd.Execute(cfg); err != nil {
		fmt.Printf("运行失败 %v\n", err.Error())
	}
}
