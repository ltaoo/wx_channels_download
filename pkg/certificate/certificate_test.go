package certificate

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"
	"time"
)

func TestGeneratedRootAndFingerprint(t *testing.T) {
	cert_pem, key_pem, err := GenerateRootCA("", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	pair, err := tls.X509KeyPair(cert_pem, key_pem)
	if err != nil {
		t.Fatal(err)
	}
	root, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if !root.IsCA || root.Subject.CommonName != LocalCAName || root.Issuer.CommonName != LocalCAName || len(root.Subject.Organization) != 1 || root.Subject.Organization[0] != "dm.funzm.com" {
		t.Fatalf("unexpected root identity: %v", root.Subject)
	}
	if err := root.CheckSignatureFrom(root); err != nil {
		t.Fatal(err)
	}
	for _, thumbprint := range []string{fmt.Sprintf("%X", sha256.Sum256(root.Raw)), fmt.Sprintf("%X", sha1.Sum(root.Raw))} {
		installed, err := check_has_certificate_data(cert_pem, []Certificate{{Thumbprint: thumbprint}})
		if err != nil || !installed {
			t.Fatalf("exact fingerprint must match: %v", err)
		}
	}
	installed, err := check_has_certificate_data(cert_pem, []Certificate{{Subject: CertificateSubject{CN: LocalCAName}, Thumbprint: "different"}})
	if err != nil || installed {
		t.Fatalf("same name with a different key must not match: %v", err)
	}
	block, _ := pem.Decode(DefaultCertFiles.Cert)
	if block == nil || string(root.Raw) == string(block.Bytes) {
		t.Fatal("generated root must not reuse the shared certificate")
	}
}
