package hermes

import (
	"errors"
	"net/url"
	"strings"
)

const (
	// TaskProxyServerConfigKey is the preferred task Config key for a structured
	// proxy server definition.
	TaskProxyServerConfigKey     = "proxy_server"
	legacy_task_proxy_config_key = "proxy"
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
func apply_task_proxy(task *TaskJob) {
	if task == nil {
		return
	}

	proxy_server := task.ProxyServer
	proxy_server.Address = strings.TrimSpace(proxy_server.Address)
	if proxy_server.Address == "" {
		proxy_server = proxy_server_from_config(task.Config)
	}
	task.ProxyServer = proxy_server

	for resource_index := range task.Resources {
		for endpoint_index := range task.Resources[resource_index].Endpoints {
			task.Resources[resource_index].Endpoints[endpoint_index].ProxyServer = proxy_server
		}
	}
}

func proxy_server_from_config(config map[string]any) ProxyServer {
	if config == nil {
		return ProxyServer{}
	}
	if proxy, ok := decode_proxy_server(config[TaskProxyServerConfigKey]); ok {
		return proxy
	}
	// Keep accepting the original string/map key while callers migrate to the
	// structured proxy_server field.
	proxy, _ := decode_proxy_server(config[legacy_task_proxy_config_key])
	return proxy
}

func decode_proxy_server(value any) (ProxyServer, bool) {
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
func task_config_for_log(config map[string]any) map[string]any {
	if config == nil {
		return nil
	}
	redacted := make(map[string]any, len(config))
	for key, value := range config {
		switch key {
		case TaskProxyServerConfigKey, legacy_task_proxy_config_key:
			redacted[key] = "<redacted>"
		default:
			redacted[key] = value
		}
	}
	return redacted
}
