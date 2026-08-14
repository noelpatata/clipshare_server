package integration_test

import (
	"context"
	"testing"
	"time"

	"clipshare/src/tests/integration/client"
)

func TestDaemonTLS(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	certDir := generateCerts(t)
	net := startNetwork(ctx, t)
	cfg := copyFixture(t, "config-tls.toml")
	daemon := startDaemon(ctx, t, net, cfg, certDir)

	hostPort := daemonHostPort(ctx, t, daemon, "40403/tcp")

	// Android client with valid client certificate should connect.
	android, err := client.Dial(ctx, hostPort, clientTLSConfig(t, certDir), "")
	if err != nil {
		t.Fatalf("dial daemon with client cert: %v", err)
	}
	defer android.Close()

	text := "secure hello " + time.Now().String()
	if err := android.SendText(ctx, text); err != nil {
		t.Fatalf("send text: %v", err)
	}

	time.Sleep(500 * time.Millisecond)
	got := containerClipboard(ctx, t, daemon)
	if got != text {
		t.Errorf("clipboard: got %q, want %q", got, text)
	}
}

func TestDaemonTLSRejectsClientWithoutCert(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	certDir := generateCerts(t)
	net := startNetwork(ctx, t)
	cfg := copyFixture(t, "config-tls.toml")
	daemon := startDaemon(ctx, t, net, cfg, certDir)

	hostPort := daemonHostPort(ctx, t, daemon, "40403/tcp")

	// Android client without a client certificate should fail the TLS handshake.
	_, err := client.Dial(ctx, hostPort, serverTLSConfig(t, certDir), "")
	if err == nil {
		t.Fatal("expected TLS handshake to fail without client cert")
	}
}
