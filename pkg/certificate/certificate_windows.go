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
	// PowerShell 2.0 compatible command
	cmd := "Get-ChildItem Cert:\\LocalMachine\\Root | ForEach-Object { $_.Thumbprint + \"###\" + $_.Subject }"
	ps := exec.Command("powershell.exe", "-NoProfile", "-Command", cmd)
	output, err2 := ps.CombinedOutput()
	if err2 != nil {
		return nil, fmt.Errorf("获取证书时发生错误，%v\n", err2.Error())
	}

	var certificates []Certificate
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "###", 2)
		if len(parts) < 2 {
			continue
		}
		thumbprint := parts[0]
		subject_str := parts[1]

		subj := CertificateSubject{}
		pairs := strings.Split(subject_str, ",")
		for _, p := range pairs {
			kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
			if len(kv) != 2 {
				continue
			}
			key := kv[0]
			value := kv[1]
			switch key {
			case "CN":
				subj.CN = value
			case "OU":
				subj.OU = value
			case "O":
				subj.O = value
			case "L":
				subj.L = value
			case "S":
				subj.S = value
			case "C":
				subj.C = value
			}
		}
		certificates = append(certificates, Certificate{
			Thumbprint: thumbprint,
			Subject:    subj,
		})
	}
	return certificates, nil
}

func installCertificate(cert_data []byte) error {
	cert_file, err := os.CreateTemp("", "SunnyRoot.cer")
	if err != nil {
		return fmt.Errorf("没有创建证书的权限，%v\n", err.Error())
	}
	defer os.Remove(cert_file.Name())
	if _, err := cert_file.Write(cert_data); err != nil {
		return fmt.Errorf("获取证书失败，%v\n", err.Error())
	}
	if err := cert_file.Close(); err != nil {
		return fmt.Errorf("生成证书失败，%v\n", err.Error())
	}
	// Use certutil for Windows 7 compatibility
	cmd := exec.Command("certutil", "-addstore", "Root", cert_file.Name())
	output, err2 := cmd.CombinedOutput()
	if err2 != nil {
		return fmt.Errorf("安装证书时发生错误，%v\n", string(output))
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
	ps := exec.Command("powershell.exe", "-NoProfile", "-Command", cmd)
	output, err2 := ps.CombinedOutput()
	if err2 != nil {
		return fmt.Errorf("删除证书时发生错误，%v\n", string(output))
	}
	return nil
}
