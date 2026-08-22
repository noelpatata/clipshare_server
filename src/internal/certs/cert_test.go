package certs_test

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"strings"
	"testing"

	"clipshare/src/internal/certs"
)

// TestQrContent locks the QR payload format shared with the Android app: a
// "clipshare-p12:" prefix followed by unpadded base64 of a gzipped envelope of
// three u16-length-prefixed DER segments (key, leaf, CA), small enough to fit
// in a scannable QR code. The CA is bundled so a single scan installs the
// private key for mutual TLS and trusts the server.
func TestQrContent(t *testing.T) {
	dir := t.TempDir()
	if err := certs.Init(dir); err != nil {
		t.Fatalf("init CA: %v", err)
	}
	if err := certs.Issue(dir, "phone1", "client", nil); err != nil {
		t.Fatalf("issue client cert: %v", err)
	}

	content, err := certs.QrContent(dir, "phone1", "client")
	if err != nil {
		t.Fatalf("QrContent: %v", err)
	}

	if !strings.HasPrefix(content, "clipshare-p12:") {
		t.Fatalf("content %q missing clipshare-p12: prefix", content)
	}

	raw := strings.TrimPrefix(content, "clipshare-p12:")
	if strings.Contains(raw, "=") {
		t.Error("QR payload must use unpadded base64")
	}
	payload, err := base64.RawStdEncoding.DecodeString(raw)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if len(payload) == 0 {
		t.Fatal("decoded payload is empty")
	}
	if len(content) > 900 {
		t.Errorf("QR payload is %d chars; too large to scan reliably", len(content))
	}

	// The payload must be the gzip-compressed envelope the Android app parses.
	gz, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("payload is not gzip: %v", err)
	}
	body, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("gunzip: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("envelope is empty")
	}

	// Three segments: key, leaf certificate, then CA certificate (auto-trusted
	// by the app on import).
	off := 0
	for i := 0; i < 3; i++ {
		if off+2 > len(body) {
			t.Fatalf("envelope truncated at segment %d", i)
		}
		segLen := int(body[off])<<8 | int(body[off+1])
		off += 2
		if off+segLen > len(body) {
			t.Fatalf("envelope segment %d overruns buffer", i)
		}
		if segLen == 0 {
			t.Fatalf("envelope segment %d is empty", i)
		}
		off += segLen
	}
	if off != len(body) {
		t.Fatalf("envelope has %d trailing bytes", len(body)-off)
	}
}
