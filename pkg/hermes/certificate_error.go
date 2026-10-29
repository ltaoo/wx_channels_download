package hermes

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
)

// IsCertificateVerifyError reports whether err was caused by the peer's TLS
// certificate failing verification, for example an unknown authority, a
// hostname mismatch, or an expired certificate.
//
// The built-in HTTP drivers use this to replay a direct request over IPv4:
// a CDN edge may serve a healthy certificate on one address family and a
// wrong one on the other, and a TLS handshake failure never falls back to
// the next address.
func IsCertificateVerifyError(err error) bool {
	var verify_err *tls.CertificateVerificationError
	if errors.As(err, &verify_err) {
		return true
	}
	var unknown_authority_err x509.UnknownAuthorityError
	if errors.As(err, &unknown_authority_err) {
		return true
	}
	var hostname_err x509.HostnameError
	if errors.As(err, &hostname_err) {
		return true
	}
	var invalid_err x509.CertificateInvalidError
	if errors.As(err, &invalid_err) {
		return true
	}
	return false
}
