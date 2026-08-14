package integration_test

import (
	"context"
	"testing"
	"time"

	"clipshare/src/tests/integration/client"
)

func TestDaemonNoTLS(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	net := startNetwork(ctx, t)
	cfg := copyFixture(t, "config-notls.toml")
	daemon := startDaemon(ctx, t, net, cfg, "")

	hostPort := daemonHostPort(ctx, t, daemon, "40403/tcp")

	android, err := client.Dial(ctx, hostPort, nil, "")
	if err != nil {
		t.Fatalf("dial daemon: %v", err)
	}
	defer android.Close()

	text := "hello from android " + time.Now().String()
	if err := android.SendText(ctx, text); err != nil {
		t.Fatalf("send text: %v", err)
	}

	// Give the daemon time to write to its clipboard. The X11 selection
	// transfer is asynchronous; reading too early can return INCR chunk data.
	time.Sleep(2 * time.Second)

	got := containerClipboard(ctx, t, daemon)
	if got != text {
		t.Errorf("clipboard: got %q, want %q", got, text)
	}
}
