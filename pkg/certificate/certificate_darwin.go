//go:build darwin

package certificate

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

func certificate_user() (*user.User, error) {
	if os.Geteuid() == 0 {
		if sudo_uid := os.Getenv("SUDO_UID"); sudo_uid != "" && sudo_uid != "0" {
			return user.LookupId(sudo_uid)
		}
		return nil, errors.New("无法确定证书所属用户，请以普通用户或通过 sudo 启动")
	}
	return user.Current()
}

func new_certificate_user_command(cert_user *user.User, args ...string) *exec.Cmd {
	if cert_user.Uid != strconv.Itoa(os.Geteuid()) {
		return exec.Command("/usr/bin/sudo", append([]string{"-H", "-u", "#" + cert_user.Uid, "--", "/usr/bin/security"}, args...)...)
	}
	return exec.Command("/usr/bin/security", args...)
}

func fetch_certificates() ([]Certificate, error) {
	cert_user, err := certificate_user()
	if err != nil {
		return nil, err
	}
	output, err := new_certificate_user_command(cert_user, "find-certificate", "-a", "-p").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch certificates: %w", err)
	}
	return parse_keychain_certificates(output)
}

func parse_keychain_certificates(data []byte) ([]Certificate, error) {
	var certificates []Certificate
	for {
		block, rest := pem.Decode(data)
		if block == nil {
			return certificates, nil
		}
		data = rest
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse keychain certificate: %w", err)
		}
		certificates = append(certificates, Certificate{
			Thumbprint: fmt.Sprintf("%X", sha256.Sum256(cert.Raw)),
			Subject: CertificateSubject{
				CN: cert.Subject.CommonName,
				OU: strings.Join(cert.Subject.OrganizationalUnit, ", "),
				O:  strings.Join(cert.Subject.Organization, ", "),
				L:  strings.Join(cert.Subject.Locality, ", "),
				S:  strings.Join(cert.Subject.Province, ", "),
				C:  strings.Join(cert.Subject.Country, ", "),
			},
		})
	}
}

func install_certificate(cert_data []byte) error {
	cert_file, err := os.CreateTemp("/tmp", "wx_channels_download-root-ca-*.cer")
	if err != nil {
		return fmt.Errorf("permission denied while creating the certificate file: %w", err)
	}
	defer func() {
		_ = os.Remove(cert_file.Name())
	}()
	if _, err := cert_file.Write(cert_data); err != nil {
		_ = cert_file.Close()
		return fmt.Errorf("failed to write the certificate: %w", err)
	}
	if err := cert_file.Close(); err != nil {
		return fmt.Errorf("failed to generate the certificate: %w", err)
	}

	// sudo starts create root-owned files; only the public certificate must be readable by the user.
	if err := os.Chmod(cert_file.Name(), 0644); err != nil {
		return fmt.Errorf("failed to set certificate permissions: %w", err)
	}
	ps, err := new_install_certificate_command(cert_file.Name())
	if err != nil {
		return err
	}
	output, install_err := ps.CombinedOutput()
	if install_err != nil {
		output_text := strings.TrimSpace(string(output))
		if output_text == "" {
			return fmt.Errorf("user certificate installation failed: %w", install_err)
		}
		return fmt.Errorf("user certificate installation failed: %w; output: %s", install_err, output_text)
	}
	return nil
}

func new_install_certificate_command(cert_path string) (*exec.Cmd, error) {
	cert_user, err := certificate_user()
	if err != nil {
		return nil, err
	}
	// Omitting -d selects user trust; omitting -p keeps both SSL and basic validation working.
	return new_certificate_user_command(cert_user,
		"add-trusted-cert", "-r", "trustRoot",
		"-k", filepath.Join(cert_user.HomeDir, "Library", "Keychains", "login.keychain-db"),
		cert_path,
	), nil
}

