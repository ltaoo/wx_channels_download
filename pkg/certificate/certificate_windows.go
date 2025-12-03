//go:build windows

package certificate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func fetchCertificates() ([]Certificate, error) {
	cmd := "$certs = Get-ChildItem Cert:\\LocalMachine\\Root | Select-Object Thumbprint, Subject; @($certs) | ConvertTo-Json -Compress"
	ps := exec.Command("powershell.exe", "-NoProfile", "-Command", cmd)
	output, err2 := ps.CombinedOutput()
	if err2 != nil {
		return nil, errors.New(fmt.Sprintf("获取证书时发生错误，%v\n", err2.Error()))
	}
	type WindowsCert struct {
		Thumbprint string `json:"Thumbprint"`
		Subject    string `json:"Subject"`
	}
	var raw_arr []WindowsCert
	if err := json.Unmarshal(output, &raw_arr); err != nil {
		var one WindowsCert
		if err2 := json.Unmarshal(output, &one); err2 != nil {
			return nil, errors.New(fmt.Sprintf("解析证书列表失败，%v\n", err.Error()))
		}
		raw_arr = []WindowsCert{one}
	}
	var certificates []Certificate
	for _, pc := range raw_arr {
		subj := CertificateSubject{}
		pairs := strings.Split(pc.Subject, ",")
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
			Thumbprint: pc.Thumbprint,
			Subject:    subj,
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
