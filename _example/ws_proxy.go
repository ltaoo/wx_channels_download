package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"wx_channel/internal/interceptor"
	"wx_channel/internal/interceptor/proxy"
	"wx_channel/pkg/certificate"
)

// WebSocket 代理转发示例
// 完全模拟真实环境下的插件加载逻辑

const (
	webHost   = "channels.weixin.qq.com"
	proxyPort = 2024
	webPort   = 8081
)

// ProxyConfig 代理转发配置
type ProxyConfig struct {
	MatchHost      string `json:"matchHost"`      // 拦截的 fake 域名
	TargetProtocol string `json:"targetProtocol"` // 源服务协议 (ws/wss)
	TargetHost     string `json:"targetHost"`     // 源服务地址
	TargetPort     int    `json:"targetPort"`     // 源服务端口
}

var (
	currentConfig = ProxyConfig{
		MatchHost:      "remoteapi.weixin.qq.com",
		TargetProtocol: "ws",
		TargetHost:     "127.0.0.1",
		TargetPort:     2022,
	}
	currentProxy proxy.InnerProxy
	proxyMu      sync.RWMutex
)

func buildProxy(cfg ProxyConfig) (proxy.InnerProxy, error) {
	cert := certificate.DefaultCertFiles
	p, err := proxy.NewProxy(cert.Cert, cert.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("创建代理失败: %v", err)
	}

	// 添加转发规则: fake 域名 -> 源服务
	if cfg.MatchHost != "" && cfg.TargetHost != "" && cfg.TargetPort != 0 {
		fmt.Printf("添加转发规则: %s -> %s://%s:%d\n", cfg.MatchHost, cfg.TargetProtocol, cfg.TargetHost, cfg.TargetPort)
		p.AddPlugin(&proxy.Plugin{
			Match: cfg.MatchHost,
			Target: &proxy.TargetConfig{
				Protocol: cfg.TargetProtocol,
				Host:     cfg.TargetHost,
				Port:     cfg.TargetPort,
			},
		})
	}

	// 添加静态资源转发规则
	fmt.Printf("添加转发规则: https://%s -> http://127.0.0.1:%d\n", webHost, webPort)
	p.AddPlugin(&proxy.Plugin{
		Match: webHost,
		Target: &proxy.TargetConfig{
			Protocol: "http",
			Host:     "127.0.0.1",
			Port:     webPort,
		},
	})

	// 添加核心业务插件
	interceptorCfg := &interceptor.InterceptorConfig{
		Version:                       "dev-test",
		DebugShowError:                true,
		ChannelsDisableLocationToHome: true,
	}
	ic := &interceptor.Interceptor{
		Version:           interceptorCfg.Version,
		Settings:          interceptorCfg,
		FrontendVariables: make(map[string]any),
	}
	fmt.Println("加载核心插件: SimpleChannelInterceptorPlugin")
	p.AddPlugin(interceptor.CreateSimpleChannelInterceptorPlugin(ic))

	return p, nil
}

func main() {
	// 1. 初始化代理
	p, err := buildProxy(currentConfig)
	if err != nil {
		panic(err)
	}
	proxyMu.Lock()
	currentProxy = p
	proxyMu.Unlock()

	// 2. 启动静态文件服务 + 配置 API
	mux := http.NewServeMux()

	// 静态资源 - lib JS/CSS 文件
	if libFS, err := interceptor.LibFS(); err == nil {
		mux.Handle("/__wx_channels_assets/lib/", http.StripPrefix("/__wx_channels_assets/lib/", http.FileServer(http.FS(libFS))))
	}
	// 静态资源 - src JS 文件
	if srcFS, err := interceptor.SrcFS(); err == nil {
		mux.Handle("/__wx_channels_assets/src/", http.StripPrefix("/__wx_channels_assets/src/", http.FileServer(http.FS(srcFS))))
	}

	fileServer := http.FileServer(http.Dir("_example"))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("[Static] Request: %s %s\n", r.Method, r.URL.Path)
		fileServer.ServeHTTP(w, r)
	}))

	// GET /api/config - 获取当前配置
	// POST /api/config - 更新配置并重建代理
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			proxyMu.RLock()
			cfg := currentConfig
			proxyMu.RUnlock()
			json.NewEncoder(w).Encode(cfg)
			return
		}
		if r.Method == http.MethodPost {
			var cfg ProxyConfig
			if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
				http.Error(w, fmt.Sprintf("无效的配置: %v", err), http.StatusBadRequest)
				return
			}
			if cfg.MatchHost == "" || cfg.TargetHost == "" || cfg.TargetPort == 0 {
				http.Error(w, "matchHost, targetHost, targetPort 不能为空", http.StatusBadRequest)
				return
			}
			if cfg.TargetProtocol == "" {
				cfg.TargetProtocol = "ws"
			}

			newProxy, err := buildProxy(cfg)
			if err != nil {
				http.Error(w, fmt.Sprintf("重建代理失败: %v", err), http.StatusInternalServerError)
				return
			}

			proxyMu.Lock()
			currentConfig = cfg
			currentProxy = newProxy
			proxyMu.Unlock()

			fmt.Printf("配置已更新: %s -> %s://%s:%d\n", cfg.MatchHost, cfg.TargetProtocol, cfg.TargetHost, cfg.TargetPort)
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "config": cfg})
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	go func() {
		fmt.Printf("静态服务已启动，请访问: http://127.0.0.1:%d/ws_proxy.html\n", webPort)
		if err := http.ListenAndServe(fmt.Sprintf(":%d", webPort), mux); err != nil {
			fmt.Printf("静态服务启动失败: %v\n", err)
		}
	}()

	// 3. 启动代理服务 (handler 动态读取 currentProxy)
	go func() {
		fmt.Printf("WebSocket 代理服务正在启动，监听端口: %d\n", proxyPort)
		if err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", proxyPort), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			proxyMu.RLock()
			p := currentProxy
			proxyMu.RUnlock()
			p.ServeHTTP(w, r)
		})); err != nil {
			fmt.Printf("启动代理服务失败: %v\n", err)
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\n正在关闭代理服务...")
}
