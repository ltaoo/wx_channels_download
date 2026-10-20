package certificate

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"fmt"
	"strings"
)

//go:embed certs/SunnyNet.cer
var cert_file []byte

//go:embed certs/private.key
var private_key_file []byte

type CertFileAndKeyFile struct {
	Name       string
	Cert       []byte
	PrivateKey []byte
}

var DefaultCertFiles = &CertFileAndKeyFile{
	Name:       "SunnyNet",
	Cert:       cert_file,
	PrivateKey: private_key_file,
}

type CertificateSubject struct {
	// label
	CN string
	// cenc
	OU string
	// hpky
	O string
	// hpky
	L string
	// subj
	S string
	// cenc
	C string
}
type Certificate struct {
	Thumbprint string
	Subject    CertificateSubject
}

// Fetch all certificates
func FetchCertificates() ([]Certificate, error) {
	return fetch_certificates()
}

// Check if a certificate with the given name exists
func CheckHasCertificate(cert_name string) (bool, error) {
	certificates, err := fetch_certificates()
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

// CheckHasCertificateData distinguishes roots with the same name but different keys.
func CheckHasCertificateData(cert_data []byte) (bool, error) {
	certificates, err := fetch_certificates()
	if err != nil {
		return false, err
	}
	return check_has_certificate_data(cert_data, certificates)
}

func check_has_certificate_data(cert_data []byte, certificates []Certificate) (bool, error) {
	if block, _ := pem.Decode(cert_data); block != nil {
		cert_data = block.Bytes
	}
	cert, err := x509.ParseCertificate(cert_data)
	if err != nil {
		return false, fmt.Errorf("failed to parse root certificate: %w", err)
	}
	sha256_fingerprint := fmt.Sprintf("%X", sha256.Sum256(cert.Raw))
	// Windows exposes SHA-1 thumbprints; macOS and Linux expose SHA-256.
	sha1_fingerprint := fmt.Sprintf("%X", sha1.Sum(cert.Raw))
	for _, installed_cert := range certificates {
		fingerprint := strings.ToUpper(strings.ReplaceAll(installed_cert.Thumbprint, ":", ""))
		if fingerprint == sha256_fingerprint || fingerprint == sha1_fingerprint {
			return true, nil
		}
	}
	return false, nil
}

// Check if a certificate with the given name is trusted by the system
func CheckCertificateTrusted(cert_name string) (bool, error) {
	return check_certificate_trusted(cert_name)
}

// CheckCertificateDataTrusted checks whether the exact certificate is trusted by the system.
// cert_name is used as a fallback on platforms whose root stores are name-based.
func CheckCertificateDataTrusted(cert_data []byte, cert_name string) (bool, error) {
	return check_certificate_data_trusted(cert_data, cert_name)
}

// Install a certificate
func InstallCertificate(cert_data []byte) error {
	return install_certificate(cert_data)
}

// Uninstall a certificate by name
func UninstallCertificate(name string) error {
	return uninstall_certificate(name)
}
