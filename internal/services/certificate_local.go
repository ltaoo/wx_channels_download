package services

import (
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wx_channel/internal/config"
	"wx_channel/pkg/certificate"
)

// EnsureLocalCertificate keeps configured or trusted certificates before preparing a local CA.
func EnsureLocalCertificate(cfg *config.Config) error {
	return ensure_local_certificate(cfg, check_certificate_trusted)
}

func ensure_local_certificate(cfg *config.Config, check_certificate func([]byte, string) (bool, error)) error {
	if cfg.GetString("cert.file") != "" || cfg.GetString("cert.key") != "" {
		cert, ok := load_configured_cert_files()
		if !ok {
			return fmt.Errorf("无法读取配置的证书和私钥，请检查 cert.file 和 cert.key")
		}
		if _, err := tls.X509KeyPair(cert.Cert, cert.PrivateKey); err != nil {
			return fmt.Errorf("配置的证书与私钥无效: %w", err)
		}
		return nil
	}

	cert_info, err := load_trusted_default_cert_files(check_certificate)
	if err != nil {
		return err
	}
	if cert_info.Cert != nil {
		return nil
	}

	name := certificate.LocalCAName
	cert_pem, key_pem, err := certificate.GenerateRootCA(name, 10*365*24*time.Hour)
	if err != nil {
		return fmt.Errorf("生成本机根证书失败: %w", err)
	}
	cert_path, key_path, err := WriteGeneratedCertificate(cfg, name, cert_pem, key_pem)
	if err != nil {
		return fmt.Errorf("保存本机根证书失败: %w", err)
	}
	if err := cfg.UpdateAndSave(map[string]interface{}{
		"cert.file": cert_path,
		"cert.key":  key_path,
		"cert.name": name,
	}); err != nil {
		return fmt.Errorf("保存本机证书配置失败: %w", err)
	}
	fmt.Printf("已生成本机专属根证书: %s\n", name)
	return nil
}

func WriteGeneratedCertificate(cfg *config.Config, name string, cert_pem []byte, key_pem []byte) (string, string, error) {
	base_dir := cfg.WorkDir
	if strings.TrimSpace(base_dir) == "" {
		base_dir = cfg.RootDir
	}
	dir, err := filepath.Abs(filepath.Join(base_dir, "certs"))
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", "", err
	}
	slug := certificate_filename_slug(name)
	cert_path := filepath.Join(dir, slug+".pem")
	key_path := filepath.Join(dir, slug+".key")
	if err := os.WriteFile(cert_path, cert_pem, 0644); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(key_path, key_pem, 0600); err != nil {
		return "", "", err
	}
	return cert_path, key_path, nil
}

func certificate_filename_slug(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	prev_underscore := false
	for _, r := range name {
		ok := r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
		if ok {
			b.WriteRune(r)
			prev_underscore = false
			continue
		}
		if !prev_underscore {
			b.WriteByte('_')
			prev_underscore = true
		}
	}
	slug := strings.Trim(b.String(), "_")
	if slug == "" {
		return "wx_channels_download"
	}
	return slug
}
