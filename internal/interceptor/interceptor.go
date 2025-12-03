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

type ChannelInjectedFiles struct {
	JSFileSaver []byte
	JSZip       []byte
	JSRecorder  []byte
	JSPageSpy   []byte
	JSDebug     []byte
	JSUtils     []byte
	JSError     []byte
	JSMain      []byte
	JSLiveMain  []byte
}

type ChannelMediaSpec struct {
	FileFormat       string  `json:"file_format"`
	FirstLoadBytes   int     `json:"first_load_bytes"`
	BitRate          int     `json:"bit_rate"`
	CodingFormat     string  `json:"coding_format"`
	DynamicRangeType int     `json:"dynamic_range_type"`
	Vfps             int     `json:"vfps"`
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	DurationMs       int     `json:"duration_ms"`
	QualityScore     float64 `json:"quality_score"`
	VideoBitrate     int     `json:"video_bitrate"`
	AudioBitrate     int     `json:"audio_bitrate"`
	LevelOrder       int     `json:"level_order"`
	Bypass           string  `json:"bypass"`
	Is3az            int     `json:"is3az"`
}
type ChannelContact struct {
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	HeadURL  string `json:"head_url"`
}
type ChannelMediaProfile struct {
	Title    string             `json:"title"`
	CoverURL string             `json:"cover_url"`
	URL      string             `json:"url"`
	Size     int                `json:"size"`
	Key      string             `json:"key"`
	NonceId  string             `json:"nonce_id"`
	Nickname string             `json:"nickname"`
	Type     string             `json:"type"`
	Contact  ChannelContact     `json:"contact"`
	Spec     []ChannelMediaSpec `json:"spec"`
}
type FrontendTip struct {
	End          int     `json:"end"`
	Replace      int     `json:"replace"`
	IgnorePrefix int     `json:"ignore_prefix"`
	Prefix       *string `json:"prefix"`
	Msg          string  `json:"msg"`
}

type ServerCertFiles struct {
	CertFile       []byte
	PrivateKeyFile []byte
}
type InterceptorConfig struct {
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
	client, err := echo.NewEcho(payload.CertFiles.CertFile, payload.CertFiles.PrivateKeyFile)
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
		CertFile:       payload.CertFiles.CertFile,
		PrivateKeyFile: payload.CertFiles.PrivateKeyFile,
		CertFileName:   payload.CertFileName,
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
