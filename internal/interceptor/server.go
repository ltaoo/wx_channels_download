package interceptor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"wx_channel/internal/buildtags"
	"wx_channel/pkg/certificate"
	"wx_channel/pkg/configapi"
	"wx_channel/pkg/events"
)

type ServerDeps struct {
	ConfigProvider    configapi.Provider
	Runtime           configapi.Runtime
	CertificateLoader func() *certificate.CertFileAndKeyFile
}

type InterceptorServer struct {
	Interceptor        *Interceptor
	config_provider    configapi.Provider
	runtime            configapi.Runtime
	certificate_loader func() *certificate.CertFileAndKeyFile
	config_unsubscribe func()
	bus_unsubscribe    func()
	bus                *events.Bus
	addr               string
	server             *http.Server
	running            bool
	disabled           bool
	mu                 sync.RWMutex
}

func NewInterceptorServer(deps ServerDeps) (*InterceptorServer, error) {
	if deps.CertificateLoader == nil {
		return nil, errors.New("interceptor certificate loader is required")
	}
	settings, err := NewInterceptorSettings(deps.ConfigProvider, deps.Runtime)
	if err != nil {
		return nil, err
	}
	cert := deps.CertificateLoader()
	if cert == nil {
		return nil, errors.New("interceptor certificate is required")
	}
	interceptor := NewInterceptor(settings, cert)
	addr := settings.ProxyServerHostname + ":" + strconv.Itoa(settings.ProxyServerPort)

	server := &InterceptorServer{
		Interceptor:        interceptor,
		config_provider:    deps.ConfigProvider,
		runtime:            deps.Runtime,
		certificate_loader: deps.CertificateLoader,
		addr:               addr,
		disabled:           buildtags.UsingSunnyNet,
	}
	unsubscribe, err := ConfigDeclaration.Subscribe(deps.ConfigProvider, func(uint64) {
		_ = server.applySettingsFromConfig()
	})
	if err != nil {
		return nil, err
	}
	server.config_unsubscribe = unsubscribe
	return server, nil
}

func (s *InterceptorServer) Addr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.addr
}

func (s *InterceptorServer) SubscribeEvents(bus *events.Bus) {
	if bus == nil {
		return
	}
	s.mu.Lock()
	if s.bus_unsubscribe != nil {
		s.bus_unsubscribe()
	}
	s.bus = bus
	s.bus_unsubscribe = bus.Subscribe(events.TypeProxyCommand, func(e events.Event) {
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
			if s.isRunning() {
				_ = s.Stop()
			}
			_ = s.applySettingsFromConfig()
			_ = s.Start()
		case events.ProxyApplySettings:
			if !s.isRunning() {
				_ = s.applySettingsFromConfig()
			}
		}
	})
	s.mu.Unlock()
}

func (s *InterceptorServer) applySettingsFromConfig() error {
	settings, err := NewInterceptorSettings(s.config_provider, s.runtime)
	if err != nil {
		return err
	}
	cert := s.certificate_loader()
	if cert == nil {
		return errors.New("interceptor certificate is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return nil
	}
	s.apply_settings(settings, cert)
	return nil
}

func (s *InterceptorServer) ApplySettings(settings *InterceptorConfig, cert *certificate.CertFileAndKeyFile) {
	if settings == nil || cert == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return
	}
	s.apply_settings(settings, cert)
}

func (s *InterceptorServer) apply_settings(settings *InterceptorConfig, cert *certificate.CertFileAndKeyFile) {
	s.Interceptor.Settings = settings
	s.Interceptor.Cert = cert
	s.addr = settings.ProxyServerHostname + ":" + strconv.Itoa(settings.ProxyServerPort)
}

func (s *InterceptorServer) SetLogger(logger *zerolog.Logger) {
	s.Interceptor.SetLogger(logger)
}

func (s *InterceptorServer) ProxyTun() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Interceptor.Settings.ProxyTun
}

func (s *InterceptorServer) ProxySetSystem() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Interceptor.Settings.ProxySetSystem
}

func (s *InterceptorServer) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("proxy service is already running")
	}
	addr := s.addr
	disabled := s.disabled

	if err := s.Interceptor.Start(); err != nil {
		s.mu.Unlock()
		s.publishStatus("error", addr)
		return fmt.Errorf("failed to start interceptor: %v", err)
	}

	if disabled {
		s.running = true
		s.mu.Unlock()
		s.publishStatus("running", addr)
		return nil
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		_ = s.Interceptor.Stop()
		s.mu.Unlock()
		s.publishStatus("error", addr)
		return err
	}
	server := &http.Server{Addr: addr, Handler: s.Interceptor}
	s.server = server
	s.running = true
	s.mu.Unlock()

	go func() {
		err := server.Serve(listener)
		s.mu.Lock()
		current := s.server == server
		if current {
			s.server = nil
			s.running = false
		}
		s.mu.Unlock()
		if current {
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				fmt.Printf("代理服务 error: %v\n", err)
				s.publishStatus("error", addr)
			} else {
				fmt.Printf("代理服务 stopped\n")
				s.publishStatus("stopped", addr)
			}
		}
	}()
	s.publishStatus("running", addr)
	return nil
}

func (s *InterceptorServer) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	addr := s.addr
	server := s.server

	var stopErr error
	if err := s.Interceptor.Stop(); err != nil {
		stopErr = fmt.Errorf("failed to stop interceptor: %v", err)
	}
	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := server.Shutdown(ctx); err != nil {
			stopErr = errors.Join(stopErr, err)
		}
		cancel()
	}
	s.server = nil
	s.running = false
	s.mu.Unlock()

	if stopErr != nil {
		s.publishStatus("error", addr)
		return stopErr
	}
	s.publishStatus("stopped", addr)
	return nil
}

// Close permanently stops the server and releases its configuration and event
// subscriptions. Stop remains restart-safe for runtime proxy commands.
func (s *InterceptorServer) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	config_unsubscribe := s.config_unsubscribe
	bus_unsubscribe := s.bus_unsubscribe
	s.config_unsubscribe = nil
	s.bus_unsubscribe = nil
	s.mu.Unlock()
	if config_unsubscribe != nil {
		config_unsubscribe()
	}
	if bus_unsubscribe != nil {
		bus_unsubscribe()
	}
	return s.Stop()
}

func (s *InterceptorServer) isRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *InterceptorServer) publishStatus(status, addr string) {
	if s.bus == nil {
		return
	}
	s.bus.Publish(events.ProxyStatusChanged{
		Status: status,
		Addr:   addr,
	})
}
