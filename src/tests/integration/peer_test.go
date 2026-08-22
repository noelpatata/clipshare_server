package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"clipshare/src/tests/integration/client"
)

// TestPeerRelay verifies that two daemon containers can connect as peers and
// relay clipboard content between them. An Android client sends text to peer-a;
// peer-a forwards it to its configured peer (peer-b); peer-b writes the text to
// its own clipboard.
func TestPeerRelay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	net := startNetwork(ctx, t)
	cfgA := copyFixture(t, "config-peer-a.toml")
	cfgB := copyFixture(t, "config-peer-b.toml")

	// Start peer-b first so peer-a can connect to it.
	peerB := startDaemonWithAlias(ctx, t, net, cfgB, "", "peer-b")
	peerA := startDaemonWithAlias(ctx, t, net, cfgA, "", "peer-a")

	// Wait for peer-a to connect to peer-b.
	if err := waitForLog(ctx, peerA, "peer peer-b:40403: connected", 15*time.Second); err != nil {
		t.Fatalf("peer-a did not connect to peer-b: %v", err)
	}

	// Connect an Android client to peer-a.
	hostPort := daemonHostPort(ctx, t, peerA, "40403/tcp")
	android, err := client.Dial(ctx, hostPort, nil, "")
	if err != nil {
		t.Fatalf("dial peer-a: %v", err)
	}
	defer android.Close()

	text := "relayed to peer " + time.Now().String()
	if err := android.SendText(ctx, text); err != nil {
		t.Fatalf("send text: %v", err)
	}

	// Give peer-a time to forward the message to peer-b and for peer-b to
	// write it to its clipboard.
	time.Sleep(1 * time.Second)

	got := containerClipboard(ctx, t, peerB)
	if !strings.Contains(got, text) {
		t.Errorf("peer-b clipboard: got %q, want %q", got, text)
	}
}
