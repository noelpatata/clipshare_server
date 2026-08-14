package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"clipshare/src/tests/integration/client"
)

func TestSendCommand(t *testing.T) {
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

	text := "pushed from host " + time.Now().String()
	execSend(ctx, t, daemon, text)

	got, ok := android.WaitForText(5*time.Second, func(s string) bool {
		return strings.Contains(s, text)
	})
	if !ok {
		t.Fatalf("android did not receive sent text")
	}
	if got != text {
		t.Errorf("received text: got %q, want %q", got, text)
	}
}
