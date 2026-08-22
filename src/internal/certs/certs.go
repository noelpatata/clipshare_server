// Package certs manages the ClipShare private CA and per-device certificates
// used for mutual TLS.
package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

const (
	CaCertFile = "ca.pem"
	CaKeyFile  = "ca.key"
)

// CertFile returns the certificate file name for a device. Server certs always
// use server.pem (matching the config defaults); client certs keep the device
// name so one certs dir can hold several.
func CertFile(name, kind string) string {
	if kind == "server" {
		return "server.pem"
	}
	return name + "-" + kind + ".pem"
}

// KeyFile returns the private key file name for a device (see CertFile).
func KeyFile(name, kind string) string {
	if kind == "server" {
		return "server.key"
	}
	return name + "-" + kind + ".key"
}

// NewKey generates an ECDSA P-256 private key.
func NewKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

// Init creates a new private CA (ca.pem / ca.key) in dir. It refuses to
// overwrite an existing CA.
func Init(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	caPath := filepath.Join(dir, CaCertFile)
	if _, err := os.Stat(caPath); err == nil {
		return fmt.Errorf("CA already exists at %s (delete it to regenerate)", caPath)
	}
	key, err := NewKey()
	if err != nil {
		return err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "clipshare-ca"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(10, 0, 0),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return err
	}
	if err := writePEM(filepath.Join(dir, CaCertFile), "CERTIFICATE", der, 0o644); err != nil {
		return err
	}
	keyDer, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	return writePEM(filepath.Join(dir, CaKeyFile), "PRIVATE KEY", keyDer, 0o600)
}

// Issue signs a leaf certificate for name (CN) with the CA in dir. kind is
// "server" or "client"; server certs embed the given IPs as SAN entries
// (defaults to the machine's non-loopback IPs).
func Issue(dir, name, kind string, ips []string) error {
	if kind != "server" && kind != "client" {
		return fmt.Errorf("kind must be 'server' or 'client'")
	}
	caCert, caKey, err := LoadCA(dir)
	if err != nil {
		return err
	}
	key, err := NewKey()
	if err != nil {
		return err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: randSerial(),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	if kind == "server" {
		tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
		if len(ips) == 0 {
			ips = LocalIPs()
		}
		for _, ip := range ips {
			if parsed := net.ParseIP(ip); parsed != nil {
				tmpl.IPAddresses = append(tmpl.IPAddresses, parsed)
			} else {
				tmpl.DNSNames = append(tmpl.DNSNames, ip)
			}
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		return err
	}
	if err := writePEM(filepath.Join(dir, CertFile(name, kind)), "CERTIFICATE", der, 0o644); err != nil {
		return err
	}
	keyDer, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	return writePEM(filepath.Join(dir, KeyFile(name, kind)), "PRIVATE KEY", keyDer, 0o600)
}

// LoadCA loads the CA certificate and key from dir.
func LoadCA(dir string) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	caCert, err := parseCertFile(filepath.Join(dir, CaCertFile))
	if err != nil {
		return nil, nil, fmt.Errorf("load %s: %w", CaCertFile, err)
	}
	key, err := parseKeyFile(filepath.Join(dir, CaKeyFile))
	if err != nil {
		return nil, nil, fmt.Errorf("load %s: %w", CaKeyFile, err)
	}
	return caCert, key, nil
}

func randSerial() *big.Int {
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return big.NewInt(time.Now().UnixNano())
	}
	return n
}
