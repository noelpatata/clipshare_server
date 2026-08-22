package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"clipshare/src/tests/integration/client"
)

func TestSendOneShot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	net := startNetwork(ctx, t)
	cfg := copyFixture(t, "config-notls.toml")
	text := "one-shot delivery " + time.Now().String()

	oneShot := startOneShot(ctx, t, net, cfg, text)
	hostPort := daemonHostPort(ctx, t, oneShot, "40403/tcp")

	android, err := client.Dial(ctx, hostPort, nil, "")
	if err != nil {
		t.Fatalf("dial one-shot daemon: %v", err)
	}
	defer android.Close()

	got, ok := android.WaitForText(15*time.Second, func(s string) bool {
		return strings.Contains(s, text)
	})
	if !ok {
		t.Fatalf("android did not receive one-shot text")
	}
	if got != text {
		t.Errorf("received text: got %q, want %q", got, text)
	}

	exitCode := waitForContainerExit(ctx, t, oneShot, 15*time.Second)
	if exitCode != 0 {
		t.Errorf("one-shot container exited with code %d", exitCode)
	}
}
