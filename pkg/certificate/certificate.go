package certificate

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

type Subject struct {
	CN string
	OU string
	O  string
	L  string
	S  string
	C  string
}
type Certificate struct {
	Thumbprint string
	Subject    Subject
}

func fetchCertificatesInWindows() ([]Certificate, error) {
	// 获取指定 store 所有证书
	cmd := fmt.Sprintf("Get-ChildItem Cert:\\LocalMachine\\Root")
	ps := exec.Command("powershell.exe", "-Command", cmd)
	output, err2 := ps.CombinedOutput()
	if err2 != nil {
		return nil, errors.New(fmt.Sprintf("获取证书时发生错误，%v\n", err2.Error()))
	}
	var certificates []Certificate
	lines := strings.Split(string(output), "\n")
	// 跳过前两行（列名）
	for i := 2; i < len(lines)-1; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		parts := strings.Split(line, " ")
		if len(parts) >= 2 {
			subject := Subject{}
			for _, part := range parts[1:] {
				part = strings.Replace(part, ",", "", 1)
				kv := strings.Split(part, "=")
				if len(kv) == 2 {
					key := strings.TrimSpace(kv[0])
					value := strings.TrimSpace(kv[1])
					switch key {
					case "CN":
						subject.CN = value
					case "OU":
						subject.OU = value
					case "O":
						subject.O = value
					case "L":
						subject.L = value
					case "S":
						subject.S = value
					case "C":
						subject.C = value
					}
				}
			}
			certificates = append(certificates, Certificate{
				Thumbprint: parts[0],
				Subject:    subject,
			})
		}
	}
	return certificates, nil
}
func fetchCertificatesInMacOS() ([]Certificate, error) {
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
		// 找到匹配的字符串
		matches := re.FindStringSubmatch(labl)
		if len(matches) < 1 {
			continue
		}
		label := matches[1]
		certificates = append(certificates, Certificate{
			Thumbprint: "",
			Subject: Subject{
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

func fetchCertificates() ([]Certificate, error) {
	os_env := runtime.GOOS
	switch os_env {
	case "linux":
		fmt.Println("Running on Linux")
	case "darwin":
		return fetchCertificatesInMacOS()
	case "windows":
		return fetchCertificatesInWindows()
	default:
		fmt.Printf("Running on %s\n", os_env)
	}
	return nil, errors.New(fmt.Sprintf("unknown OS\n"))

}
func CheckCertificate(cert_name string) (bool, error) {
	certificates, err := fetchCertificates()
	if err != nil {
		return false, err
	}
	for _, cert := range certificates {
		if cert.Subject.CN == cert_name {
			return true, nil
		}
	}
	return false, nil
}
func removeCertificate() {
	// 删除指定证书
	// Remove-Item "Cert:\LocalMachine\Root\D70CD039051F77C30673B8209FC15EFA650ED52C"
}
func installCertificateInWindows(cert_data []byte) error {
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
	cmd := fmt.Sprintf("Import-Certificate -FilePath '%s' -CertStoreLocation Cert:\\LocalMachine\\Root", cert_file.Name())
	ps := exec.Command("powershell.exe", "-Command", cmd)
	output, err2 := ps.CombinedOutput()
	if err2 != nil {
		return errors.New(fmt.Sprintf("安装证书时发生错误，%v\n", output))
	}
	return nil
}
func installCertificateInMacOS(cert_data []byte) error {
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
		return errors.New(fmt.Sprintf("安装证书时发生错误，%v\n", output))
	}
	return nil
}

func InstallCertificate(cert_data []byte) error {
	os_env := runtime.GOOS
	switch os_env {
	case "linux":
		fmt.Println("Running on Linux")
	case "darwin":
		return installCertificateInMacOS(cert_data)
	case "windows":
		return installCertificateInWindows(cert_data)
	default:
		fmt.Printf("Running on %s\n", os_env)
	}
	return errors.New(fmt.Sprintf("unknown OS\n"))
}
