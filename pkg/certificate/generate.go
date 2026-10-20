package certificate

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"time"
)

const LocalCAName = "wx_channels_download"

func GenerateRootCA(name string, valid_for time.Duration) ([]byte, []byte, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = LocalCAName
	}
	if valid_for <= 0 {
		valid_for = 10 * 365 * 24 * time.Hour
	}

	private_key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	serial_limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial_number, err := rand.Int(rand.Reader, serial_limit)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	template := x509.Certificate{
		SerialNumber: serial_number,
		Subject: pkix.Name{
			CommonName:   name,
			Organization: []string{"dm.funzm.com"},
		},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(valid_for),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}

	cert_der, err := x509.CreateCertificate(rand.Reader, &template, &template, &private_key.PublicKey, private_key)
	if err != nil {
		return nil, nil, err
	}
	cert_pem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert_der})
	key_pem := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(private_key)})
	return cert_pem, key_pem, nil
}
