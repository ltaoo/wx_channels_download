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

func checkCertificateTrusted(cert_name string) (bool, error) {
	// Get SHA1 fingerprint using security find-certificate -c <name> -Z
	fpCmd := exec.Command("security", "find-certificate", "-c", cert_name, "-Z")
	fpOutput, err := fpCmd.CombinedOutput()
	if err != nil {
		return false, nil
	}
	// Parse SHA1 hash from output: lines with "SHA-1 hash:" prefix
	fingerprint := findSHA1Hash(string(fpOutput), cert_name)
	if fingerprint == "" {
		return false, nil
	}

	// Check trust settings for admin and user domains
	for _, args := range [][]string{{"dump-trust-settings"}, {"dump-trust-settings", "-d"}} {
		cmd := exec.Command("security", args...)
		output, err := cmd.CombinedOutput()
		text := strings.ToUpper(string(output))
		if err != nil {
			if strings.Contains(text, "NO TRUST SETTINGS") || strings.Contains(text, "NO KEYCHAIN IS AVAILABLE") {
				continue
			}
			continue
		}
		normalized := strings.NewReplacer(":", "", " ", "", "\n", "", "\t", "").Replace(text)
		fpUpper := strings.ToUpper(strings.NewReplacer(":", "", " ", "").Replace(fingerprint))
		if strings.Contains(normalized, fpUpper) &&
			(strings.Contains(text, "TRUST ROOT") || strings.Contains(text, "TRUSTROOT")) {
			return true, nil
		}
	}
	return false, nil
}

func findSHA1Hash(output string, certName string) string {
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if strings.Contains(line, "SHA-1 hash:") {
			// The hash is on the next line or after the colon
			hash := strings.TrimSpace(strings.TrimPrefix(line, "SHA-1 hash:"))
			if hash != "" {
				return hash
			}
			// Try next line
			if i+1 < len(lines) {
				return strings.TrimSpace(lines[i+1])
			}
		}
	}
	return ""
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
