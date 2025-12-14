package interceptor

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/ltaoo/echo"
	"github.com/rs/zerolog"

	"wx_channel/pkg/certificate"
	"wx_channel/pkg/proxy"
)

type Interceptor struct {
	Version     string
	Debug       bool
	Settings    *InterceptorSettings
	Cert        *certificate.CertFileAndKeyFile
	echo        *echo.Echo
	PostPlugins []*echo.Plugin // echo 的插件，将在 echo 初始化后传给 echo
	log         *zerolog.Logger
}

func NewInterceptor(payload *InterceptorSettings, cert *certificate.CertFileAndKeyFile) *Interceptor {
	log := zerolog.New(io.Discard).With().Timestamp().Str("component", "interceptor").Str("version", payload.Version).Logger()
	return &Interceptor{
		Version:  payload.Version,
		Debug:    payload.DebugShowError,
		Settings: payload,
		Cert:     cert,
		log:      &log,
		echo:     nil,
	}
}

func (c *Interceptor) Start() error {
	echo.SetLogEnabled(false)
	client, err := echo.NewEcho(c.Cert.Cert, c.Cert.PrivateKey)
	if err != nil {
		return err
	}
	client.AddPlugin(CreateChannelInterceptorPlugin(c.Version, Assets, c.Settings))
	if c.Debug {
		client.AddPlugin(&echo.Plugin{
			Match: "debug.weixin.qq.com",
			Target: &echo.TargetConfig{
				Protocol: "http",
				Host:     "127.0.0.1",
				Port:     6752,
			},
		})
	}
	if len(c.PostPlugins) != 0 {
		for _, plugin := range c.PostPlugins {
			fmt.Println("add plugin")
			client.AddPlugin(plugin)
		}
	}
	c.echo = client
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
		if err := proxy.EnableProxy(proxy.ProxySettings{
			Device:   c.Settings.ProxyDevice,
			Hostname: c.Settings.ProxyServerHostname,
			Port:     strconv.Itoa(c.Settings.ProxyServerPort),
		}); err != nil {
			return fmt.Errorf("设置代理失败: %v", err)
		}
	}
	return nil
}

func (c *Interceptor) Stop() error {
	if c.Settings.ProxySetSystem {
		arg := proxy.ProxySettings{
			Device:   c.Settings.ProxyDevice,
			Hostname: c.Settings.ProxyServerHostname,
			Port:     strconv.Itoa(c.Settings.ProxyServerPort),
		}
		err := proxy.DisableProxy(arg)
		if err != nil {
			return fmt.Errorf("关闭系统代理失败: %v", err)
		}
	}
	return nil
}

func (c *Interceptor) SetVersion(v string) {
	c.Version = v
}
func (c *Interceptor) AddPostPlugin(plugin *echo.Plugin) {
	c.PostPlugins = append(c.PostPlugins, plugin)
}
func (c *Interceptor) AddPlugin(plugin *echo.Plugin) {
	if c.echo != nil {
		c.echo.AddPlugin(plugin)
	}
}
func (c *Interceptor) SetLog(writer io.Writer) {
	l := zerolog.New(writer).With().Timestamp().Str("component", "interceptor").Str("version", c.Version).Logger()
	c.log = &l
}
func (c *Interceptor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.echo.ServeHTTP(w, r)
}
