package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"wx_channel/internal/interceptor"
	"wx_channel/internal/interceptor/proxy"
	"wx_channel/pkg/certificate"
)

// WebSocket 代理转发示例
// 完全模拟真实环境下的插件加载逻辑

const (
	sourceHost = "kf.qq.com"
	webHost    = "channels.weixin.qq.com"
	targetHost = "127.0.0.1"
	targetPort = 2022
	proxyPort  = 2024
	webPort    = 8080
)

func main() {
	// 1. 启动静态文件服务 (用于模拟前端页面)
	go func() {
		fs := http.FileServer(http.Dir("_example"))
		http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Printf("[Static] Request: %s %s\n", r.Method, r.URL.Path)
			fs.ServeHTTP(w, r)
		}))
		fmt.Printf("静态服务已启动，请访问: http://127.0.0.1:%d/ws_proxy.html\n", webPort)
		if err := http.ListenAndServe(fmt.Sprintf(":%d", webPort), nil); err != nil {
			fmt.Printf("静态服务启动失败: %v\n", err)
		}
	}()

	// 2. 初始化 Proxy
	// 使用默认证书 (真实环境中也是如此)
	cert := certificate.DefaultCertFiles
	p, err := proxy.NewProxy(cert.Cert, cert.PrivateKey)
	if err != nil {
		panic(fmt.Errorf("创建代理失败: %v", err))
	}

	// 3. 构造 Interceptor 上下文
	// CreateSimpleChannelInterceptorPlugin 需要这些上下文信息
	interceptorCfg := &interceptor.InterceptorConfig{
		Version:                       "dev-test",
		DebugShowError:                true,
		ChannelsDisableLocationToHome: true, // 模拟真实配置
	}
	ic := &interceptor.Interceptor{
		Version:           interceptorCfg.Version,
		Settings:          interceptorCfg,
		FrontendVariables: make(map[string]any),
	}

	// 4. 添加转发规则 (解决 DNS 解析失败问题)
	// 这是为了让 remoteapi.weixin.qq.com 能够被正确转发到目标 WebSocket 服务
	fmt.Printf("添加转发规则: wss://%s -> ws://%s:%d\n", sourceHost, targetHost, targetPort)
	p.AddPlugin(&proxy.Plugin{
		Match: sourceHost,
		Target: &proxy.TargetConfig{
			Protocol: "ws",
			Host:     targetHost,
			Port:     targetPort,
		},
	})

	// 5. 添加转发规则 (静态资源)
	// 这是为了方便本地测试，将 channels.qq.com 映射到本地静态服务
	fmt.Printf("添加转发规则: https://%s -> http://127.0.0.1:%d\n", webHost, webPort)
	p.AddPlugin(&proxy.Plugin{
		Match: webHost,
		Target: &proxy.TargetConfig{
			Protocol: "http",
			Host:     "127.0.0.1",
			Port:     webPort,
		},
	})

	// 6. 添加核心业务插件
	// 这完全模拟了 interceptor.Start() 中的逻辑：client.AddPlugin(CreateSimpleChannelInterceptorPlugin(c, Assets))
	fmt.Println("加载核心插件: SimpleChannelInterceptorPlugin")
	corePlugin := interceptor.CreateSimpleChannelInterceptorPlugin(ic, interceptor.Assets)
	p.AddPlugin(corePlugin)

	// 7. 启动代理服务
	go func() {
		fmt.Printf("WebSocket 代理服务正在启动，监听端口: %d\n", proxyPort)
		// 使用 http.ListenAndServe 启动服务，将请求转发给 proxy
		if err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", proxyPort), http.HandlerFunc(p.ServeHTTP)); err != nil {
			fmt.Printf("启动代理服务失败: %v\n", err)
			os.Exit(1)
		}
	}()

	// 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\n正在关闭代理服务...")
	// 注意：这里没有调用 srv.Stop() 因为我们直接使用了 proxy 对象，
	// 如果 proxy 有 Stop 方法可以调用，否则直接退出即可
}
