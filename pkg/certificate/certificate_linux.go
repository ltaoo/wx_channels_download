//go:build linux

package certificate

import (
	"crypto/sha256"
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

const linux_cert_nickname = "WeChatAppEx_CA"

type linux_cert_store struct {
	cert_path      string
	update_cmd     []string
	update_err_msg string
}

// Check if running as root
func is_root() bool {
	return os.Geteuid() == 0
}

func command_exists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func detect_linux_cert_store() (*linux_cert_store, error) {
	if command_exists("update-ca-certificates") {
		return &linux_cert_store{
			cert_path:      "/usr/local/share/ca-certificates/" + linux_cert_nickname + ".crt",
			update_cmd:     []string{"update-ca-certificates", "--fresh"},
			update_err_msg: "更新 OpenSSL 证书库失败",
		}, nil
	}
	if command_exists("update-ca-trust") {
		return &linux_cert_store{
			cert_path:      "/etc/pki/ca-trust/source/anchors/" + linux_cert_nickname + ".crt",
			update_cmd:     []string{"update-ca-trust", "extract"},
			update_err_msg: "更新证书库失败 (update-ca-trust)",
		}, nil
	}
	if command_exists("trust") {
		return &linux_cert_store{
			cert_path:      "/etc/ca-certificates/trust-source/anchors/" + linux_cert_nickname + ".crt",
			update_cmd:     []string{"trust", "extract-compat"},
			update_err_msg: "更新证书库失败 (trust)",
		}, nil
	}
	return nil, fmt.Errorf("未找到支持的证书更新命令！ (update-ca-certificates、update-ca-trust 或 trust)")
}

func run_command(stdin io.Reader, args ...string) error {
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

func run_privileged(stdin io.Reader, args ...string) error {
	if is_root() {
		return run_command(stdin, args...)
	}
	fmt.Println("Administrator privileges required, requesting...")
	return run_command(stdin, append([]string{"sudo"}, args...)...)
}

func current_desktop_user() (*user.User, error) {
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
	if pkexec_uid := strings.TrimSpace(os.Getenv("PKEXEC_UID")); pkexec_uid != "" {
		if u, err := user.LookupId(pkexec_uid); err == nil {
			return u, nil
		}
	}
	if u, err := user.Current(); err == nil {
		return u, nil
	}
	return nil, fmt.Errorf("获取登录用户失败: 无法从 SUDO_USER/USER/LOGNAME/PKEXEC_UID 获取用户")
}

func run_as_user(u *user.User, stdin io.Reader, args ...string) error {
	if u == nil {
		return run_command(stdin, args...)
	}
	if strconv.Itoa(os.Geteuid()) == u.Uid {
		cmd := append([]string{}, args...)
		return run_command(stdin, cmd...)
	}
	env_args := append([]string{"env", "HOME=" + u.HomeDir}, args...)
	if is_root() && command_exists("runuser") {
		return run_command(stdin, append([]string{"runuser", "-u", u.Username, "--"}, env_args...)...)
	}
	return run_command(stdin, append([]string{"sudo", "-u", u.Username}, env_args...)...)
}

func write_system_cert(path string, cert []byte) error {
	if is_root() {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("创建证书目录失败: %v", err)
		}
		if err := os.WriteFile(path, cert, 0644); err != nil {
			return fmt.Errorf("写入证书失败: %v", err)
		}
		return nil
	}

	tmp, err := os.CreateTemp("", linux_cert_nickname+"-*.crt")
	if err != nil {
		return fmt.Errorf("创建临时证书失败: %v", err)
	}
	tmp_path := tmp.Name()
	defer os.Remove(tmp_path)
	if _, err := tmp.Write(cert); err != nil {
		tmp.Close()
		return fmt.Errorf("写入临时证书失败: %v", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时证书失败: %v", err)
	}
	if err := os.Chmod(tmp_path, 0644); err != nil {
		return fmt.Errorf("设置临时证书权限失败: %v", err)
	}
	if err := run_privileged(nil, "mkdir", "-p", filepath.Dir(path)); err != nil {
		return fmt.Errorf("创建证书目录失败: %v", err)
	}
	if err := run_privileged(nil, "install", "-m", "0644", tmp_path, path); err != nil {
		return fmt.Errorf("写入证书失败: %v", err)
	}
	return nil
}

func remove_system_cert(path string) error {
	if is_root() {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return run_privileged(nil, "rm", "-f", path)
}

func warn_if_err(prefix string, err error) {
	if err != nil {
		fmt.Printf("Warning: %s: %v\n", prefix, err)
	}
}

func install_system_nss_cert(cert_path string) {
	if !command_exists("certutil") {
		return
	}

	system_db := "/etc/pki/nssdb"
	if _, err := os.Stat(system_db); os.IsNotExist(err) {
		fmt.Printf("NSS system database not found: %s, creating...\n", system_db)
		if err := run_privileged(nil, "mkdir", "-p", system_db); err != nil {
			warn_if_err("Failed to create NSS system database directory", err)
			return
		}
		warn_if_err("Failed to set permissions on NSS system database directory", run_privileged(nil, "chmod", "700", system_db))
		err := run_privileged(strings.NewReader("\n\n"), "certutil", "-d", "sql:"+system_db, "-N", "--empty-password")
		if err != nil {
			warn_if_err("Failed to initialize NSS system database", err)
			return
		}
		fmt.Printf("NSS system database created: %s (empty password)\n", system_db)
	}
	_ = run_privileged(nil, "certutil", "-d", "sql:"+system_db, "-D", "-n", linux_cert_nickname)
	warn_if_err("Failed to add NSS system certificate", run_privileged(nil, "certutil", "-d", "sql:"+system_db, "-A", "-n", linux_cert_nickname, "-t", "CT,C,C", "-i", cert_path))
}

func install_user_nss_cert(cert_path string) {
	if !command_exists("certutil") {
		return
	}
	desktop_user, err := current_desktop_user()
	if err != nil || desktop_user.HomeDir == "" {
		warn_if_err("Failed to get login user, skipping NSS user certificate store", err)
		return
	}
	user_db := filepath.Join(desktop_user.HomeDir, ".pki", "nssdb")
	if _, err := os.Stat(user_db); os.IsNotExist(err) {
		fmt.Printf("NSS user database not found: %s, creating...\n", user_db)
		if err := run_as_user(desktop_user, nil, "mkdir", "-p", user_db); err != nil {
			warn_if_err("Failed to create NSS user database directory", err)
			return
		}
		warn_if_err("Failed to set permissions on NSS user database directory", run_as_user(desktop_user, nil, "chmod", "700", user_db))
		err := run_as_user(desktop_user, strings.NewReader("\n\n"), "certutil", "-d", "sql:"+user_db, "-N", "--empty-password")
		if err != nil {
			warn_if_err("Failed to initialize NSS user database", err)
			return
		}
		fmt.Printf("NSS user database created: %s (empty password)\n", user_db)
	}
	_ = run_as_user(desktop_user, nil, "certutil", "-d", "sql:"+user_db, "-D", "-n", linux_cert_nickname)
	warn_if_err("Failed to add NSS user certificate", run_as_user(desktop_user, nil, "certutil", "-d", "sql:"+user_db, "-A", "-n", linux_cert_nickname, "-t", "CT,C,C", "-i", cert_path))
}

func uninstall_nss_certs() {
	if !command_exists("certutil") {
		return
	}
	system_db := "/etc/pki/nssdb"
	if _, err := os.Stat(system_db); err == nil {
		_ = run_privileged(nil, "certutil", "-d", "sql:"+system_db, "-D", "-n", linux_cert_nickname)
	}
	desktop_user, err := current_desktop_user()
	if err != nil || desktop_user.HomeDir == "" {
		warn_if_err("Failed to get login user, skipping NSS user certificate store", err)
		return
	}
	user_db := filepath.Join(desktop_user.HomeDir, ".pki", "nssdb")
	if _, err := os.Stat(user_db); err == nil {
		_ = run_as_user(desktop_user, nil, "certutil", "-d", "sql:"+user_db, "-D", "-n", linux_cert_nickname)
	}
}

func fetch_certificates() ([]Certificate, error) {
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
					Thumbprint: fmt.Sprintf("%X", sha256.Sum256(cert.Raw)),
					Subject:    CertificateSubject{CN: cert.Subject.CommonName},
				})
			}
			return nil
		})
	}
	return certs, nil
}

