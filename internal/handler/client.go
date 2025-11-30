package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/ltaoo/echo"
	"github.com/ltaoo/echo/plugin"

	"wx_channel/config"
	"wx_channel/pkg/certificate"
	"wx_channel/pkg/proxy"
)

type HandlerClient struct {
	Version        string
	SetSystemProxy bool
	Device         string
	Hostname       string
	Port           int
	Debug          bool
	CertFile       []byte
	PrivateKeyFile []byte
	CertFileName   string
	channel_files  *ChannelInjectedFiles
	cfg            *config.Config
	echo           *echo.Echo
}
type ServerCertFiles struct {
	CertFile       []byte
	PrivateKeyFile []byte
}
type HandlerClientPayload struct {
	Version        string
	SetSystemProxy bool
	Device         string
	Hostname       string
	Port           int
	CertFiles      *ServerCertFiles
	CertFileName   string
	ChannelFiles   *ChannelInjectedFiles
	Cfg            *config.Config
	Debug          bool
}

func NewHandlerClient(payload HandlerClientPayload) (*HandlerClient, error) {
	echo, err := echo.NewEcho(payload.CertFiles.CertFile, payload.CertFiles.PrivateKeyFile)
	if err != nil {
		return nil, err
	}
	echo.AddPlugin(HandleHttpRequestEcho(payload.Version, payload.ChannelFiles, payload.Cfg))
	if payload.Debug {
		echo.AddPlugin(&plugin.Plugin{
			Match: "debug.weixin.qq.com",
			Target: &plugin.TargetConfig{
				Protocol: "http",
				Host:     "127.0.0.1",
				Port:     6752,
			},
		})
	}
	return &HandlerClient{
		Version:        payload.Version,
		SetSystemProxy: payload.SetSystemProxy,
		Device:         payload.Device,
		Port:           payload.Port,
		Debug:          payload.Debug,
		CertFile:       payload.CertFiles.CertFile,
		PrivateKeyFile: payload.CertFiles.PrivateKeyFile,
		CertFileName:   payload.CertFileName,
		channel_files:  payload.ChannelFiles,
		cfg:            payload.Cfg,
		echo:           echo,
	}, nil
}

func (c *HandlerClient) Start() error {
	existing, err := certificate.CheckHasCertificate(c.CertFileName)
	if err != nil {
		return fmt.Errorf("检查证书失败: %v", err)
	}

	if !existing {
		fmt.Printf("正在安装证书...\n")
		if err := certificate.InstallCertificate(c.CertFile); err != nil {
			return fmt.Errorf("安装证书失败: %v", err)
		}
	}

	if c.SetSystemProxy {
		if err := proxy.EnableProxy(proxy.ProxySettings{
			Device:   c.Device,
			Hostname: c.Hostname,
			Port:     strconv.Itoa(c.Port),
		}); err != nil {
			return fmt.Errorf("设置代理失败: %v", err)
		}
	}
	return nil
}

func (c *HandlerClient) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.echo.ServeHTTP(w, r)
}

func (c *HandlerClient) Stop() error {
	if c.SetSystemProxy {
		arg := proxy.ProxySettings{
			Device:   c.Device,
			Hostname: c.Hostname,
			Port:     strconv.Itoa(c.Port),
		}
		err := proxy.DisableProxy(arg)
		if err != nil {
			return fmt.Errorf("关闭系统代理失败: %v", err)
		}
	}
	return nil
}
