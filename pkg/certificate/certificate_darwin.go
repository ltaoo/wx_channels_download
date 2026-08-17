//go:build darwin

package certificate

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

func fetchCertificates() ([]Certificate, error) {
	cmd := exec.Command("security", "find-certificate", "-a")
	output, err2 := cmd.Output()
	if err2 != nil {
		return nil, errors.New(fmt.Sprintf("failed to fetch certificates: %v\n", err2.Error()))
	}
	var certificates []Certificate
	lines := strings.Split(string(output), "\n")
	for i := 0; i < len(lines)-1; i += 13 {
		if lines[i] == "" {
			continue
		}
		// if i > len(lines)-1 {
		// 	continue
		// }
		cenc := lines[i+5]
		ctyp := lines[i+6]
		hpky := lines[i+7]
		labl := lines[i+9]
		subj := lines[i+12]
		re := regexp.MustCompile(`="([^"]{1,})"`)
		// Find matching strings
		matches := re.FindStringSubmatch(labl)
		if len(matches) < 1 {
			continue
		}
		label := matches[1]
		certificates = append(certificates, Certificate{
			Thumbprint: "",
			Subject: CertificateSubject{
				CN: label,
				OU: cenc,
				O:  ctyp,
				L:  hpky,
				S:  subj,
				C:  cenc,
			},
		})
	}
	return certificates, nil
}

func installCertificate(cert_data []byte) error {
	cert_file, err := os.CreateTemp("", "SunnyNet.cer")
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

	// execute-with-privileges acquires an interactive AuthorizationRef in the
	// logged-in user's security session and forwards it to the root child. This
	// lets the child both write System.keychain and update admin trust settings.
	// A plain root process created by `do shell script` has no such authorization
	// session, while a direct unprivileged call cannot write System.keychain.
	ps := new_install_certificate_command(cert_file.Name())
	output, install_err := ps.CombinedOutput()
	if install_err != nil {
		output_text := strings.TrimSpace(string(output))
		if output_text == "" {
			return fmt.Errorf("certificate installation authorization command failed: %w", install_err)
		}
		return fmt.Errorf("certificate installation authorization command failed: %w; output: %s", install_err, output_text)
	}
	return nil
}

func new_install_certificate_command(cert_path string) *exec.Cmd {
	return exec.Command(
		"/usr/bin/security",
		"execute-with-privileges",
		"/usr/bin/security",
		"add-trusted-cert",
		"-d",
		"-r",
		"trustRoot",
		"-k",
		"/Library/Keychains/System.keychain",
		cert_path,
	)
}

func check_certificate_trusted(cert_name string) (bool, error) {
	find_command := exec.Command("/usr/bin/security", "find-certificate", "-c", cert_name, "-p")
	cert_data, find_err := find_command.CombinedOutput()
	if find_err != nil {
		return false, nil
	}
	return check_certificate_data_trusted(cert_data, cert_name)
}

type certificate_verify_runner func(cert_path string) ([]byte, error)

func check_certificate_data_trusted(cert_data []byte, _ string) (bool, error) {
	return check_certificate_data_trusted_with_runner(cert_data, func(cert_path string) ([]byte, error) {
		verify_command := exec.Command(
			"/usr/bin/security",
			"verify-cert",
			"-c",
			cert_path,
			"-p",
			"basic",
			"-l",
			"-L",
			"-q",
		)
		return verify_command.CombinedOutput()
	})
}

func check_certificate_data_trusted_with_runner(cert_data []byte, run_verify certificate_verify_runner) (bool, error) {
	if len(cert_data) == 0 {
		return false, errors.New("certificate data is empty")
	}

	cert_file, create_err := os.CreateTemp("", "wx_channels_download-root-ca-*.cer")
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

func uninstallCertificate(certificate_name string) error {
	certificates, err := fetchCertificates()
	if err != nil {
		return err
	}
	var matched *Certificate
	for _, cert := range certificates {
		if cert.Subject.CN == certificate_name {
			matched = &cert
			break
		}
	}
	if matched == nil {
		return errors.New("no matching root certificate found")
	}
	// Use osascript with administrator privileges to modify the system keychain.
	escaped := strings.ReplaceAll(certificate_name, "'", "'\\''")
	script := fmt.Sprintf(`do shell script "security delete-certificate -c '%s'" with administrator privileges`, escaped)
	ps := exec.Command("osascript", "-e", script)
	output, err2 := ps.CombinedOutput()
	if err2 != nil {
		return errors.New(fmt.Sprintf("failed to delete the certificate: %v\n", string(output)))
	}
	return nil
}
