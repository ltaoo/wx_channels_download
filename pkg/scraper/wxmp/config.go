package wxmp

type OfficialAccountConfig struct {
	RootDir                   string   `json:"rootDir"`
	WorkDir                   string   `json:"workDir"`
	Enabled                   bool     `json:"officialAccountEnabled"`
	DebugShowError            bool     `json:"debugShowError"`
	Protocol                  string   `json:"protocol"`
	Hostname                  string   `json:"hostname"`
	Port                      int      `json:"port"`
	Addr                      string   `json:"addr"`
	RemoteServerEnabled       bool     `json:"remoteServerEnabled"`
	RemoteServerProtocol      string   `json:"remoteServerProtocol"`
	RemoteServerHostname      string   `json:"remoteServerHostname"`
	RemoteServerPort          int      `json:"remoteServerPort"`
	RefreshToken              string   `json:"officialServerRefreshToken"`
	TokenFilepath             string   `json:"tokenFilepath"`
	RefreshSkipMinutes        int      `json:"refreshSkipMinutes"`
	MaxWebsocketClients       int      `json:"maxWebsocketClients"`
	AccountIdsRefreshInterval []string `json:"accountIdsRefreshInterval"`
	GlobalScriptPath          string   `json:"-"`
	GlobalScriptURL           string   `json:"-"`
	InjectContentScript       string   `json:"injectContentScript"`
}
