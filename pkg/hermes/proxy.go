package hermes

import (
	"errors"
	"net/url"
	"strings"
)

const (
	// TaskProxyServerConfigKey is the preferred task Config key for a structured
	// proxy server definition.
	TaskProxyServerConfigKey = "proxy_server"
	legacyTaskProxyConfigKey = "proxy"
)

// ProxyServer describes a task-level forward proxy. Address accepts a complete
// http, https, or socks5 URL. A host:port value is treated as an HTTP proxy.
// Username and Password are kept separate so callers do not need to embed
// credentials in Address.
type ProxyServer struct {
	Address  string `json:"address"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// URL returns the proxy URL consumed by protocol clients. Explicit credential
// fields override credentials embedded in Address.
func (p ProxyServer) URL() (string, error) {
	address := strings.TrimSpace(p.Address)
	if address == "" {
		return "", nil
	}
	if !strings.Contains(address, "://") {
		address = "http://" + address
	}

	parsed, err := url.Parse(address)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Hostname() == "" {
		return "", errors.New("invalid proxy server address")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	switch parsed.Scheme {
	case "http", "https", "socks5":
	default:
		return "", errors.New("unsupported proxy server scheme")
	}
	if p.Username != "" || p.Password != "" {
		parsed.User = url.UserPassword(p.Username, p.Password)
	}
	return parsed.String(), nil
}

// applyTaskProxy resolves the task-level proxy once and copies it to every
// endpoint. This keeps protocol drivers stateless with respect to tasks and
// allows one engine instance to run tasks through different proxies.
func applyTaskProxy(task *TaskJob) {
	if task == nil {
		return
	}

	proxyServer := task.ProxyServer
	proxyServer.Address = strings.TrimSpace(proxyServer.Address)
	if proxyServer.Address == "" {
		proxyServer = proxyServerFromConfig(task.Config)
	}
	task.ProxyServer = proxyServer

	for resourceIndex := range task.Resources {
		for endpointIndex := range task.Resources[resourceIndex].Endpoints {
			task.Resources[resourceIndex].Endpoints[endpointIndex].ProxyServer = proxyServer
		}
	}
}

func proxyServerFromConfig(config map[string]any) ProxyServer {
	if config == nil {
		return ProxyServer{}
	}
	if proxy, ok := decodeProxyServer(config[TaskProxyServerConfigKey]); ok {
		return proxy
	}
	// Keep accepting the original string/map key while callers migrate to the
	// structured proxy_server field.
	proxy, _ := decodeProxyServer(config[legacyTaskProxyConfigKey])
	return proxy
}

func decodeProxyServer(value any) (ProxyServer, bool) {
	switch proxy := value.(type) {
	case ProxyServer:
		return proxy, strings.TrimSpace(proxy.Address) != ""
	case *ProxyServer:
		if proxy == nil {
			return ProxyServer{}, false
		}
		return *proxy, strings.TrimSpace(proxy.Address) != ""
	case string:
		proxy = strings.TrimSpace(proxy)
		return ProxyServer{Address: proxy}, proxy != ""
	case map[string]string:
		result := ProxyServer{
			Address:  proxy["address"],
			Username: proxy["username"],
			Password: proxy["password"],
		}
		return result, strings.TrimSpace(result.Address) != ""
	case map[string]any:
		result := ProxyServer{}
		result.Address, _ = proxy["address"].(string)
		result.Username, _ = proxy["username"].(string)
		result.Password, _ = proxy["password"].(string)
		return result, strings.TrimSpace(result.Address) != ""
	default:
		return ProxyServer{}, false
	}
}

// taskConfigForLog removes the entire proxy definition from structured logs so
// usernames, passwords, or credentials embedded in an address cannot leak.
func taskConfigForLog(config map[string]any) map[string]any {
	if config == nil {
		return nil
	}
	redacted := make(map[string]any, len(config))
	for key, value := range config {
		switch key {
		case TaskProxyServerConfigKey, legacyTaskProxyConfigKey:
			redacted[key] = "<redacted>"
		default:
			redacted[key] = value
		}
	}
	return redacted
}
