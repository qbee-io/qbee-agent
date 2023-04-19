package remoteaccess

import "strings"

type Credentials struct {
	// CA - the CA certificate as a slice of new-lines in PEM format.
	CA []string `json:"vpn_ca_cert"`

	// Certificate - the certificate as a slice of new-lines in PEM format.
	Certificate []string `json:"vpn_cert"`

	// Expiry - time of the certificate expiration in seconds since epoch.
	Expiry int64 `json:"vpn_cert_expiry"`

	Status string `json:"status"`
}

// CertificatePEM returns the certificate in PEM format.
func (c Credentials) CertificatePEM() []byte {
	return []byte(strings.Join(c.Certificate, "\n"))
}

// CACertificatePEM returns the CA certificate in PEM format.
func (c Credentials) CACertificatePEM() []byte {
	return []byte(strings.Join(c.CA, "\n"))
}
