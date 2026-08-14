package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"clipshare/src/tests/integration/client"
)

// TestTwoAndroidClients verifies that two Android simulator clients connected to
// the same daemon can exchange text. The daemon relays clipboard messages
// between clients; there is no desktop clipboard involved.
func TestTwoAndroidClients(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	net := startNetwork(ctx, t)
	cfg := copyFixture(t, "config-notls.toml")
	daemon := startDaemon(ctx, t, net, cfg, "")

	hostPort := daemonHostPort(ctx, t, daemon, "40403/tcp")

	androidA, err := client.Dial(ctx, hostPort, nil, "")
	if err != nil {
		t.Fatalf("dial android A: %v", err)
	}
	defer androidA.Close()

	androidB, err := client.Dial(ctx, hostPort, nil, "")
	if err != nil {
		t.Fatalf("dial android B: %v", err)
	}
	defer androidB.Close()

	textA := "from android A " + time.Now().String()
	if err := androidA.SendText(ctx, textA); err != nil {
		t.Fatalf("android A send: %v", err)
	}
	gotB, ok := androidB.WaitForText(5*time.Second, func(s string) bool {
		return strings.Contains(s, textA)
	})
	if !ok {
		t.Fatalf("android B did not receive text from android A")
	}
	if gotB != textA {
		t.Errorf("android B text: got %q, want %q", gotB, textA)
	}

	textB := "from android B " + time.Now().String()
	if err := androidB.SendText(ctx, textB); err != nil {
		t.Fatalf("android B send: %v", err)
	}
	gotA, ok := androidA.WaitForText(5*time.Second, func(s string) bool {
		return strings.Contains(s, textB)
	})
	if !ok {
		t.Fatalf("android A did not receive text from android B")
	}
	if gotA != textB {
		t.Errorf("android A text: got %q, want %q", gotA, textB)
	}
}
