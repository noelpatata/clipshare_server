package integration_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"

	"clipshare/src/tests/integration/client"
)

// containerLogs returns the container's combined log output.
func containerLogs(ctx context.Context, t *testing.T, c testcontainers.Container) string {
	t.Helper()
	logs, err := c.Logs(ctx)
	if err != nil {
		t.Fatalf("container logs: %v", err)
	}
	defer logs.Close()
	data, err := io.ReadAll(logs)
	if err != nil {
		t.Fatalf("read logs: %v", err)
	}
	return string(data)
}

// TestPeerTLSDialWithMismatchedSAN verifies the network-independence fix:
// the server certificate carries SANs (127.0.0.1/localhost) that do not match
// the address a peer dials (the "peer-b" container alias). Trust is anchored
// on the private CA alone, so the peer must still connect and relay content.
// Before the fix this dial failed with "certificate is valid for ... not for
// peer-b".
func TestPeerTLSDialWithMismatchedSAN(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	certDir := generateCerts(t)
	net := startNetwork(ctx, t)
	cfgA := copyFixture(t, "config-tls-peer-a.toml")
	cfgB := copyFixture(t, "config-tls-peer-b.toml")

	peerB := startDaemonWithAlias(ctx, t, net, cfgB, certDir, "peer-b")
	peerA := startDaemonWithAlias(ctx, t, net, cfgA, certDir, "peer-a")

	// peer-a dials "peer-b:40403"; the cert SANs do not include it. With
	// hostname verification skipped (CA trust only) this must succeed.
	if err := waitForLog(ctx, peerA, "peer peer-b:40403: connected", 20*time.Second); err != nil {
		t.Fatalf("peer-a did not connect to peer-b over TLS: %v", err)
	}

	// End-to-end: an Android client (with a CA-signed cert) pushes text to
	// peer-a, which relays it to peer-b over the mTLS peer connection.
	hostPort := daemonHostPort(ctx, t, peerA, "40403/tcp")
	android, err := client.Dial(ctx, hostPort, clientTLSConfig(t, certDir), "")
	if err != nil {
		t.Fatalf("dial peer-a: %v", err)
	}
	defer android.Close()

	text := "tls relay " + time.Now().String()
	if err := android.SendText(ctx, text); err != nil {
		t.Fatalf("send text: %v", err)
	}

	time.Sleep(1 * time.Second)
	got := containerClipboard(ctx, t, peerB)
	if !strings.Contains(got, text) {
		t.Errorf("peer-b clipboard: got %q, want %q", got, text)
	}
}

// TestPeerTLSDialWithMismatchedIPSAN reproduces the exact wifi-change scenario:
// the server certificate was issued with an IP SAN for one network
// (10.99.99.99) but the peer connects to a different address (the "peer-b"
// container). The dialed address is not in the cert, so without the fix Go
// rejects it with "certificate is valid for 10.99.99.99, not peer-b".
func TestPeerTLSDialWithMismatchedIPSAN(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	certDir := generateCertsWithSANs(t, []string{"10.99.99.99"})
	net := startNetwork(ctx, t)
	cfgA := copyFixture(t, "config-tls-peer-a.toml")
	cfgB := copyFixture(t, "config-tls-peer-b.toml")

	peerB := startDaemonWithAlias(ctx, t, net, cfgB, certDir, "peer-b")
	_ = peerB
	peerA := startDaemonWithAlias(ctx, t, net, cfgA, certDir, "peer-a")

	if err := waitForLog(ctx, peerA, "peer peer-b:40403: connected", 20*time.Second); err != nil {
		t.Fatalf("peer-a did not connect to peer-b over TLS despite IP mismatch: %v", err)
	}
}

// TestPeerTLSRejectsForeignCA ensures hostname verification being disabled
// does not weaken chain trust: a peer signed by a different CA must fail the
// TLS handshake and never report a successful connection.
func TestPeerTLSRejectsForeignCA(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Two independent CAs: peer-a trusts CA A, peer-b presents a cert from CA B.
	certDirA := generateCerts(t)
	certDirB := generateCerts(t)
	net := startNetwork(ctx, t)
	cfgA := copyFixture(t, "config-tls-peer-a.toml")
	cfgB := copyFixture(t, "config-tls-peer-b.toml")

	peerB := startDaemonWithAlias(ctx, t, net, cfgB, certDirB, "peer-b")
	_ = peerB
	peerA := startDaemonWithAlias(ctx, t, net, cfgA, certDirA, "peer-a")

	// The dial failure is logged at debug level.
	if err := waitForLog(ctx, peerA, "peer peer-b:40403: dial failed", 20*time.Second); err != nil {
		t.Fatalf("expected peer-a to log a TLS dial failure: %v", err)
	}
	if logs := containerLogs(ctx, t, peerA); strings.Contains(logs, "peer peer-b:40403: connected") {
		t.Fatal("peer-a connected to a device signed by a different CA")
	}
}

// TestPeerTLSStrictRejectsMismatchedHostname verifies the configurable
// hostname verification: with verify_hostname at its default (true), a peer
// whose certificate does not cover the dialed address must be rejected, even
// though it is signed by the trusted CA.
func TestPeerTLSStrictRejectsMismatchedHostname(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// The server cert only covers 10.99.99.99; peer-a dials the "peer-b" alias.
	certDir := generateCertsWithSANs(t, []string{"10.99.99.99"})
	net := startNetwork(ctx, t)
	cfgStrictA := copyFixture(t, "config-tls-strict-peer-a.toml")
	cfgB := copyFixture(t, "config-tls-peer-b.toml")

	peerB := startDaemonWithAlias(ctx, t, net, cfgB, certDir, "peer-b")
	_ = peerB
	peerA := startDaemonWithAlias(ctx, t, net, cfgStrictA, certDir, "peer-a")

	// peer-b is listening before peer-a starts, so the dial failure is a
	// certificate/hostname rejection, not a connection refusal.
	if err := waitForLog(ctx, peerA, "peer peer-b:40403: dial failed", 20*time.Second); err != nil {
		t.Fatalf("expected strict hostname verification to reject the mismatched address: %v", err)
	}
	if logs := containerLogs(ctx, t, peerA); strings.Contains(logs, "peer peer-b:40403: connected") {
		t.Fatal("peer-a connected despite strict hostname verification")
	}
}