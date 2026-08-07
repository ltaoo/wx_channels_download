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
	certFile := viper.GetString("cert.file")
	if certFile != "" {
		if absPath, err := filepath.Abs(certFile); err == nil && isUnderCertsDir(absPath) {
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
	activeCert := LoadCertFiles()
	var certs []AvailableCert

	// 1. SunnyNet (always available as fallback)
	sunnyEntry := AvailableCert{
		Cert:     certificate.DefaultCertFiles,
		Source:   CertSourceSunnyNet,
		IsLegacy: true,
		IsActive: activeCert.Name == certificate.DefaultCertFiles.Name,
		RiskWarnings: []string{
			"该证书为旧版SunnyNet证书，使用硬编码密钥对，存在安全风险，建议替换为本机生成的证书",
		},
	}
	certs = append(certs, sunnyEntry)

	// 2. mitmproxy (available if cert files exist on disk)
	if mitmCert := tryLoadMitmproxyCert(); mitmCert != nil {
		mitmEntry := AvailableCert{
			Cert:     mitmCert,
			Source:   CertSourceMitmproxy,
			IsLegacy: true,
			IsActive: activeCert.Name == "mitmproxy",
			RiskWarnings: []string{
				"当前使用第三方mitmproxy证书，非本机生成，存在潜在安全风险，建议替换为本机生成的证书",
			},
		}
		certs = append(certs, mitmEntry)
	}

	// 3. Configured/generated cert (available if cert.file + cert.key are set)
	if confCert, ok := loadConfiguredCertFiles(); ok {
		source := CertSourceConfigured
		if absPath, err := filepath.Abs(viper.GetString("cert.file")); err == nil && isUnderCertsDir(absPath) {
			source = CertSourceGenerated
		}
		isActive := activeCert.Name == confCert.Name // compare name since object identities differ
		// Also compare by source: only the configured/generated cert can be active
		// when activeCert is neither SunnyNet nor mitmproxy.
		if !isActive && activeCert.Name != certificate.DefaultCertFiles.Name && activeCert.Name != "mitmproxy" {
			isActive = true
		}
		confEntry := AvailableCert{
			Cert:     confCert,
			Source:   source,
			IsLegacy: false,
			IsActive: isActive,
		}
		certs = append(certs, confEntry)
	}

	return certs
}

// tryLoadMitmproxyCert loads the mitmproxy certificate from standard locations.
// Returns nil if the cert files are not found.
func tryLoadMitmproxyCert() *certificate.CertFileAndKeyFile {
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
		certPath := filepath.Join(dir, "mitmproxy-ca-cert.pem")
		keyPath := filepath.Join(dir, "mitmproxy-ca.pem")
		if certBytes, err1 := os.ReadFile(certPath); err1 == nil {
			if keyBytes, err2 := os.ReadFile(keyPath); err2 == nil {
				return &certificate.CertFileAndKeyFile{
					Name:       "mitmproxy",
					Cert:       certBytes,
					PrivateKey: keyBytes,
				}
			}
		}
		if keyBytes, err := os.ReadFile(keyPath); err == nil {
			rest := keyBytes
			var certBlocks [][]byte
			var keyBlock []byte
			for {
				block, rem := pem.Decode(rest)
				if block == nil {
					break
				}
				rest = rem
				if block.Type == "CERTIFICATE" {
					enc := pem.EncodeToMemory(block)
					if enc != nil {
						certBlocks = append(certBlocks, enc)
					}
				} else if strings.Contains(block.Type, "PRIVATE KEY") {
					enc := pem.EncodeToMemory(block)
					if enc != nil {
						keyBlock = enc
					}
				}
			}
			if len(certBlocks) > 0 && len(keyBlock) > 0 {
				return &certificate.CertFileAndKeyFile{
					Name:       "mitmproxy",
					Cert:       bytes.Join(certBlocks, []byte("")),
					PrivateKey: keyBlock,
				}
			}
		}
	}
	return nil
}

func LoadCertFiles() *certificate.CertFileAndKeyFile {
	if cert, ok := loadConfiguredCertFiles(); ok {
		return cert
	}
	if mitmCert := tryLoadMitmproxyCert(); mitmCert != nil {
		return mitmCert
	}
	return certificate.DefaultCertFiles
}

func loadConfiguredCertFiles() (*certificate.CertFileAndKeyFile, bool) {
	certFilePath := viper.GetString("cert.file")
	certKeyFilePath := viper.GetString("cert.key")
	if certFilePath != "" && certKeyFilePath != "" {
		if certBytes, err := os.ReadFile(certFilePath); err == nil {
			if certKeyBytes, err2 := os.ReadFile(certKeyFilePath); err2 == nil {
				certName := viper.GetString("cert.name")
				if strings.TrimSpace(certName) == "" {
					certName = certificate.DefaultCertFiles.Name
				}
				return &certificate.CertFileAndKeyFile{
					Name:       certName,
					Cert:       certBytes,
					PrivateKey: certKeyBytes,
				}, true
			}
		}
	}
	return nil, false
}

func isUnderCertsDir(absCertPath string) bool {
	return filepath.Base(filepath.Dir(absCertPath)) == "certs"
}
