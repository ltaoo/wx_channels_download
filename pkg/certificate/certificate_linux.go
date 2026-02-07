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
	"strings"
)

// 检查是否以 root 权限运行
func isRoot() bool {
	return os.Geteuid() == 0
}

// 确保 root 权限，如果没有则自动请求 sudo
func ensureRoot(commandName string) bool {
	if !isRoot() {
		execPath, err := os.Executable()
		if err != nil {
			return false
		}
		fmt.Println("需要管理员权限，正在请求...")
		cmd := exec.Command("sudo", execPath, commandName)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return false
		}
		os.Exit(0)
	}
	return true
}

// 执行 certutil 命令
func runCertutilQuiet(args ...string) error {
	cmd := exec.Command("certutil", args...)
	// certutil --empty-password 仍然需要 stdin 输入（按两次 Enter）
	cmd.Stdin = strings.NewReader("\n\n")
	return cmd.Run()
}

func fetchCertificates() ([]Certificate, error) {
	var certs []Certificate
	paths := []string{
		"/etc/ssl/certs",
		"/usr/local/share/ca-certificates",
		"/etc/ca-certificates/trust-source/anchors", // Arch Linux
	}

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
	// 检查 root 权限
	if !ensureRoot("install") {
		return fmt.Errorf("需要 root 权限安装证书")
	}

	var certPath string
	var updateCmd []string
	var updateErrMsg string

	// 简单检测发行版类型
	if _, err := exec.LookPath("update-ca-certificates"); err == nil {
		// Ubuntu/Debian 系
		certPath = "/usr/local/share/ca-certificates/WeChatAppEx_CA.crt"
		updateCmd = []string{"update-ca-certificates", "--fresh"}
		updateErrMsg = "更新 OpenSSL 证书库失败"
	} else if _, err := exec.LookPath("trust"); err == nil {
		// Arch Linux 系
		certPath = "/etc/ca-certificates/trust-source/anchors/WeChatAppEx_CA.crt"
		updateCmd = []string{"trust", "extract-compat"}
		updateErrMsg = "更新证书库失败 (trust)"
	} else {
		return fmt.Errorf("未找到支持的证书更新命令！ (update-ca-certificates 或 trust)")
	}

	// 创建证书目录
	if err := os.MkdirAll(filepath.Dir(certPath), 0755); err != nil {
		return fmt.Errorf("创建证书目录失败: %v", err)
	}

	// 写入证书文件
	if err := os.WriteFile(certPath, cert, 0644); err != nil {
		return fmt.Errorf("写入证书失败: %v", err)
	}

	// 更新证书库
	if output, err := exec.Command(updateCmd[0], updateCmd[1:]...).CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %v\n输出: %s", updateErrMsg, err, string(output))
	}

	// NSS 证书库处理 (Firefox/Chromium)
	if _, err := exec.LookPath("certutil"); err == nil {
		// 系统 NSS 数据库
		systemDB := "/etc/pki/nssdb"
		if _, err := os.Stat(systemDB); os.IsNotExist(err) {
			// Arch Linux 默认没有 NSS 数据库，但是是必要的。需要创建
			fmt.Printf("未找到 NSS 系统数据库: %s，正在创建...\n", systemDB)
			if err := os.MkdirAll(systemDB, 0700); err != nil {
				fmt.Printf("警告: 创建 NSS 数据库目录失败: %v\n", err)
			} else {
				// 初始化空数据库（无密码，需要自动输入回车）
				if err := runCertutilQuiet("-d", "sql:"+systemDB, "-N", "--empty-password"); err == nil {
					fmt.Printf("已创建 NSS 系统数据库: %s (密码为空)\n", systemDB)
				}
			}
		}
		// 添加证书到系统 NSS 数据库
		if _, err := os.Stat(systemDB); err == nil {
			exec.Command("certutil", "-d", "sql:"+systemDB, "-D", "-n", "WeChatAppEx_CA").Run()
			exec.Command("certutil", "-d", "sql:"+systemDB, "-A", "-n", "WeChatAppEx_CA", "-t", "CT,C,C", "-i", certPath).Run()
		}

		// 用户 NSS 数据库
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			userDB := filepath.Join(home, ".pki", "nssdb")
			if _, err := os.Stat(userDB); os.IsNotExist(err) {
				fmt.Printf("未找到 NSS 用户数据库: %s，正在创建...\n", userDB)
				os.MkdirAll(userDB, 0700)
				// 初始化空数据库（无密码，需要自动输入回车）
				if err := runCertutilQuiet("-d", "sql:"+userDB, "-N", "--empty-password"); err == nil {
					fmt.Printf("已创建 NSS 用户数据库: %s (密码为空)\n", userDB)
				}
			}
			exec.Command("certutil", "-d", "sql:"+userDB, "-D", "-n", "WeChatAppEx_CA").Run()
			exec.Command("certutil", "-d", "sql:"+userDB, "-A", "-n", "WeChatAppEx_CA", "-t", "CT,C,C", "-i", certPath).Run()
		}
	}
	return nil
}

func uninstallCertificate(name string) error {
	// 检查 root 权限
	if !ensureRoot("uninstall") {
		return fmt.Errorf("需要 root 权限卸载证书")
	}

	var certPath string
	var updateCmd []string
	var updateErrMsg string

	// 简单检测发行版类型
	if _, err := exec.LookPath("update-ca-certificates"); err == nil {
		// Ubuntu/Debian 系
		certPath = "/usr/local/share/ca-certificates/WeChatAppEx_CA.crt"
		updateCmd = []string{"update-ca-certificates", "--fresh"}
		updateErrMsg = "更新 OpenSSL 证书库失败"
	} else if _, err := exec.LookPath("trust"); err == nil {
		// Arch Linux 系
		certPath = "/etc/ca-certificates/trust-source/anchors/WeChatAppEx_CA.crt"
		updateCmd = []string{"trust", "extract-compat"}
		updateErrMsg = "更新证书库失败 (trust)"
	} else {
		return fmt.Errorf("未找到支持的证书更新命令！ (update-ca-certificates 或 trust)")
	}

	// 删除证书文件
	_ = os.Remove(certPath)

	// 更新证书库
	if output, err := exec.Command(updateCmd[0], updateCmd[1:]...).CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %v\n输出: %s", updateErrMsg, err, string(output))
	}

	// 从 NSS 证书库删除
	if _, err := exec.LookPath("certutil"); err == nil {
		// 系统 NSS 数据库
		systemDB := "/etc/pki/nssdb"
		if _, err := os.Stat(systemDB); err == nil {
			exec.Command("certutil", "-d", "sql:"+systemDB, "-D", "-n", "WeChatAppEx_CA").Run()
		}

		// 用户 NSS 数据库
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			userDB := filepath.Join(home, ".pki", "nssdb")
			exec.Command("certutil", "-d", "sql:"+userDB, "-D", "-n", "WeChatAppEx_CA").Run()
		}
	}
	return nil
}
