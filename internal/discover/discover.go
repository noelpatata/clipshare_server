package discover

import (
	"context"
	"fmt"
	"log"

	"clipshare/internal/config"
)

// Start begins LAN discovery (mDNS + UDP beacon) when the connection mode is
// "discover". It returns immediately and runs the enabled methods in the
// background until ctx is cancelled. An error is returned only when a enabled
// method fails to start; failures after startup are logged.
func Start(ctx context.Context, cfg *config.Config) error {
	if cfg.Connection.Mode != config.ModeDiscover {
		log.Printf("connection mode %q: not advertising (whitelist-only)", cfg.Connection.Mode)
		return nil
	}

	if err := advertiseIfEnabled(ctx, cfg); err != nil {
		return fmt.Errorf("mdns advertise: %w", err)
	}

	go func() {
		if err := beaconIfEnabled(ctx, cfg); err != nil {
			log.Printf("udp beacon failed: %v", err)
		}
	}()

	return nil
}
