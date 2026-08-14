package websocket

import (
	"crypto/tls"
	"net"

	"clipshare/internal/certs"
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
func (s *Server) peerTLSConfig() (*tls.Config, error) {
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
		RootCAs:      pool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func tlsListener(ln net.Listener, cfg *tls.Config) net.Listener {
	return tls.NewListener(ln, cfg)
}
