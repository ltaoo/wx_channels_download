package certificate

import (
	_ "embed"
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
	return fetchCertificates()
}

// Check if a certificate with the given name exists
func CheckHasCertificate(cert_name string) (bool, error) {
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
	return installCertificate(cert_data)
}

// Uninstall a certificate by name
func UninstallCertificate(name string) error {
	return uninstallCertificate(name)
}