func install_certificate(cert []byte) error {
	store, err := detect_linux_cert_store()
	if err != nil {
		return err
	}

	if err := write_system_cert(store.cert_path, cert); err != nil {
		return err
	}

	if err := run_privileged(nil, store.update_cmd...); err != nil {
		return fmt.Errorf("%s: %v", store.update_err_msg, err)
	}

	// NSS certificate store handling (Firefox/Chromium); failures do not affect system CA installation.
	install_system_nss_cert(store.cert_path)
	install_user_nss_cert(store.cert_path)
	return nil
}

func check_certificate_trusted(cert_name string) (bool, error) {
	// On Linux, a cert installed via update-ca-certificates is considered trusted.
	installed, err := CheckHasCertificate(cert_name)
	if err != nil {
		return false, err
	}
	return installed, nil
}

func check_certificate_data_trusted(_ []byte, cert_name string) (bool, error) {
	return check_certificate_trusted(cert_name)
}

func uninstall_certificate(name string) error {
	store, err := detect_linux_cert_store()
	if err != nil {
		return err
	}

	if err := remove_system_cert(store.cert_path); err != nil {
		return fmt.Errorf("删除证书失败: %v", err)
	}
	if err := run_privileged(nil, store.update_cmd...); err != nil {
		return fmt.Errorf("%s: %v", store.update_err_msg, err)
	}
	uninstall_nss_certs()
	return nil
}
