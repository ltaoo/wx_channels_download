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
		return nil, errors.New(fmt.Sprintf("获取证书时发生错误，%v\n", err2.Error()))
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
	cert_file, err := os.CreateTemp("", "SunnyRoot.cer")
	if err != nil {
		return errors.New(fmt.Sprintf("没有创建证书的权限，%v\n", err.Error()))
	}
	defer os.Remove(cert_file.Name())
	if _, err := cert_file.Write(cert_data); err != nil {
		return errors.New(fmt.Sprintf("获取证书失败，%v\n", err.Error()))
	}
	if err := cert_file.Close(); err != nil {
		return errors.New(fmt.Sprintf("生成证书失败，%v\n", err.Error()))
	}
	cmd := fmt.Sprintf("security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain '%s'", cert_file.Name())
	ps := exec.Command("bash", "-c", cmd)
	output, err2 := ps.CombinedOutput()
	if err2 != nil {
		return errors.New(fmt.Sprintf("安装证书时发生错误，%v\n", string(output)))
	}
	return nil
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
		return errors.New("没有找到匹配的根证书")
	}
	// Use osascript with administrator privileges to modify the system keychain.
	escaped := strings.ReplaceAll(certificate_name, "'", "'\\''")
	script := fmt.Sprintf(`do shell script "security delete-certificate -c '%s'" with administrator privileges`, escaped)
	ps := exec.Command("osascript", "-e", script)
	output, err2 := ps.CombinedOutput()
	if err2 != nil {
		return errors.New(fmt.Sprintf("删除证书时发生错误，%v\n", string(output)))
	}
	return nil
}
