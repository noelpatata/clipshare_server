package discover

import (
	"context"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/grandcat/zeroconf"
)

const ServiceType = "_clipshare._tcp"

// Advertise announces the daemon over mDNS and periodic UDP broadcasts.
// domain "" means "local". tls reports whether clients must use wss://.
func Advertise(ctx context.Context, name string, port int, mdns bool, tls bool) error {
	if !mdns {
		log.Printf("mdns advertising disabled")
	} else {
		server, err := zeroconf.Register(name, ServiceType, "local.", port,
			[]string{"name=" + name, "tls=" + strconv.FormatBool(tls)}, nil)
		if err != nil {
			return err
		}
		go func() {
			<-ctx.Done()
			server.Shutdown()
		}()
		log.Printf("mdns: advertising %s.%s on port %d (tls=%v)", name, ServiceType, port, tls)
	}
	return nil
}

// Beacon sends periodic UDP JSON beacons so clients on the same subnet can
// discover the daemon without mDNS. Sends immediately, then every interval.
// The shared token is deliberately NOT included (plaintext leak on the LAN);
// authentication is handled over the TLS handshake or token query.
func Beacon(ctx context.Context, name string, port int, interval time.Duration, tls bool) error {
	conn, err := net.DialUDP("udp4", nil,
		&net.UDPAddr{IP: net.IPv4bcast, Port: 40404})
	if err != nil {
		return err
	}
	defer conn.Close()
	payload := beaconJSON(name, port, tls)
	t := time.NewTicker(interval)
	defer t.Stop()
	send := func() {
		if _, err := conn.Write(payload); err != nil {
			log.Printf("beacon send: %v", err)
		}
	}
	send()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			send()
		}
	}
}
