package services

import (
	"bytes"
	"crypto/tls"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"

	"wx_channel/internal/config"
	"wx_channel/pkg/certificate"
)

func new_certificate_test_config(t *testing.T) *config.Config {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("APPDATA", dir)
	return &config.Config{WorkDir: dir, RootDir: dir, FullPath: filepath.Join(dir, "config.yaml")}
}

func TestEnsureLocalCertificateConfigured(t *testing.T) {
	cert_pem, key_pem, err := certificate.GenerateRootCA("configured", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	for _, test_case := range []struct {
		name         string
		cert_data    []byte
		key_data     []byte
		trusted      bool
		check_err    error
		expected_err string
		expect_check bool
	}{
		{name: "missing certificate", key_data: key_pem, expected_err: "无法读取"},
		{name: "missing key", cert_data: cert_pem, expected_err: "无法读取"},
		{name: "invalid certificate", cert_data: []byte("invalid"), key_data: key_pem, expected_err: "证书与私钥无效"},
		{name: "mismatched key", cert_data: cert_pem, key_data: certificate.DefaultCertFiles.PrivateKey, expected_err: "证书与私钥无效"},
		{name: "not installed or trusted", cert_data: cert_pem, key_data: key_pem, expect_check: true, expected_err: "未安装或未受信任"},
		{name: "trust check failed", cert_data: cert_pem, key_data: key_pem, expect_check: true, check_err: errors.New("keychain unavailable"), expected_err: "keychain unavailable"},
		{name: "trusted", cert_data: cert_pem, key_data: key_pem, expect_check: true, trusted: true},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			cfg := new_certificate_test_config(t)
			cert_path := filepath.Join(cfg.WorkDir, "configured.pem")
			key_path := filepath.Join(cfg.WorkDir, "configured.key")
			viper.Set("cert.file", cert_path)
			viper.Set("cert.key", key_path)
			viper.Set("cert.name", "configured")
			viper.Set("proxy.skipInstallRootCert", true)
			for path, data := range map[string][]byte{cert_path: test_case.cert_data, key_path: test_case.key_data} {
				if data != nil {
					if err := os.WriteFile(path, data, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			checked := false
			err := ensure_local_certificate(cfg, func(data []byte, name string) (bool, error) {
				checked = true
				if name != "configured" || !bytes.Equal(data, cert_pem) {
					t.Fatal("configured certificate was replaced")
				}
				return test_case.trusted, test_case.check_err
			}, func([]byte) error {
				t.Fatal("configured certificates must never be installed automatically")
				return nil
			})
			if test_case.expected_err == "" && err != nil || test_case.expected_err != "" && (err == nil || !strings.Contains(err.Error(), test_case.expected_err)) {
				t.Fatalf("got %v, want error containing %q", err, test_case.expected_err)
			}
			if checked != test_case.expect_check {
				t.Fatalf("trust checked=%v, want %v", checked, test_case.expect_check)
			}
			if _, err := os.Stat(cfg.FullPath); !os.IsNotExist(err) {
				t.Fatalf("configured certificate must not rewrite config: %v", err)
			}
		})
	}
}

func TestEnsureLocalCertificateReusesTrustedRoots(t *testing.T) {
	for _, test_case := range []struct {
		name            string
		trusted_name    string
		expected_checks []string
	}{
		{name: "SunnyNet first", trusted_name: "SunnyNet", expected_checks: []string{"SunnyNet"}},
		{name: "mitmproxy fallback", trusted_name: "mitmproxy", expected_checks: []string{"SunnyNet", "mitmproxy"}},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			cfg := new_certificate_test_config(t)
			dir := filepath.Join(cfg.WorkDir, ".mitmproxy")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			for name, data := range map[string][]byte{"mitmproxy-ca-cert.pem": certificate.DefaultCertFiles.Cert, "mitmproxy-ca.pem": certificate.DefaultCertFiles.PrivateKey} {
				if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			var checked_names []string
			err := ensure_local_certificate(cfg, func(_ []byte, name string) (bool, error) {
				checked_names = append(checked_names, name)
				return name == test_case.trusted_name, nil
			}, func([]byte) error {
				t.Fatal("trusted roots must be reused without installation")
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(checked_names, test_case.expected_checks) {
				t.Fatalf("checked %v, want %v", checked_names, test_case.expected_checks)
			}
			if viper.GetString("cert.file") != "" {
				t.Fatal("reused roots must not be written to configuration")
			}
			if _, err := os.Stat(filepath.Join(cfg.WorkDir, "certs")); !os.IsNotExist(err) {
				t.Fatalf("reusing a root must not generate a certificate: %v", err)
			}
		})
	}
}

func TestEnsureLocalCertificateInstallsBeforeSaving(t *testing.T) {
	for _, test_case := range []struct {
		name         string
		install_err  error
		check_err    error
		trusted      bool
		expected_err string
	}{
		{name: "installation failed", install_err: errors.New("installation denied"), expected_err: "安装本机根证书失败"},
		{name: "verification failed", check_err: errors.New("verification unavailable"), expected_err: "验证本机根证书安装和信任状态失败"},
		{name: "still untrusted", expected_err: "安装后仍未受信任"},
		{name: "trusted and saved", trusted: true},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			cfg := new_certificate_test_config(t)
			viper.Set("proxy.skipInstallRootCert", true)
			var installed_cert []byte
			err := ensure_local_certificate(cfg, func(data []byte, name string) (bool, error) {
				if name != certificate.LocalCAName {
					return false, nil
				}
				if !bytes.Equal(data, installed_cert) {
					t.Fatal("new root must be installed before verifying trust")
				}
				return test_case.trusted, test_case.check_err
			}, func(data []byte) error {
				if viper.GetString("cert.file") != "" {
					t.Fatal("configuration must not be saved before installation")
				}
				installed_cert = data
				return test_case.install_err
			})
			if test_case.expected_err != "" {
				if err == nil || !strings.Contains(err.Error(), test_case.expected_err) {
					t.Fatalf("got %v, want error containing %q", err, test_case.expected_err)
				}
				if viper.GetString("cert.file") != "" {
					t.Fatal("failed installation or verification must not persist certificate configuration")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			cert_data, err := os.ReadFile(viper.GetString("cert.file"))
			if err != nil || !bytes.Equal(cert_data, installed_cert) {
				t.Fatalf("persisted certificate differs from installed certificate: %v", err)
			}
			if _, err := tls.LoadX509KeyPair(viper.GetString("cert.file"), viper.GetString("cert.key")); err != nil {
				t.Fatal(err)
			}
			saved := viper.New()
			saved.SetConfigFile(cfg.FullPath)
			if err := saved.ReadInConfig(); err != nil {
				t.Fatal(err)
			}
			if saved.GetString("cert.file") != viper.GetString("cert.file") || saved.GetString("cert.name") != certificate.LocalCAName {
				t.Fatal("generated certificate configuration was not persisted")
			}
			if err := ensure_local_certificate(cfg, func(data []byte, _ string) (bool, error) {
				return bytes.Equal(data, installed_cert), nil
			}, func([]byte) error {
				t.Fatal("subsequent startups must reuse the configured certificate")
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
