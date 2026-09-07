package interceptor

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"runtime"
	"strconv"

	"github.com/ltaoo/echo"
	"github.com/rs/zerolog"

	"wx_channel/internal/buildtags"
	"wx_channel/internal/interceptor/proxy"
	"wx_channel/pkg/certificate"
	"wx_channel/pkg/platform"
	"wx_channel/pkg/system"
)

type Interceptor struct {
	Version     string
	Debug       bool
	Settings    *InterceptorConfig
	Headers     map[string]string
	Cert        *certificate.CertFileAndKeyFile
	proxy       proxy.InnerProxy
	PostPlugins []interface{} // echo plugins, passed to echo after initialization
	log         *zerolog.Logger
	OnCookies   func(url string, cookies []*http.Cookie) // Callback invoked when cookies are captured
}

func NewInterceptor(cfg *InterceptorConfig, cert *certificate.CertFileAndKeyFile, logger *zerolog.Logger) *Interceptor {
	log := new_interceptor_logger(logger, cfg.Version)
	return &Interceptor{
		Version:  cfg.Version,
		Debug:    cfg.DebugShowError,
		Settings: cfg,
		Cert:     cert,
		log:      log,
		proxy:    nil,
	}
}

func new_interceptor_logger(parent *zerolog.Logger, version string) *zerolog.Logger {
	if parent == nil {
		l := zerolog.New(io.Discard).With().Timestamp().Str("component", "interceptor").Str("version", version).Logger()
		return &l
	}
	l := parent.With().Str("component", "interceptor").Str("version", version).Logger()
	return &l
}

func (c *Interceptor) Start() error {
	echo.SetLogEnabled(c.Settings.EchoLogEnabled)
	client, err := proxy.NewProxy(c.Cert.Cert, c.Cert.PrivateKey, c.Settings.ProxyUpstreamProxy, c.Settings.ProxyTun, c.Settings.ProxyServerHostname, c.Settings.ProxyServerPort, c.Settings.ProxyDefaultInterface, &proxy.TCPRelayConfig{
		Enabled:  c.Settings.ProxyTCPRelayEnabled,
		Hostname: c.Settings.ProxyTCPRelayHostname,
		Port:     c.Settings.ProxyTCPRelayPort,
	})
	if err != nil {
		return err
	}
	if len(c.PostPlugins) != 0 {
		for _, plugin := range c.PostPlugins {
			client.AddPlugin(plugin)
		}
	}
	c.proxy = client
	if !c.Settings.ProxySkipInstallRootCert {
		existing, err := certificate.CheckHasCertificate(c.Cert.Name)
		if err != nil {
			return fmt.Errorf("failed to check certificate: %v", err)
		}
		if !existing {
			// macOS and Linux elevate only the certificate installation command.
			// Windows still uses the application-level elevation flow.
			if runtime.GOOS == "windows" && !platform.IsAdmin() {
				if !platform.RequestAdminPermission() {
					return fmt.Errorf("failed to elevate privileges for certificate installation; please run as administrator")
				}
				// The elevated process inherits the current arguments and will
				// restart the proxy setup, including certificate installation.
				os.Exit(0)
			}
			fmt.Printf("Installing certificate...\n")
			if err := certificate.InstallCertificate(c.Cert.Cert); err != nil {
				return fmt.Errorf("failed to install certificate: %v", err)
			}
			installed, verify_err := certificate.CheckHasCertificate(c.Cert.Name)
			if verify_err != nil {
				return fmt.Errorf("failed to verify certificate installation: %v", verify_err)
			}
			if !installed {
				return fmt.Errorf("certificate not found in the system keychain after installation: %v", c.Cert.Name)
			}
		}
	}
	if !buildtags.UsingSunnyNet && c.Settings.ProxySetSystem && !c.Settings.ProxyTun {
		if err := system.EnableProxy(system.ProxySettings{
			Device:   c.Settings.ProxyDevice,
			Hostname: c.Settings.ProxyServerHostname,
			Port:     strconv.Itoa(c.Settings.ProxyServerPort),
		}); err != nil {
			return fmt.Errorf("failed to configure proxy: %v", err)
		}
	}
	if err := client.Start(c.Settings.ProxyServerPort); err != nil {
		return err
	}
	return nil
}

