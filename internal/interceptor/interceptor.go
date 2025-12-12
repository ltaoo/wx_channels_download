package interceptor

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/ltaoo/echo"

	"wx_channel/config"
	"wx_channel/pkg/certificate"
	"wx_channel/pkg/proxy"
)

type InterceptorConfig struct {
	Version        string
	SetSystemProxy bool
	Device         string
	Hostname       string
	Port           int
	CertFiles      *certificate.CertFileAndKeyFile
	ChannelFiles   *ChannelInjectedFiles
	Cfg            *config.Config
	Debug          bool
}

type Interceptor struct {
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

func NewInterceptor(payload InterceptorConfig) (*Interceptor, error) {
	echo.SetLogEnabled(false)
	client, err := echo.NewEcho(payload.CertFiles.Cert, payload.CertFiles.PrivateKey)
	if err != nil {
		return nil, err
	}
	client.AddPlugin(CreateChannelInterceptorPlugin(payload.Version, payload.ChannelFiles, payload.Cfg))
	if payload.Debug {
		client.AddPlugin(&echo.Plugin{
			Match: "debug.weixin.qq.com",
			Target: &echo.TargetConfig{
				Protocol: "http",
				Host:     "127.0.0.1",
				Port:     6752,
			},
		})
	}
	return &Interceptor{
		Version:        payload.Version,
		SetSystemProxy: payload.SetSystemProxy,
		Device:         payload.Device,
		Port:           payload.Port,
		Debug:          payload.Debug,
		CertFile:       payload.CertFiles.Cert,
		PrivateKeyFile: payload.CertFiles.PrivateKey,
		CertFileName:   payload.CertFiles.Name,
		channel_files:  payload.ChannelFiles,
		cfg:            payload.Cfg,
		echo:           client,
	}, nil
}

func (c *Interceptor) Start() error {
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

func (c *Interceptor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.echo.ServeHTTP(w, r)
}

func (c *Interceptor) Stop() error {
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
