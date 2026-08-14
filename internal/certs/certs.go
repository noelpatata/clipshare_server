// Package certs manages the ClipShare private CA and per-device certificates
// used for mutual TLS.
package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

const (
	CaCertFile  = "ca.pem"
	CaKeyFile   = "ca.key"
	P12Password = "clipshare"
)

// File names for a named device. Server certs always use the fixed
// server.pem/server.key (matching the config defaults); client certs keep
// the device name so one certs dir can hold several.
func CertFile(name, kind string) string {
	if kind == "server" {
		return "server.pem"
	}
	return name + "-" + kind + ".pem"
}

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
// "server" or "client"; server certs also receive a ClientAuth EKU so a
// desktop cert can be used for outbound peer dials too. server certs embed
// the given IPs as SAN entries (defaults to the machine's non-loopback IPs).
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

// LoadPool builds a CertPool from a PEM CA file.
func LoadPool(path string) (*x509.CertPool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read CA %s: %w", path, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil, fmt.Errorf("no certificates found in %s", path)
	}
	return pool, nil
}

// LoadKeyPair loads a TLS certificate/key pair.
func LoadKeyPair(certPath, keyPath string) (tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("load keypair (%s, %s): %w", certPath, keyPath, err)
	}
	return cert, nil
}

// ExportP12 writes a PKCS#12 bundle (key + leaf + CA chain) that the Android
// app can import, e.g. for a client device.
func ExportP12(dir, name, kind, out string) error {
	certPEM, err := os.ReadFile(filepath.Join(dir, CertFile(name, kind)))
	if err != nil {
		return err
	}
	keyPEM, err := os.ReadFile(filepath.Join(dir, KeyFile(name, kind)))
	if err != nil {
		return err
	}
	caPEM, err := os.ReadFile(filepath.Join(dir, CaCertFile))
	if err != nil {
		return err
	}
	cert, err := parseCertPEM(certPEM)
	if err != nil {
		return err
	}
	key, err := parseKeyPEM(keyPEM)
	if err != nil {
		return err
	}
	caCert, err := parseCertPEM(caPEM)
	if err != nil {
		return err
	}
	// Legacy (3DES + SHA-1) is used deliberately: Android's bundled
	// BouncyCastle PKCS#12 parser does not handle Modern's PBMAC1/AES
	// bags on all API levels. Legacy is universally supported.
	der, err := pkcs12.Legacy.Encode(key, cert, []*x509.Certificate{caCert}, P12Password)
	if err != nil {
		return fmt.Errorf("pkcs12 encode: %w", err)
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(der)
	return err
}

// List returns a human-readable summary of the certificates in dir.
func List(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no certs directory at %s (run 'clipshare cert init')", dir)
		}
		return "", err
	}
	var b strings.Builder
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".pem") {
			continue
		}
		cert, err := parseCertFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "%-32s CN=%-20s not-after=%s\n",
			e.Name(), cert.Subject.CommonName, cert.NotAfter.Format(time.RFC3339))
	}
	return b.String(), nil
}

// LocalIPs returns the machine's non-loopback IPv4 addresses.
func LocalIPs() []string {
	var out []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return out
	}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() || ipnet.IP.To4() == nil {
			continue
		}
		out = append(out, ipnet.IP.String())
	}
	sort.Strings(out)
	return out
}

func randSerial() *big.Int {
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return big.NewInt(time.Now().UnixNano())
	}
	return n
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
