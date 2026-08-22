package certs

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"sort"
)

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
