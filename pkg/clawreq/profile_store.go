package clawreq

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

const chrome_151_version = "151.0.7922.109"

// Chrome 151 is deliberately data-only and is not wired into Config or Client.
// Enabling it requires upgrading the module to Go 1.24.1 and replacing the
// current CycleTLS backend with a modern tls-client version that can represent
// X25519MLKEM768, new ALPS, ECH GREASE, randomized extensions, and Chrome's
// HTTP/2 settings together.
//
//go:embed profiles/151.0.7922.109/profile.json
var chrome_151_profile_json []byte

type stored_browser_profile struct {
	SchemaVersion int                     `json:"schema_version"`
	Status        string                  `json:"status"`
	Browser       stored_browser_identity `json:"browser"`
	TLS           stored_tls_profile      `json:"tls"`
	HTTP2         stored_http2_profile    `json:"http2"`
	Headers       stored_header_profile   `json:"headers"`
}

type stored_browser_identity struct {
	Family          string `json:"family"`
	Version         string `json:"version"`
	MajorVersion    int    `json:"major_version"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platform_version"`
	Architecture    string `json:"architecture"`
	Bitness         string `json:"bitness"`
	UserAgent       string `json:"user_agent"`
	SecCHUA         string `json:"sec_ch_ua"`
	SecCHUAMobile   string `json:"sec_ch_ua_mobile"`
	SecCHUAPlatform string `json:"sec_ch_ua_platform"`
}

type stored_tls_profile struct {
	GreasePlaceholder                int                     `json:"grease_placeholder"`
	RandomExtensionOrder             bool                    `json:"random_extension_order"`
	CipherSuites                     []int                   `json:"cipher_suites"`
	SampleExtensionOrder             []int                   `json:"sample_extension_order"`
	SupportedGroups                  []int                   `json:"supported_groups"`
	KeyShareGroups                   []int                   `json:"key_share_groups"`
	SignatureAlgorithms              []int                   `json:"signature_algorithms"`
	SupportedVersions                []int                   `json:"supported_versions"`
	ALPN                             []string                `json:"alpn"`
	ALPS                             stored_alps_profile     `json:"alps"`
	CertificateCompressionAlgorithms []int                   `json:"certificate_compression_algorithms"`
	ECPointFormats                   []int                   `json:"ec_point_formats"`
	PSKKeyExchangeModes              []int                   `json:"psk_key_exchange_modes"`
	ECHGrease                        bool                    `json:"ech_grease"`
	SessionTicket                    bool                    `json:"session_ticket"`
	Fingerprints                     stored_tls_fingerprints `json:"fingerprints"`
}

type stored_alps_profile struct {
	Codepoint int      `json:"codepoint"`
	Protocols []string `json:"protocols"`
}

type stored_tls_fingerprints struct {
	SampleJA3     string `json:"sample_ja3"`
	SampleJA3Hash string `json:"sample_ja3_hash"`
	JA4           string `json:"ja4"`
	JA4R          string `json:"ja4_r"`
	Peetprint     string `json:"peetprint"`
	PeetprintHash string `json:"peetprint_hash"`
}

type stored_http2_profile struct {
	Settings                []stored_http2_setting       `json:"settings"`
	ConnectionFlowIncrement int                          `json:"connection_flow_increment"`
	PseudoHeaderOrder       []string                     `json:"pseudo_header_order"`
	HeaderPriority          stored_http2_header_priority `json:"header_priority"`
	AkamaiFingerprint       string                       `json:"akamai_fingerprint"`
	AkamaiFingerprintHash   string                       `json:"akamai_fingerprint_hash"`
}

type stored_http2_setting struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Value int    `json:"value"`
}

type stored_http2_header_priority struct {
	Exclusive bool `json:"exclusive"`
	DependsOn int  `json:"depends_on"`
	Weight    int  `json:"weight"`
}

type stored_header_profile struct {
	Values      map[string]string `json:"values"`
	SampleOrder []string          `json:"sample_order"`
}

func load_chrome_151_profile() (*stored_browser_profile, error) {
	var stored_profile stored_browser_profile
	if err := json.Unmarshal(chrome_151_profile_json, &stored_profile); err != nil {
		return nil, fmt.Errorf("clawreq: decode Chrome 151 profile: %w", err)
	}
	if err := stored_profile.validate(); err != nil {
		return nil, err
	}
	return &stored_profile, nil
}

func (stored_profile *stored_browser_profile) validate() error {
	if stored_profile.SchemaVersion != 1 {
		return fmt.Errorf("clawreq: unsupported profile schema %d", stored_profile.SchemaVersion)
	}
	if stored_profile.Status != "captured_not_enabled" {
		return fmt.Errorf("clawreq: Chrome 151 profile must remain disabled")
	}
	if stored_profile.Browser.Version != chrome_151_version {
		return fmt.Errorf("clawreq: Chrome 151 profile version is %q", stored_profile.Browser.Version)
	}
	if !stored_profile.TLS.RandomExtensionOrder {
		return fmt.Errorf("clawreq: Chrome 151 extension randomization is missing")
	}
	if !contains_int(stored_profile.TLS.SupportedGroups, 4588) {
		return fmt.Errorf("clawreq: Chrome 151 X25519MLKEM768 group is missing")
	}
	if !contains_int(stored_profile.TLS.SampleExtensionOrder, 17613) {
		return fmt.Errorf("clawreq: Chrome 151 ALPS extension is missing")
	}
	if stored_profile.TLS.Fingerprints.JA4 == "" {
		return fmt.Errorf("clawreq: Chrome 151 JA4 fingerprint is missing")
	}
	if len(stored_profile.HTTP2.Settings) != 4 || stored_profile.HTTP2.ConnectionFlowIncrement != 15663105 {
		return fmt.Errorf("clawreq: Chrome 151 HTTP/2 settings are incomplete")
	}
	return nil
}

func contains_int(values []int, expected_value int) bool {
	for _, value := range values {
		if value == expected_value {
			return true
		}
	}
	return false
}
