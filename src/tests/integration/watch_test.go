package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"clipshare/src/tests/integration/client"
)

func TestWatch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	net := startNetwork(ctx, t)
	cfg := copyFixture(t, "config-notls.toml")
	text := "pushed to watch " + time.Now().String()

	watch := startWatch(ctx, t, net, cfg, 10*time.Second)
	hostPort := daemonHostPort(ctx, t, watch, "40403/tcp")

	android, err := client.Dial(ctx, hostPort, nil, "")
	if err != nil {
		t.Fatalf("dial watch daemon: %v", err)
	}
	defer android.Close()

	if err := android.SendText(ctx, text); err != nil {
		t.Fatalf("send text: %v", err)
	}

	// Wait for the watch container to exit (it writes the clipboard to a file
	// before exiting).
	exitCode := waitForContainerExit(ctx, t, watch, 15*time.Second)
	if exitCode != 0 {
		t.Errorf("watch container exited with code %d", exitCode)
	}

	got := watchClipboard(ctx, t, watch)

	if !strings.Contains(got, text) {
		t.Errorf("watch clipboard: got %q, want %q", got, text)
	}
}
