package commands_test

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"strings"
	"testing"

	"clipshare/src/internal/certs"
	"clipshare/src/internal/cli/commands"
)

// TestQrContent locks the QR payload format shared with the Android app: a
// "clipshare-p12:" prefix followed by unpadded base64 of a gzipped envelope
// (version byte + u16-length-prefixed key/leaf DER), small enough to fit in a
// scannable QR code. The CA is deliberately not included (see --type ca).
func TestQrContent(t *testing.T) {
	dir := t.TempDir()
	if err := certs.Init(dir); err != nil {
		t.Fatalf("init CA: %v", err)
	}
	if err := certs.Issue(dir, "phone1", "client", nil); err != nil {
		t.Fatalf("issue client cert: %v", err)
	}

	content, err := commands.QrContent(dir, "phone1", "client")
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
	if len(content) > 700 {
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
	if len(body) == 0 || body[0] != certs.QrFormatVersion {
		t.Fatalf("envelope missing version byte %d", certs.QrFormatVersion)
	}

	// Two segments: key then leaf certificate. No CA (that is shared via
	// the separate "clipshare-ca:" QR).
	off := 1
	for i := 0; i < 2; i++ {
		if off+2 > len(body) {
			t.Fatalf("envelope truncated at segment %d", i)
		}
		segLen := int(body[off])<<8 | int(body[off+1])
		off += 2
		if off+segLen > len(body) {
			t.Fatalf("envelope segment %d overruns buffer", i)
		}
		off += segLen
	}
	if off != len(body) {
		t.Fatalf("envelope has %d trailing bytes", len(body)-off)
	}
}

// TestCaQrContent locks the "clipshare-ca:" QR payload: unpadded base64 of the
// PEM CA certificate, which the app imports once to trust the server.
func TestCaQrContent(t *testing.T) {
	dir := t.TempDir()
	if err := certs.Init(dir); err != nil {
		t.Fatalf("init CA: %v", err)
	}

	pem, err := certs.QrCaContent(dir)
	if err != nil {
		t.Fatalf("QrCaContent: %v", err)
	}
	content := "clipshare-ca:" + base64.RawStdEncoding.EncodeToString(pem)
	if !strings.HasPrefix(content, "clipshare-ca:") {
		t.Fatalf("content %q missing clipshare-ca: prefix", content)
	}
	raw := strings.TrimPrefix(content, "clipshare-ca:")
	decoded, err := base64.RawStdEncoding.DecodeString(raw)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if !strings.Contains(string(decoded), "BEGIN CERTIFICATE") {
		t.Fatalf("CA payload is not PEM: %q", decoded)
	}
}