package websocket

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"

	"clipshare/src/internal/certs"
)

// tlsConfig builds the server-side TLS config: this device's cert plus a
// private CA pool and mandatory client certificates (mutual TLS).
func (s *Server) tlsConfig() (*tls.Config, error) {
	cert, err := certs.LoadKeyPair(s.cfg.TLS.Cert, s.cfg.TLS.Key)
	if err != nil {
		return nil, err
	}
	pool, err := certs.LoadPool(s.cfg.TLS.CA)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// peerTLSConfig builds a client-side TLS config for outbound peer dials:
// this device's cert is presented to the remote daemon and the CA pool
// verifies the remote daemon's cert.
//
// Hostname/IP-address verification is controlled by tls.verify_hostname
// (default true, strict). When disabled, the certificate's SAN IPs are ignored
// and trust is anchored solely on the private CA, so a device stays
// connectable across wifi/DHCP changes without re-issuing certificates. In
// that mode chain validation is re-enabled manually via VerifyConnection so
// InsecureSkipVerify only disables the SAN/hostname check, not the signature
// chain.
func (s *Server) peerTLSConfig() (*tls.Config, error) {
	cert, err := certs.LoadKeyPair(s.cfg.TLS.Cert, s.cfg.TLS.Key)
	if err != nil {
		return nil, err
	}
	pool, err := certs.LoadPool(s.cfg.TLS.CA)
	if err != nil {
		return nil, err
	}
	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		MinVersion:   tls.VersionTLS12,
	}
	if !s.cfg.TLS.VerifyHostname {
		cfg.InsecureSkipVerify = true
		cfg.VerifyConnection = verifyChain(pool)
	}
	return cfg, nil
}

// verifyChain returns a VerifyConnection callback that validates the server's
// certificate chain against the given CA pool while ignoring the dialed
// hostname/IP. It is the counterpart to InsecureSkipVerify: chain trust is
// kept, address binding is dropped.
func verifyChain(pool *x509.CertPool) func(tls.ConnectionState) error {
	return func(cs tls.ConnectionState) error {
		peers := cs.PeerCertificates
		if len(peers) == 0 {
			return fmt.Errorf("no peer certificate presented")
		}
		opts := x509.VerifyOptions{
			Roots:         pool,
			Intermediates: x509.NewCertPool(),
			KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		}
		for _, c := range peers[1:] {
			opts.Intermediates.AddCert(c)
		}
		_, err := peers[0].Verify(opts)
		return err
	}
}

func tlsListener(ln net.Listener, cfg *tls.Config) net.Listener {
	return tls.NewListener(ln, cfg)
}