func (c *Interceptor) Stop() error {
	if !buildtags.UsingSunnyNet && c.Settings.ProxySetSystem && !c.Settings.ProxyTun {
		arg := system.ProxySettings{
			Device:   c.Settings.ProxyDevice,
			Hostname: c.Settings.ProxyServerHostname,
			Port:     strconv.Itoa(c.Settings.ProxyServerPort),
		}
		_, err := system.DisableProxyIfMatches(arg)
		if err != nil {
			return fmt.Errorf("failed to disable system proxy: %v", err)
		}
	}
	if c.proxy != nil {
		if err := c.proxy.Close(); err != nil {
			return fmt.Errorf("failed to stop proxy service: %v", err)
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

func (c *Interceptor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if h, _, err := net.SplitHostPort(r.Host); err == nil {
		host = h
	}
	is_local := false
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		is_local = true
	}
	if host == "localhost" || host == c.Settings.ProxyServerHostname {
		is_local = true
	}

	is_upgrade := r.Header.Get("Upgrade") != ""
	if is_local || is_upgrade {
		c.log.Info().
			Str("method", r.Method).
			Str("host", r.Host).
			Str("url_scheme", r.URL.Scheme).
			Str("url_host", r.URL.Host).
			Str("path", r.URL.Path).
			Bool("is_local", is_local).
			Bool("is_upgrade", is_upgrade).
			Str("upgrade", r.Header.Get("Upgrade")).
			Str("connection", r.Header.Get("Connection")).
			Msg("interceptor request")
	}

	if is_local && r.URL.Path == "/cert" {
		w.Header().Set("Content-Type", "application/x-x509-ca-cert")
		w.Header().Set("Content-Disposition", "attachment; filename=\"SunnyNet.cer\"")
		w.Write(c.Cert.Cert)
		return
	}
	if is_local && r.Method == http.MethodConnect {
		c.serve_loopback_tunnel(w, r)
		return
	}
	if is_local && (r.URL.Path == "/" || r.URL.Path == "") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<html><head><meta charset="utf-8"><title>wx_channels_download</title><style>@media(prefers-color-scheme:dark){body{background:#3c3c3c;color:#e0e0e0}a{color:#7cb8ff}}</style></head><body><h1>Proxy service is running</h1><p><a href="/cert">Click to download the certificate</a></p></body></html>`)
		return
	}
	// Loopback targets (for example the local API server on 127.0.0.1:2022)
	// must bypass MITM entirely. Routing them through the interceptor breaks
	// plain HTTP and, in particular, WebSocket upgrades, which fail with close
	// code 1006 because the MITM engine does not relay the upgrade for loopback
	// destinations.
	if is_local {
		c.serve_loopback_direct(w, r)
		return
	}
	c.proxy.ServeHTTP(w, r)
}

// loopback_direct_transport dials loopback destinations directly, never through
// an upstream or ambient proxy. It is cloned from the default transport so it
// keeps sane timeouts and connection pooling.
var loopback_direct_transport = func() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = nil
	return t
}()

// serve_loopback_tunnel handles CONNECT requests to loopback targets by
// establishing a raw TCP tunnel, bypassing the MITM engine entirely.
func (c *Interceptor) serve_loopback_tunnel(w http.ResponseWriter, r *http.Request) {
	target_addr := r.URL.Host
	if target_addr == "" {
		target_addr = r.Host
	}
	c.log.Info().Str("target_addr", target_addr).Msg("interceptor: tunnel loopback directly")

	dst, err := net.Dial("tcp", target_addr)
	if err != nil {
		c.log.Error().Err(err).Str("target_addr", target_addr).Msg("interceptor: loopback tunnel dial failed")
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		dst.Close()
		http.Error(w, "hijacking unsupported", http.StatusInternalServerError)
		return
	}
	client_conn, _, err := hijacker.Hijack()
	if err != nil {
		dst.Close()
		return
	}
	if _, err := client_conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		client_conn.Close()
		dst.Close()
		return
	}
	go func() {
		_, _ = io.Copy(dst, client_conn)
		dst.Close()
	}()
	go func() {
		_, _ = io.Copy(client_conn, dst)
		client_conn.Close()
	}()
}

// serve_loopback_direct forwards a request straight to its loopback target,
// supporting both plain HTTP and WebSocket upgrades.
func (c *Interceptor) serve_loopback_direct(w http.ResponseWriter, r *http.Request) {
	scheme := r.URL.Scheme
	switch scheme {
	case "":
		scheme = "http"
	case "ws":
		scheme = "http"
	case "wss":
		scheme = "https"
	}
	target_host := r.URL.Host
	if target_host == "" {
		target_host = r.Host
	}
	target, err := url.Parse(scheme + "://" + target_host)
	if err != nil {
		c.log.Error().Err(err).Str("target_host", target_host).Msg("interceptor: invalid loopback target")
		http.Error(w, "invalid loopback target", http.StatusBadGateway)
		return
	}
	c.log.Info().
		Str("target", target.String()).
		Str("path", r.URL.Path).
		Msg("interceptor: bypass loopback directly")
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
		},
		Transport: loopback_direct_transport,
	}
	proxy.ServeHTTP(w, r)
}