func check_certificate_trusted(cert_name string) (bool, error) {
	cert_user, err := certificate_user()
	if err != nil {
		return false, err
	}
	find_command := new_certificate_user_command(cert_user, "find-certificate", "-c", cert_name, "-p")
	cert_data, find_err := find_command.CombinedOutput()
	if find_err != nil {
		return false, nil
	}
	return check_certificate_data_trusted(cert_data, cert_name)
}

type certificate_verify_runner func(cert_path string) ([]byte, error)

func check_certificate_data_trusted(cert_data []byte, _ string) (bool, error) {
	return check_certificate_data_trusted_with_runner(cert_data, func(cert_path string) ([]byte, error) {
		cert_user, err := certificate_user()
		if err != nil {
			return nil, err
		}
		verify_command := new_certificate_user_command(cert_user,
			"verify-cert", "-c", cert_path, "-p", "basic", "-l", "-L", "-q",
		)
		return verify_command.CombinedOutput()
	})
}

func check_certificate_data_trusted_with_runner(cert_data []byte, run_verify certificate_verify_runner) (bool, error) {
	if len(cert_data) == 0 {
		return false, errors.New("certificate data is empty")
	}

	cert_file, create_err := os.CreateTemp("/tmp", "wx_channels_download-root-ca-*.cer")
	if create_err != nil {
		return false, fmt.Errorf("failed to create temporary certificate file: %w", create_err)
	}
	cert_path := cert_file.Name()
	defer func() {
		_ = os.Remove(cert_path)
	}()

	if _, write_err := cert_file.Write(cert_data); write_err != nil {
		_ = cert_file.Close()
		return false, fmt.Errorf("failed to write temporary certificate file: %w", write_err)
	}
	if close_err := cert_file.Close(); close_err != nil {
		return false, fmt.Errorf("failed to close temporary certificate file: %w", close_err)
	}

	if err := os.Chmod(cert_path, 0644); err != nil {
		return false, fmt.Errorf("failed to set temporary certificate permissions: %w", err)
	}

	output, verify_err := run_verify(cert_path)
	if verify_err == nil {
		return true, nil
	}
	var exit_err *exec.ExitError
	if errors.As(verify_err, &exit_err) {
		return false, nil
	}
	output_text := strings.TrimSpace(string(output))
	if output_text == "" {
		return false, fmt.Errorf("failed to verify certificate trust: %w", verify_err)
	}
	return false, fmt.Errorf("failed to verify certificate trust: %w; output: %s", verify_err, output_text)
}

func uninstall_certificate(certificate_name string) error {
	cert_user, err := certificate_user()
	if err != nil {
		return err
	}
	user_keychain := filepath.Join(cert_user.HomeDir, "Library", "Keychains", "login.keychain-db")
	// Delete user trust with the certificate; older system roots still require authorization.
	for _, keychain := range []string{user_keychain, "/Library/Keychains/System.keychain"} {
		output, find_err := new_certificate_user_command(cert_user, "find-certificate", "-a", "-p", keychain).Output()
		if find_err != nil {
			return fmt.Errorf("failed to read keychain %s: %w", keychain, find_err)
		}
		certificates, err := parse_keychain_certificates(output)
		if err != nil {
			return err
		}
		for _, cert := range certificates {
			if cert.Subject.CN != certificate_name {
				continue
			}
			ps := new_uninstall_certificate_command(cert_user, keychain, cert.Thumbprint)
			output, err := ps.CombinedOutput()
			if err != nil {
				return fmt.Errorf("failed to delete certificate: %w; output: %s", err, strings.TrimSpace(string(output)))
			}
			return nil
		}
	}
	return errors.New("no matching root certificate found")
}

func new_uninstall_certificate_command(cert_user *user.User, keychain string, thumbprint string) *exec.Cmd {
	args := []string{"delete-certificate", "-t", "-Z", thumbprint, keychain}
	if keychain == "/Library/Keychains/System.keychain" {
		return exec.Command("/usr/bin/security", append([]string{"execute-with-privileges", "/usr/bin/security"}, args...)...)
	}
	return new_certificate_user_command(cert_user, args...)
}
