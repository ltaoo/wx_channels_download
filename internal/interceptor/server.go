package interceptor

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"wx_channel/internal/buildtags"
	"wx_channel/internal/config"
	"wx_channel/internal/events"
	"wx_channel/internal/services"
	"wx_channel/pkg/certificate"
)

type InterceptorServer struct {
	Interceptor *Interceptor
	cfg         *config.Config
	bus         *events.Bus
	addr        string
	server      *http.Server
	running     bool
}

func NewInterceptorServer(cfg *config.Config, cert *certificate.CertFileAndKeyFile) *InterceptorServer {
	settings := NewInterceptorSettings(cfg)
	interceptor := NewInterceptor(settings, cert)

	return &InterceptorServer{
		addr:        settings.ProxyServerHostname + ":" + strconv.Itoa(settings.ProxyServerPort),
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
			if s.running {
				_ = s.Stop()
			}
			s.applySettingsFromConfig()
			_ = s.Start()
		case events.ProxyApplySettings:
			if !s.running {
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
	s.ApplySettings(NewInterceptorSettings(s.cfg), services.LoadCertFiles())
}

func (s *InterceptorServer) ApplySettings(settings *InterceptorConfig, cert *certificate.CertFileAndKeyFile) {
	s.Interceptor.Settings = settings
	s.Interceptor.Cert = cert
	s.addr = settings.ProxyServerHostname + ":" + strconv.Itoa(settings.ProxyServerPort)
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

func (s *InterceptorServer) Addr() string {
	return s.addr
}

func (s *InterceptorServer) Start() error {
	var listener net.Listener
	if !buildtags.UsingSunnyNet {
		l, err := net.Listen("tcp", s.addr)
		if err != nil {
			return err
		}
		listener = l
	}
	if err := s.Interceptor.Start(); err != nil {
		if listener != nil {
			_ = listener.Close()
		}
		return fmt.Errorf("failed to start interceptor: %v", err)
	}
	if listener != nil {
		server := &http.Server{
			Addr:    s.addr,
			Handler: s.Interceptor,
		}
		s.server = server
		go func() {
			if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
				fmt.Printf("代理服务 error: %v\n", err)
				return
			}
			fmt.Println("代理服务 stopped")
		}()
	}
	s.running = true
	s.publishStatus("running")
	return nil
}

func (s *InterceptorServer) Stop() error {
	var shutdownErr error
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownErr = s.server.Shutdown(ctx)
		s.server = nil
	}
	if err := s.Interceptor.Stop(); err != nil {
		return fmt.Errorf("failed to stop interceptor: %v", err)
	}
	s.running = false
	s.publishStatus("stopped")
	if shutdownErr != nil {
		return shutdownErr
	}
	return nil
}

func (s *InterceptorServer) publishStatus(status string) {
	if s.bus == nil {
		return
	}
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
