package certs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
