//go:build windows

package certificate

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func fetchCertificates() ([]Certificate, error) {
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
			subject := CertificateSubject{}
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
	cmd := fmt.Sprintf("Import-Certificate -FilePath '%s' -CertStoreLocation Cert:\\LocalMachine\\Root", cert_file.Name())
	ps := exec.Command("powershell.exe", "-Command", cmd)
	output, err2 := ps.CombinedOutput()
	if err2 != nil {
		return errors.New(fmt.Sprintf("安装证书时发生错误，%v\n", string(output)))
	}
	return nil
}

func uninstallCertificate(name string) error {
	fmt.Println(name)
	// Remove-Item "Cert:\LocalMachine\Root\D70CD039051F77C30673B8209FC15EFA650ED52C"
	certificates, err := fetchCertificates()
	if err != nil {
		return err
	}
	var matched *Certificate
	for _, cert := range certificates {
		if cert.Subject.CN == name {
			matched = &cert
			break
		}
	}
	if matched == nil {
		return errors.New("没有找到要删除的证书")
	}
	cmd := fmt.Sprintf("Get-ChildItem Cert:\\LocalMachine\\Root\\%v | Remove-Item", matched.Thumbprint)
	ps := exec.Command("powershell.exe", "-Command", cmd)
	output, err2 := ps.CombinedOutput()
	if err2 != nil {
		return errors.New(fmt.Sprintf("删除证书时发生错误，%v\n", string(output)))
	}
	return nil
}
