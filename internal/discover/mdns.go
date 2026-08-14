package discover

import (
	"context"
	"log"
	"strconv"

	"github.com/grandcat/zeroconf"

	"clipshare/internal/config"
	"clipshare/internal/consts"
)

// Advertise announces the daemon over mDNS. domain "" means "local".
// tls reports whether clients must use wss://.
func Advertise(ctx context.Context, name string, port int, tls bool) error {
	server, err := zeroconf.Register(name, consts.ServiceType, "local.", port,
		[]string{"name=" + name, "tls=" + strconv.FormatBool(tls)}, nil)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		server.Shutdown()
	}()
	log.Printf("mdns: advertising %s.%s on port %d (tls=%v)", name, consts.ServiceType, port, tls)
	return nil
}

// advertiseIfEnabled starts mDNS advertising when configured to do so.
func advertiseIfEnabled(ctx context.Context, cfg *config.Config) error {
	if !cfg.Discovery.MDNS {
		log.Printf("mdns advertising disabled")
		return nil
	}
	return Advertise(ctx, cfg.DeviceName, cfg.Server.Port, cfg.TLS.Enabled)
}
