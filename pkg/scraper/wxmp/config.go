package wxmp

type OfficialAccountConfig struct {
	RootDir                   string
	WorkDir                   string
	Enabled                   bool `json:"officialAccountEnabled"`
	DebugShowError            bool
	Protocol                  string
	Hostname                  string
	Port                      int
	Addr                      string
	RemoteServerEnabled       bool   `json:"remoteServerEnabled"`
	RemoteServerProtocol      string `json:"remoteServerProtocol"`
	RemoteServerHostname      string `json:"remoteServerHostname"`
	RemoteServerPort          int    `json:"remoteServerPort"`
	RefreshToken              string `json:"officialServerRefreshToken"`
	TokenFilepath             string
	RefreshSkipMinutes        int
	MaxWebsocketClients       int
	AccountIdsRefreshInterval []string
	GlobalScriptPath          string `json:"-"`
	GlobalScriptURL           string `json:"-"`
	InjectContentScript       string
}
