package certs

import (
	"bytes"
	"compress/gzip"
	"crypto/x509"
	"encoding/base64"
	"os"
	"path/filepath"
)

// QrFormatVersion is the wire version of the compact QR envelope shared with
// the Android app. Version 2 dropped the CA certificate from device QRs.
const QrFormatVersion = 0x02

// QrBytes returns a compact QR payload body for importing a device identity
// into the ClipShare app: the PKCS#8 key and leaf certificate in DER, gzipped.
// The app decompresses this and rebuilds the PKCS#12 bundle locally.
//
// The CA certificate is deliberately NOT included: it is shared server
// infrastructure that only needs importing once (see QrCaContent), and
// dropping it keeps the QR small enough to scan reliably.
func QrBytes(dir, name, kind string) ([]byte, error) {
	d, err := loadDevice(dir, name, kind, false)
	if err != nil {
		return nil, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(d.key)
	if err != nil {
		return nil, err
	}

	// Envelope: version byte + two u16-length-prefixed DER segments
	// (key, leaf certificate).
	body := make([]byte, 0, 1+4+len(keyDER)+len(d.cert.Raw))
	body = append(body, QrFormatVersion)
	body = appendSeg(body, keyDER)
	body = appendSeg(body, d.cert.Raw)

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(body); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// QrCaContent returns the PEM of the private CA. The phone needs this once
// (per server) to trust the server's certificate; it is deliberately separate
// from the device QRs so those stay small.
func QrCaContent(dir string) ([]byte, error) {
	return os.ReadFile(filepath.Join(dir, CaCertFile))
}

// QrContent returns the QR payload for importing a device identity into the
// ClipShare app: the "clipshare-p12:" prefix plus unpadded base64 of the
// compact gzipped envelope from QrBytes.
func QrContent(dir, name, kind string) (string, error) {
	qr, err := QrBytes(dir, name, kind)
	if err != nil {
		return "", err
	}
	return "clipshare-p12:" + base64.RawStdEncoding.EncodeToString(qr), nil
}

// appendSeg appends data to dst with a big-endian u16 length prefix.
func appendSeg(dst, data []byte) []byte {
	dst = append(dst, byte(len(data)>>8), byte(len(data)))
	return append(dst, data...)
}
