package certs

import (
	"bytes"
	"compress/gzip"
	"crypto/x509"
	"encoding/base64"
)

// QrBytes returns a compact QR payload body for importing a device identity
// into the ClipShare app: the PKCS#8 key, leaf certificate and CA certificate
// in DER, gzipped. The app decompresses this, rebuilds the PKCS#12 bundle
// locally and auto-trusts the CA, so a single scan installs the private key
// for mutual TLS and trusts the server.
func QrBytes(dir, name, kind string) ([]byte, error) {
	d, err := loadDevice(dir, name, kind, true)
	if err != nil {
		return nil, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(d.key)
	if err != nil {
		return nil, err
	}

	// Envelope: three u16-length-prefixed DER segments (key, leaf, CA).
	body := make([]byte, 0, 6+len(keyDER)+len(d.cert.Raw)+len(d.ca.Raw))
	body = appendSeg(body, keyDER)
	body = appendSeg(body, d.cert.Raw)
	body = appendSeg(body, d.ca.Raw)

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
