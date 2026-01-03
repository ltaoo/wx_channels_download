package interceptor

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"

	"github.com/ltaoo/echo"
	"github.com/rs/zerolog"

	"wx_channel/internal/interceptor/proxy"
	"wx_channel/pkg/certificate"
	"wx_channel/pkg/system"
)

type Interceptor struct {
	Version           string
	Debug             bool
	Settings          *InterceptorConfig
	Headers           map[string]string
	Cert              *certificate.CertFileAndKeyFile
	proxy             proxy.InnerProxy
	PostPlugins       []interface{}  // echo 的插件，将在 echo 初始化后传给 echo
	FrontendVariables map[string]any // 前端额外的全局变量
	log               *zerolog.Logger
}

func NewInterceptor(cfg *InterceptorConfig, cert *certificate.CertFileAndKeyFile) *Interceptor {
	log := zerolog.New(io.Discard).With().Timestamp().Str("component", "interceptor").Str("version", cfg.Version).Logger()
	return &Interceptor{
		Version:           cfg.Version,
		Debug:             cfg.DebugShowError,
		Settings:          cfg,
		FrontendVariables: make(map[string]any),
		Cert:              cert,
		log:               &log,
		proxy:             nil,
	}
}

func (c *Interceptor) Start() error {
	echo.SetLogEnabled(false)
	client, err := proxy.NewProxy(c.Cert.Cert, c.Cert.PrivateKey)
	if err != nil {
		return err
	}
	client.AddPlugin(CreateChannelInterceptorPlugin(c, Assets))
	if c.Debug {
		client.AddPlugin(&proxy.Plugin{
			Match: "debug.weixin.qq.com",
			Target: &proxy.TargetConfig{
				Protocol: "http",
				Host:     "127.0.0.1",
				Port:     6752,
			},
		})
	}
	if len(c.PostPlugins) != 0 {
		for _, plugin := range c.PostPlugins {
			client.AddPlugin(plugin)
		}
	}
	c.proxy = client
	existing, err := certificate.CheckHasCertificate(c.Cert.Name)
	if err != nil {
		return fmt.Errorf("检查证书失败: %v", err)
	}
	if !existing {
		fmt.Printf("正在安装证书...\n")
		if err := certificate.InstallCertificate(c.Cert.Cert); err != nil {
			return fmt.Errorf("安装证书失败: %v", err)
		}
	}
	if c.Settings.ProxySetSystem {
		if err := system.EnableProxy(system.ProxySettings{
			Device:   c.Settings.ProxyDevice,
			Hostname: c.Settings.ProxyServerHostname,
			Port:     strconv.Itoa(c.Settings.ProxyServerPort),
		}); err != nil {
			return fmt.Errorf("设置代理失败: %v", err)
		}
	}
	if err := client.Start(c.Settings.ProxyServerPort); err != nil {
		return err
	}
	return nil
}

func (c *Interceptor) Stop() error {
	if c.Settings.ProxySetSystem {
		arg := system.ProxySettings{
			Device:   c.Settings.ProxyDevice,
			Hostname: c.Settings.ProxyServerHostname,
			Port:     strconv.Itoa(c.Settings.ProxyServerPort),
		}
		err := system.DisableProxy(arg)
		if err != nil {
			return fmt.Errorf("关闭系统代理失败: %v", err)
		}
	}
	return nil
}

func (c *Interceptor) SetVersion(v string) {
	c.Version = v
}
func (c *Interceptor) AddPostPlugin(plugin interface{}) {
	c.PostPlugins = append(c.PostPlugins, plugin)
}
func (c *Interceptor) AddPlugin(plugin interface{}) {
	if c.proxy != nil {
		c.proxy.AddPlugin(plugin)
	}
}
func (c *Interceptor) SetLog(writer io.Writer) {
	l := zerolog.New(writer).With().Timestamp().Str("component", "interceptor").Str("version", c.Version).Logger()
	c.log = &l
}
func (c *Interceptor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if h, _, err := net.SplitHostPort(r.Host); err == nil {
		host = h
	}
	isLocal := false
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		isLocal = true
	}
	if host == "localhost" || host == c.Settings.ProxyServerHostname {
		isLocal = true
	}
	if isLocal && r.URL.Path == "/cert" {
		w.Header().Set("Content-Type", "application/x-x509-ca-cert")
		w.Header().Set("Content-Disposition", "attachment; filename=\"SunnyNet.cer\"")
		w.Write(c.Cert.Cert)
		return
	}
	if isLocal && (r.URL.Path == "/" || r.URL.Path == "") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<html><head><title>wx_channels_download</title></head><body><h1>代理服务运行中</h1><p><a href="/cert">点击下载证书</a></p></body></html>`)
		return
	}
	c.proxy.ServeHTTP(w, r)
}
