package api

type api_endpoint_config struct {
	Protocol string `json:"protocol"`
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
}

type proxy_runtime_config struct {
	System              bool   `json:"system"`
	Tun                 bool   `json:"tun"`
	Hostname            string `json:"hostname"`
	Port                int    `json:"port"`
	DefaultInterface    string `json:"defaultInterface"`
	SkipInstallRootCert bool   `json:"skipInstallRootCert"`
	UpstreamProxy       string `json:"upstreamProxy"`
	TCPRelay            struct {
		Enabled  bool   `json:"enabled"`
		Hostname string `json:"hostname"`
		Port     int    `json:"port"`
	} `json:"tcpRelay"`
}

type certificate_runtime_config struct {
	Name string `json:"name"`
	File string `json:"file"`
	Key  string `json:"key"`
}

func (c *APIClient) current_api_endpoint_config() api_endpoint_config {
	cfg := api_endpoint_config{Protocol: "http", Hostname: "127.0.0.1", Port: 2022}
	if c == nil || c.config_store == nil {
		return cfg
	}
	if err := ConfigDeclaration.Decode(c.config_store, "api", &cfg); err != nil {
		return cfg
	}
	if cfg.Hostname == "" {
		cfg.Hostname = "127.0.0.1"
	}
	if cfg.Port <= 0 {
		cfg.Port = 2022
	}
	return cfg
}

func (c *APIClient) current_proxy_config() proxy_runtime_config {
	cfg := proxy_runtime_config{Hostname: "127.0.0.1", Port: 2023}
	cfg.TCPRelay.Hostname = "127.0.0.1"
	cfg.TCPRelay.Port = 9900
	if c == nil || c.config_store == nil {
		return cfg
	}
	if err := ConfigDeclaration.Decode(c.config_store, "proxy", &cfg); err != nil {
		return cfg
	}
	if cfg.Hostname == "" {
		cfg.Hostname = "127.0.0.1"
	}
	if cfg.Port <= 0 {
		cfg.Port = 2023
	}
	if cfg.TCPRelay.Hostname == "" {
		cfg.TCPRelay.Hostname = "127.0.0.1"
	}
	if cfg.TCPRelay.Port <= 0 {
		cfg.TCPRelay.Port = 9900
	}
	return cfg
}

func (c *APIClient) current_certificate_config() certificate_runtime_config {
	var cfg certificate_runtime_config
	if c == nil || c.config_store == nil {
		return cfg
	}
	_ = ConfigDeclaration.Decode(c.config_store, "cert", &cfg)
	return cfg
}
