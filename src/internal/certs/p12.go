package certs

import (
	"crypto/x509"
	"fmt"
	"os"

	pkcs12 "software.sslmate.com/src/go-pkcs12"

	"clipshare/src/internal/consts"
)

// ExportP12 writes a PKCS#12 bundle (key + leaf + CA chain) that the Android
// app can import, e.g. for a client device.
func ExportP12(dir, name, kind, out string) error {
	der, err := P12Bytes(dir, name, kind)
	if err != nil {
		return err
	}
	return os.WriteFile(out, der, 0o600)
}

// P12Bytes returns the PKCS#12 bundle (key + leaf + CA chain) for a device.
func P12Bytes(dir, name, kind string) ([]byte, error) {
	d, err := loadDevice(dir, name, kind, true)
	if err != nil {
		return nil, err
	}
	// Legacy (3DES + SHA-1) is used deliberately: Android's bundled
	// BouncyCastle PKCS#12 parser does not handle Modern's PBMAC1/AES
	// bags on all API levels. Legacy is universally supported.
	der, err := pkcs12.Legacy.Encode(d.key, d.cert, []*x509.Certificate{d.ca}, consts.P12Password)
	if err != nil {
		return nil, fmt.Errorf("pkcs12 encode: %w", err)
	}
	return der, nil
}
