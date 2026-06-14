//go:build linux

package certificate

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

const linuxCertNickname = "WeChatAppEx_CA"

type linuxCertStore struct {
	certPath     string
	updateCmd    []string
	updateErrMsg string
}

// 检查是否以 root 权限运行
func isRoot() bool {
	return os.Geteuid() == 0
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func detectLinuxCertStore() (*linuxCertStore, error) {
	if commandExists("update-ca-certificates") {
		return &linuxCertStore{
			certPath:     "/usr/local/share/ca-certificates/" + linuxCertNickname + ".crt",
			updateCmd:    []string{"update-ca-certificates", "--fresh"},
			updateErrMsg: "更新 OpenSSL 证书库失败",
		}, nil
	}
	if commandExists("update-ca-trust") {
		return &linuxCertStore{
			certPath:     "/etc/pki/ca-trust/source/anchors/" + linuxCertNickname + ".crt",
			updateCmd:    []string{"update-ca-trust", "extract"},
			updateErrMsg: "更新证书库失败 (update-ca-trust)",
		}, nil
	}
	if commandExists("trust") {
		return &linuxCertStore{
			certPath:     "/etc/ca-certificates/trust-source/anchors/" + linuxCertNickname + ".crt",
			updateCmd:    []string{"trust", "extract-compat"},
			updateErrMsg: "更新证书库失败 (trust)",
		}, nil
	}
	return nil, fmt.Errorf("未找到支持的证书更新命令！ (update-ca-certificates、update-ca-trust 或 trust)")
}

func runCommand(stdin io.Reader, args ...string) error {
	if len(args) == 0 {
		return nil
	}
	cmd := exec.Command(args[0], args[1:]...)
	if stdin != nil {
		cmd.Stdin = stdin
	} else {
		cmd.Stdin = os.Stdin
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runPrivileged(stdin io.Reader, args ...string) error {
	if isRoot() {
		return runCommand(stdin, args...)
	}
	fmt.Println("需要管理员权限，正在请求...")
	return runCommand(stdin, append([]string{"sudo"}, args...)...)
}

func currentDesktopUser() (*user.User, error) {
	candidates := []string{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || value == "root" {
			return
		}
		for _, item := range candidates {
			if item == value {
				return
			}
		}
		candidates = append(candidates, value)
	}

	add(os.Getenv("SUDO_USER"))
	add(os.Getenv("USER"))
	add(os.Getenv("LOGNAME"))

	for _, name := range candidates {
		if u, err := user.Lookup(name); err == nil {
			return u, nil
		}
	}
	if pkexecUID := strings.TrimSpace(os.Getenv("PKEXEC_UID")); pkexecUID != "" {
		if u, err := user.LookupId(pkexecUID); err == nil {
			return u, nil
		}
	}
	if u, err := user.Current(); err == nil {
		return u, nil
	}
	return nil, fmt.Errorf("获取登录用户失败: 无法从 SUDO_USER/USER/LOGNAME/PKEXEC_UID 获取用户")
}

func runAsUser(u *user.User, stdin io.Reader, args ...string) error {
	if u == nil {
		return runCommand(stdin, args...)
	}
	if strconv.Itoa(os.Geteuid()) == u.Uid {
		cmd := append([]string{}, args...)
		return runCommand(stdin, cmd...)
	}
	envArgs := append([]string{"env", "HOME=" + u.HomeDir}, args...)
	if isRoot() && commandExists("runuser") {
		return runCommand(stdin, append([]string{"runuser", "-u", u.Username, "--"}, envArgs...)...)
	}
	return runCommand(stdin, append([]string{"sudo", "-u", u.Username}, envArgs...)...)
}

func writeSystemCert(path string, cert []byte) error {
	if isRoot() {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("创建证书目录失败: %v", err)
		}
		if err := os.WriteFile(path, cert, 0644); err != nil {
			return fmt.Errorf("写入证书失败: %v", err)
		}
		return nil
	}

	tmp, err := os.CreateTemp("", linuxCertNickname+"-*.crt")
	if err != nil {
		return fmt.Errorf("创建临时证书失败: %v", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(cert); err != nil {
		tmp.Close()
		return fmt.Errorf("写入临时证书失败: %v", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时证书失败: %v", err)
	}
	if err := os.Chmod(tmpPath, 0644); err != nil {
		return fmt.Errorf("设置临时证书权限失败: %v", err)
	}
	if err := runPrivileged(nil, "mkdir", "-p", filepath.Dir(path)); err != nil {
		return fmt.Errorf("创建证书目录失败: %v", err)
	}
	if err := runPrivileged(nil, "install", "-m", "0644", tmpPath, path); err != nil {
		return fmt.Errorf("写入证书失败: %v", err)
	}
	return nil
}

func removeSystemCert(path string) error {
	if isRoot() {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return runPrivileged(nil, "rm", "-f", path)
}

func warnIfErr(prefix string, err error) {
	if err != nil {
		fmt.Printf("警告: %s: %v\n", prefix, err)
	}
}

func installSystemNSSCert(certPath string) {
	if !commandExists("certutil") {
		return
	}

	systemDB := "/etc/pki/nssdb"
	if _, err := os.Stat(systemDB); os.IsNotExist(err) {
		fmt.Printf("未找到 NSS 系统数据库: %s，正在创建...\n", systemDB)
		if err := runPrivileged(nil, "mkdir", "-p", systemDB); err != nil {
			warnIfErr("创建 NSS 系统数据库目录失败", err)
			return
		}
		warnIfErr("设置 NSS 系统数据库目录权限失败", runPrivileged(nil, "chmod", "700", systemDB))
		err := runPrivileged(strings.NewReader("\n\n"), "certutil", "-d", "sql:"+systemDB, "-N", "--empty-password")
		if err != nil {
			warnIfErr("初始化 NSS 系统数据库失败", err)
			return
		}
		fmt.Printf("已创建 NSS 系统数据库: %s (密码为空)\n", systemDB)
	}
	_ = runPrivileged(nil, "certutil", "-d", "sql:"+systemDB, "-D", "-n", linuxCertNickname)
	warnIfErr("添加 NSS 系统证书失败", runPrivileged(nil, "certutil", "-d", "sql:"+systemDB, "-A", "-n", linuxCertNickname, "-t", "CT,C,C", "-i", certPath))
}

func installUserNSSCert(certPath string) {
	if !commandExists("certutil") {
		return
	}
	desktopUser, err := currentDesktopUser()
	if err != nil || desktopUser.HomeDir == "" {
		warnIfErr("获取登录用户失败，跳过 NSS 用户证书库", err)
		return
	}
	userDB := filepath.Join(desktopUser.HomeDir, ".pki", "nssdb")
	if _, err := os.Stat(userDB); os.IsNotExist(err) {
		fmt.Printf("未找到 NSS 用户数据库: %s，正在创建...\n", userDB)
		if err := runAsUser(desktopUser, nil, "mkdir", "-p", userDB); err != nil {
			warnIfErr("创建 NSS 用户数据库目录失败", err)
			return
		}
		warnIfErr("设置 NSS 用户数据库目录权限失败", runAsUser(desktopUser, nil, "chmod", "700", userDB))
		err := runAsUser(desktopUser, strings.NewReader("\n\n"), "certutil", "-d", "sql:"+userDB, "-N", "--empty-password")
		if err != nil {
			warnIfErr("初始化 NSS 用户数据库失败", err)
			return
		}
		fmt.Printf("已创建 NSS 用户数据库: %s (密码为空)\n", userDB)
	}
	_ = runAsUser(desktopUser, nil, "certutil", "-d", "sql:"+userDB, "-D", "-n", linuxCertNickname)
	warnIfErr("添加 NSS 用户证书失败", runAsUser(desktopUser, nil, "certutil", "-d", "sql:"+userDB, "-A", "-n", linuxCertNickname, "-t", "CT,C,C", "-i", certPath))
}

func uninstallNSSCerts() {
	if !commandExists("certutil") {
		return
	}
	systemDB := "/etc/pki/nssdb"
	if _, err := os.Stat(systemDB); err == nil {
		_ = runPrivileged(nil, "certutil", "-d", "sql:"+systemDB, "-D", "-n", linuxCertNickname)
	}
	desktopUser, err := currentDesktopUser()
	if err != nil || desktopUser.HomeDir == "" {
		warnIfErr("获取登录用户失败，跳过 NSS 用户证书库", err)
		return
	}
	userDB := filepath.Join(desktopUser.HomeDir, ".pki", "nssdb")
	if _, err := os.Stat(userDB); err == nil {
		_ = runAsUser(desktopUser, nil, "certutil", "-d", "sql:"+userDB, "-D", "-n", linuxCertNickname)
	}
}

func fetchCertificates() ([]Certificate, error) {
	var certs []Certificate
	paths := []string{
		"/etc/ssl/certs",
		"/usr/local/share/ca-certificates",
		"/etc/ca-certificates/trust-source/anchors", // Arch Linux
		"/etc/pki/ca-trust/source/anchors",          // Fedora/RHEL
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
	store, err := detectLinuxCertStore()
	if err != nil {
		return err
	}

	if err := writeSystemCert(store.certPath, cert); err != nil {
		return err
	}

	if err := runPrivileged(nil, store.updateCmd...); err != nil {
		return fmt.Errorf("%s: %v", store.updateErrMsg, err)
	}

	// NSS 证书库处理 (Firefox/Chromium)，失败不影响系统 CA 安装结果。
	installSystemNSSCert(store.certPath)
	installUserNSSCert(store.certPath)
	return nil
}

func uninstallCertificate(name string) error {
	store, err := detectLinuxCertStore()
	if err != nil {
		return err
	}

	if err := removeSystemCert(store.certPath); err != nil {
		return fmt.Errorf("删除证书失败: %v", err)
	}
	if err := runPrivileged(nil, store.updateCmd...); err != nil {
		return fmt.Errorf("%s: %v", store.updateErrMsg, err)
	}
	uninstallNSSCerts()
	return nil
}
