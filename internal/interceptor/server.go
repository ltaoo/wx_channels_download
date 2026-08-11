package interceptor

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog"

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
	lifecycleMu sync.Mutex
}

func NewInterceptorServer(cfg *config.Config, cert *certificate.CertFileAndKeyFile, logger *zerolog.Logger) *InterceptorServer {
	settings := NewInterceptorSettings(cfg)
	interceptor := NewInterceptor(settings, cert, logger)

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

func (s *InterceptorServer) ProxyTun() bool {
	return s.Interceptor.Settings.ProxyTun
}

func (s *InterceptorServer) ProxySetSystem() bool {
	return s.Interceptor.Settings.ProxySetSystem
}

// ProxyDevice returns the network service name the system proxy is written to, or an empty
// string when it should be detected automatically.
func (s *InterceptorServer) ProxyDevice() string {
	return s.Interceptor.Settings.ProxyDevice
}

// SetProxyDevice pins the network service used for the system proxy. Callers resolve it once
// before starting so that enabling and disabling cannot end up on different services when the
// primary service changes while the application runs.
func (s *InterceptorServer) SetProxyDevice(device string) {
	s.Interceptor.Settings.ProxyDevice = device
}

func (s *InterceptorServer) Addr() string {
	return s.addr
}

func (s *InterceptorServer) Start() error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()

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
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()

	// Disable the system proxy before waiting for active HTTP connections.
	// Console-close cleanup on Windows may be forcibly terminated after a
	// short timeout, and leaving the proxy enabled breaks the user's network.
	interceptorErr := s.Interceptor.Stop()

	var shutdownErr error
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownErr = s.server.Shutdown(ctx)
		s.server = nil
	}
	s.running = false
	s.publishStatus("stopped")
	if interceptorErr != nil {
		return fmt.Errorf("failed to stop interceptor: %v", interceptorErr)
	}
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
