package certs

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

// device holds a parsed device bundle: key, leaf and optional CA certificate.
type device struct {
	key  *ecdsa.PrivateKey
	cert *x509.Certificate
	ca   *x509.Certificate
}

// loadDevice reads and parses a device's key, leaf certificate and (optionally)
// the CA certificate from dir.
func loadDevice(dir, name, kind string, includeCA bool) (*device, error) {
	certPEM, err := os.ReadFile(filepath.Join(dir, CertFile(name, kind)))
	if err != nil {
		return nil, err
	}
	keyPEM, err := os.ReadFile(filepath.Join(dir, KeyFile(name, kind)))
	if err != nil {
		return nil, err
	}
	d := &device{}
	if d.cert, err = parseCertPEM(certPEM); err != nil {
		return nil, err
	}
	if d.key, err = parseKeyPEM(keyPEM); err != nil {
		return nil, err
	}
	if includeCA {
		caPEM, err := os.ReadFile(filepath.Join(dir, CaCertFile))
		if err != nil {
			return nil, err
		}
		if d.ca, err = parseCertPEM(caPEM); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func writePEM(path, blockType string, der []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer f.Close()
	return pem.Encode(f, &pem.Block{Type: blockType, Bytes: der})
}

func parseCertFile(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseCertPEM(data)
}

func parseCertPEM(data []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("no CERTIFICATE block found")
	}
	return x509.ParseCertificate(block.Bytes)
}

func parseKeyFile(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseKeyPEM(data)
}

func parseKeyPEM(data []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no key block found")
	}
	switch block.Type {
	case "PRIVATE KEY":
		k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		ec, ok := k.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("key is not ECDSA")
		}
		return ec, nil
	case "EC PRIVATE KEY":
		return x509.ParseECPrivateKey(block.Bytes)
	default:
		return nil, fmt.Errorf("unsupported key block type %q", block.Type)
	}
}
