//go:build linux

package certificate

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

func fetchCertificates() ([]Certificate, error) {
	var certs []Certificate
	paths := []string{"/etc/ssl/certs", "/usr/local/share/ca-certificates"}

	for _, dir := range paths {
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			ext := filepath.Ext(path)
			if ext != ".crt" && ext != ".pem" {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			for {
				block, rest := pem.Decode(data)
				if block == nil {
					break
				}
				data = rest
				if block.Type != "CERTIFICATE" {
					continue
				}
				cert, err := x509.ParseCertificate(block.Bytes)
				if err != nil {
					continue
				}
				certs = append(certs, Certificate{
					Subject: CertificateSubject{CN: cert.Subject.CommonName},
				})
			}
			return nil
		})
	}
	return certs, nil
}

func installCertificate(cert []byte) error {
	certPath := "/usr/local/share/ca-certificates/WeChatAppEx_CA.crt"
	err := os.WriteFile(certPath, cert, 0644)
	if err != nil {
		return fmt.Errorf("写入证书失败: %v", err)
	}
	if output, err := exec.Command("update-ca-certificates", "--fresh").CombinedOutput(); err != nil {
		return fmt.Errorf("更新 OpenSSL 证书库失败: %v\n输出: %s", err, string(output))
	}
	if _, err := exec.LookPath("certutil"); err == nil {
		exec.Command("certutil", "-d", "sql:/etc/pki/nssdb", "-D", "-n", "WeChatAppEx_CA").Run()
		exec.Command("certutil", "-d", "sql:/etc/pki/nssdb", "-A", "-n", "WeChatAppEx_CA", "-t", "CT,C,C", "-i", certPath).Run()
		if home, _ := os.UserHomeDir(); home != "" {
			userDB := filepath.Join(home, ".pki", "nssdb")
			os.MkdirAll(userDB, 0700)
			userDBSQL := "sql:" + userDB
			exec.Command("certutil", "-d", userDBSQL, "-D", "-n", "WeChatAppEx_CA").Run()
			exec.Command("certutil", "-d", userDBSQL, "-A", "-n", "WeChatAppEx_CA", "-t", "CT,C,C", "-i", certPath).Run()
		}
	}
	return nil
}

func uninstallCertificate(name string) error {
	certPath := "/usr/local/share/ca-certificates/WeChatAppEx_CA.crt"
	_ = os.Remove(certPath)
	if output, err := exec.Command("update-ca-certificates", "--fresh").CombinedOutput(); err != nil {
		return fmt.Errorf("更新 OpenSSL 证书库失败: %v\n输出: %s", err, string(output))
	}
	exec.Command("certutil", "-d", "sql:/etc/pki/nssdb", "-D", "-n", "WeChatAppEx_CA").Run()
	if home, _ := os.UserHomeDir(); home != "" {
		userDB := filepath.Join(home, ".pki", "nssdb")
		userDBSQL := "sql:" + userDB
		exec.Command("certutil", "-d", userDBSQL, "-D", "-n", "WeChatAppEx_CA").Run()
	}
	return nil
}
