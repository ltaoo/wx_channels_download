package interceptor

import (
	"fmt"
	"io"
	"strconv"

	"wx_channel/internal/buildtags"
	"wx_channel/internal/config"
	"wx_channel/internal/events"
	"wx_channel/internal/manager"
	"wx_channel/pkg/certificate"
)

type InterceptorServer struct {
	*manager.HTTPServer
	Interceptor *Interceptor
	cfg         *config.Config
	bus         *events.Bus
}

func NewInterceptorServer(cfg *config.Config, cert *certificate.CertFileAndKeyFile) *InterceptorServer {
	settings := NewInterceptorSettings(cfg)
	interceptor := NewInterceptor(settings, cert)
	addr := settings.ProxyServerHostname + ":" + strconv.Itoa(settings.ProxyServerPort)
	srv := manager.NewHTTPServer("代理服务", addr)
	if buildtags.UsingSunnyNet {
		srv.Disable()
	}
	srv.SetHandler(interceptor)

	return &InterceptorServer{
		HTTPServer:  srv,
		Interceptor: interceptor,
		cfg:         cfg,
	}
}

func (s *InterceptorServer) SubscribeEvents(bus *events.Bus) {
	s.bus = bus
	bus.Subscribe(events.TypeProxyCommand, func(e events.Event) {
		cmd, ok := e.(events.ProxyCommand)
		if !ok {
			return
		}
		switch cmd.Action {
		case events.ProxyStart:
			_ = s.Start()
		case events.ProxyStop:
			_ = s.Stop()
		case events.ProxyRestart:
			if status := s.Status(); status == manager.StatusRunning || status == manager.StatusStopping {
				_ = s.Stop()
			}
			s.applySettingsFromConfig()
			_ = s.Start()
		case events.ProxyApplySettings:
			if s.Status() != manager.StatusRunning {
				s.applySettingsFromConfig()
			}
		}
	})
	bus.Subscribe(events.TypeServiceCommand, func(e events.Event) {
		cmd, ok := e.(events.ServiceCommand)
		if !ok || cmd.Name != "interceptor" {
			return
		}
		switch cmd.Action {
		case "start":
			_ = s.Start()
		case "stop":
			_ = s.Stop()
		}
	})
}

func (s *InterceptorServer) applySettingsFromConfig() {
	if s.cfg == nil {
		return
	}
	s.ApplySettings(NewInterceptorSettings(s.cfg), config.LoadCertFiles())
}

func (s *InterceptorServer) ApplySettings(settings *InterceptorConfig, cert *certificate.CertFileAndKeyFile) {
	s.Interceptor.Settings = settings
	s.Interceptor.Cert = cert
	s.HTTPServer.SetAddr(settings.ProxyServerHostname + ":" + strconv.Itoa(settings.ProxyServerPort))
	s.HTTPServer.SetHandler(s.Interceptor)
}

func (s *InterceptorServer) SetLog(writer io.Writer) {
	s.Interceptor.SetLog(writer)
}

func (s *InterceptorServer) ProxyTun() bool {
	return s.Interceptor.Settings.ProxyTun
}

func (s *InterceptorServer) ProxySetSystem() bool {
	return s.Interceptor.Settings.ProxySetSystem
}

func (s *InterceptorServer) Start() error {
	if err := s.Interceptor.Start(); err != nil {
		return fmt.Errorf("failed to start interceptor: %v", err)
	}
	if err := s.HTTPServer.Start(); err != nil {
		return err
	}
	s.publishStatus()
	return nil
}

func (s *InterceptorServer) Stop() error {
	if err := s.Interceptor.Stop(); err != nil {
		return fmt.Errorf("failed to stop interceptor: %v", err)
	}
	if err := s.HTTPServer.Stop(); err != nil {
		return err
	}
	s.publishStatus()
	return nil
}

func (s *InterceptorServer) publishStatus() {
	if s.bus == nil {
		return
	}
	status := string(s.Status())
	addr := s.Addr()
	s.bus.Publish(events.ProxyStatusChanged{
		Status: status,
		Addr:   addr,
	})
	s.bus.Publish(events.ServiceStatusChanged{
		Name:   "interceptor",
		Title:  "代理服务",
		Addr:   addr,
		Status: status,
	})
}
