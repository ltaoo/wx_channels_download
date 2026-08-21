package services

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/viper"

	"wx_channel/internal/config"
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

// CertificateService provides certificate status independently of any
// transport such as HTTP or MCP.
type CertificateService struct {
	application_config *config.Config
}

func NewCertificateService(application_config *config.Config) *CertificateService {
	return &CertificateService{application_config: application_config}
}

// Status returns the active certificate and every available alternative.
func (s *CertificateService) Status() map[string]any {
	cert_info := LoadCertFilesWithInfo()
	cert := cert_info.Cert
	installed, install_err := certificate.CheckHasCertificate(cert.Name)
	data := map[string]any{
		"name":          cert.Name,
		"source":        string(cert_info.Source),
		"is_legacy":     cert_info.IsLegacy,
		"risk_warnings": cert_info.RiskWarnings,
		"installed":     installed,
		"trusted":       false,
		"pem":           string(cert.Cert),
	}
	if installed {
		trusted, trust_err := certificate.CheckCertificateDataTrusted(cert.Cert, cert.Name)
		if trust_err == nil {
			data["trusted"] = trusted
		}
		if trust_err != nil && install_err == nil {
			install_err = trust_err
		}
	}
	if install_err != nil {
		data["install_status_error"] = install_err.Error()
	}
	if details, err := inspect_certificate(cert.Cert); err == nil {
		data["detail"] = details
	} else {
		data["parse_error"] = err.Error()
	}
	if s != nil && s.application_config != nil {
		data["configured"] = map[string]any{
			"name": s.application_config.GetString("cert.name"),
			"file": s.application_config.GetString("cert.file"),
			"key":  s.application_config.GetString("cert.key"),
		}
	}

	available_certs := ScanAvailableCerts()
	cert_list := make([]map[string]any, 0, len(available_certs))
	for _, available_cert := range available_certs {
		cert_list = append(cert_list, certificate_status_entry(available_cert))
	}
	data["all_certificates"] = cert_list
	return data
}

func certificate_status_entry(available_cert AvailableCert) map[string]any {
	installed, _ := certificate.CheckHasCertificate(available_cert.Cert.Name)
	entry := map[string]any{
		"name":          available_cert.Cert.Name,
		"source":        string(available_cert.Source),
		"is_legacy":     available_cert.IsLegacy,
		"is_active":     available_cert.IsActive,
		"installed":     installed,
		"trusted":       false,
		"risk_warnings": available_cert.RiskWarnings,
	}
	if installed {
		trusted, trust_err := certificate.CheckCertificateDataTrusted(available_cert.Cert.Cert, available_cert.Cert.Name)
		if trust_err == nil {
			entry["trusted"] = trusted
		}
	}
	if details, err := inspect_certificate(available_cert.Cert.Cert); err == nil {
		entry["detail"] = details
	}
	return entry
}

func inspect_certificate(data []byte) (map[string]any, error) {
	cert, err := parse_first_certificate(data)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(cert.Raw)
	return map[string]any{
		"subject_common_name": cert.Subject.CommonName,
		"issuer_common_name":  cert.Issuer.CommonName,
		"serial_number":       cert.SerialNumber.String(),
		"not_before":          cert.NotBefore.Format(time.RFC3339),
		"not_after":           cert.NotAfter.Format(time.RFC3339),
		"expired":             time.Now().After(cert.NotAfter),
		"is_ca":               cert.IsCA,
		"dns_names":           cert.DNSNames,
		"organizations":       cert.Subject.Organization,
		"fingerprint_sha256":  format_fingerprint(sum[:]),
	}, nil
}

func parse_first_certificate(data []byte) (*x509.Certificate, error) {
	rest := data
	for {
		block, next := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = next
		if block.Type != "CERTIFICATE" {
			continue
		}
		return x509.ParseCertificate(block.Bytes)
	}
	return x509.ParseCertificate(data)
}

func format_fingerprint(data []byte) string {
	encoded := strings.ToUpper(hex.EncodeToString(data))
	parts := make([]string, 0, len(encoded)/2)
	for index := 0; index+2 <= len(encoded); index += 2 {
		parts = append(parts, encoded[index:index+2])
	}
	return strings.Join(parts, ":")
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
