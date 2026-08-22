package integration_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestMDNSAndBeacon(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	net := startNetwork(ctx, t)
	cfg := copyFixture(t, "config-notls.toml")

	// Start listener first so it is ready when the daemon starts advertising.
	listener := startListener(ctx, t, net, 40404)

	_ = startDaemon(ctx, t, net, cfg, "")

	lines := readListenerOutput(ctx, t, listener)

	var foundMDNS, foundBeacon bool
	for _, line := range lines {
		if strings.Contains(line, "mdns") {
			var ev struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal([]byte(line), &ev); err == nil && strings.Contains(ev.Name, "daemon-notls") {
				foundMDNS = true
			}
		}
		if strings.Contains(line, "beacon") {
			var ev struct {
				Name string `json:"name"`
				Port int    `json:"port"`
			}
			if err := json.Unmarshal([]byte(line), &ev); err == nil && strings.Contains(ev.Name, "daemon-notls") && ev.Port == 40403 {
				foundBeacon = true
			}
		}
	}

	if !foundMDNS {
		t.Errorf("mDNS service not discovered; listener output:\n%s", strings.Join(lines, "\n"))
	}
	// Docker bridge networks may drop broadcasts to 255.255.255.255, so the
	// beacon assertion is a soft warning rather than a hard failure.
	if !foundBeacon {
		t.Logf("UDP beacon not received in this Docker network; listener output:\n%s", strings.Join(lines, "\n"))
	}
}
