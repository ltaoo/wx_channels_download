package services

import (
	"bytes"
	"encoding/pem"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/viper"

	"wx_channel/pkg/certificate"
)

type CertSource string

const (
	CertSourceSunnyNet   CertSource = "sunny_net"
	CertSourceMitmproxy  CertSource = "mitmproxy"
	CertSourceConfigured CertSource = "configured"
	CertSourceGenerated  CertSource = "generated"
)

type CertFilesInfo struct {
	Cert         *certificate.CertFileAndKeyFile
	Source       CertSource
	IsLegacy     bool
	RiskWarnings []string
}

func LoadCertFilesWithInfo() CertFilesInfo {
	cert := LoadCertFiles()
	info := CertFilesInfo{Cert: cert}

	if cert.Name == certificate.DefaultCertFiles.Name {
		info.Source = CertSourceSunnyNet
		info.IsLegacy = true
		info.RiskWarnings = []string{"该证书为旧版SunnyNet证书，使用硬编码密钥对，存在安全风险，建议删除后安装本机专有证书"}
		return info
	}

	if cert.Name == "mitmproxy" {
		info.Source = CertSourceMitmproxy
		info.IsLegacy = true
		info.RiskWarnings = []string{"当前使用第三方mitmproxy证书，非本机生成，存在潜在安全风险"}
		return info
	}

	// cert.file and cert.key are configured; determine if user-configured or app-generated.
	// App-generated certs are written to the certs/ subdirectory of the work dir.
	cert_file := viper.GetString("cert.file")
	if cert_file != "" {
		if abs_path, err := filepath.Abs(cert_file); err == nil && is_under_certs_dir(abs_path) {
			info.Source = CertSourceGenerated
			return info
		}
	}

	info.Source = CertSourceConfigured
	return info
}

// AvailableCert represents a certificate available to the proxy.
type AvailableCert struct {
	Cert         *certificate.CertFileAndKeyFile
	Source       CertSource
	IsLegacy     bool
	IsActive     bool
	RiskWarnings []string
}

// ScanAvailableCerts returns all certificates known to the system,
// including the built-in SunnyNet cert, mitmproxy cert (if present),
// and the user-configured or generated cert. Exactly one cert is marked active.
func ScanAvailableCerts() []AvailableCert {
	active_cert := LoadCertFiles()
	var certs []AvailableCert

	// 1. SunnyNet (always available as fallback)
	sunny_entry := AvailableCert{
		Cert:     certificate.DefaultCertFiles,
		Source:   CertSourceSunnyNet,
		IsLegacy: true,
		IsActive: active_cert.Name == certificate.DefaultCertFiles.Name,
		RiskWarnings: []string{
			"该证书为旧版SunnyNet证书，使用硬编码密钥对，存在安全风险，建议替换为本机生成的证书",
		},
	}
	certs = append(certs, sunny_entry)

	// 2. mitmproxy (available if cert files exist on disk)
	if mitm_cert := try_load_mitmproxy_cert(); mitm_cert != nil {
		mitm_entry := AvailableCert{
			Cert:     mitm_cert,
			Source:   CertSourceMitmproxy,
			IsLegacy: true,
			IsActive: active_cert.Name == "mitmproxy",
			RiskWarnings: []string{
				"当前使用第三方mitmproxy证书，非本机生成，存在潜在安全风险，建议替换为本机生成的证书",
			},
		}
		certs = append(certs, mitm_entry)
	}

	// 3. Configured/generated cert (available if cert.file + cert.key are set)
	if conf_cert, ok := load_configured_cert_files(); ok {
		source := CertSourceConfigured
		if abs_path, err := filepath.Abs(viper.GetString("cert.file")); err == nil && is_under_certs_dir(abs_path) {
			source = CertSourceGenerated
		}
		is_active := active_cert.Name == conf_cert.Name // compare name since object identities differ
		// Also compare by source: only the configured/generated cert can be active
		// when active_cert is neither SunnyNet nor mitmproxy.
		if !is_active && active_cert.Name != certificate.DefaultCertFiles.Name && active_cert.Name != "mitmproxy" {
			is_active = true
		}
		conf_entry := AvailableCert{
			Cert:     conf_cert,
			Source:   source,
			IsLegacy: false,
			IsActive: is_active,
		}
		certs = append(certs, conf_entry)
	}

	return certs
}

// try_load_mitmproxy_cert loads the mitmproxy certificate from standard locations.
// Returns nil if the cert files are not found.
func try_load_mitmproxy_cert() *certificate.CertFileAndKeyFile {
	var dirs []string
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".mitmproxy"))
	}
	if runtime.GOOS == "windows" {
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			dirs = append(dirs, filepath.Join(appdata, "mitmproxy"))
		}
	}
	for _, dir := range dirs {
		cert_path := filepath.Join(dir, "mitmproxy-ca-cert.pem")
		key_path := filepath.Join(dir, "mitmproxy-ca.pem")
		if cert_bytes, err1 := os.ReadFile(cert_path); err1 == nil {
			if key_bytes, err2 := os.ReadFile(key_path); err2 == nil {
				return &certificate.CertFileAndKeyFile{
					Name:       "mitmproxy",
					Cert:       cert_bytes,
					PrivateKey: key_bytes,
				}
			}
		}
		if key_bytes, err := os.ReadFile(key_path); err == nil {
			rest := key_bytes
			var cert_blocks [][]byte
			var key_block []byte
			for {
				block, rem := pem.Decode(rest)
				if block == nil {
					break
				}
				rest = rem
				if block.Type == "CERTIFICATE" {
					enc := pem.EncodeToMemory(block)
					if enc != nil {
						cert_blocks = append(cert_blocks, enc)
					}
				} else if strings.Contains(block.Type, "PRIVATE KEY") {
					enc := pem.EncodeToMemory(block)
					if enc != nil {
						key_block = enc
					}
				}
			}
			if len(cert_blocks) > 0 && len(key_block) > 0 {
				return &certificate.CertFileAndKeyFile{
					Name:       "mitmproxy",
					Cert:       bytes.Join(cert_blocks, []byte("")),
					PrivateKey: key_block,
				}
			}
		}
	}
	return nil
}

func LoadCertFiles() *certificate.CertFileAndKeyFile {
	if cert, ok := load_configured_cert_files(); ok {
		return cert
	}
	if mitm_cert := try_load_mitmproxy_cert(); mitm_cert != nil {
		return mitm_cert
	}
	return certificate.DefaultCertFiles
}

func load_configured_cert_files() (*certificate.CertFileAndKeyFile, bool) {
	cert_file_path := viper.GetString("cert.file")
	cert_key_file_path := viper.GetString("cert.key")
	if cert_file_path != "" && cert_key_file_path != "" {
		if cert_bytes, err := os.ReadFile(cert_file_path); err == nil {
			if cert_key_bytes, err2 := os.ReadFile(cert_key_file_path); err2 == nil {
				cert_name := viper.GetString("cert.name")
				if strings.TrimSpace(cert_name) == "" {
					cert_name = certificate.DefaultCertFiles.Name
				}
				return &certificate.CertFileAndKeyFile{
					Name:       cert_name,
					Cert:       cert_bytes,
					PrivateKey: cert_key_bytes,
				}, true
			}
		}
	}
	return nil, false
}

func is_under_certs_dir(abs_cert_path string) bool {
	return filepath.Base(filepath.Dir(abs_cert_path)) == "certs"
}
