package discover

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/grandcat/zeroconf"
)

const ServiceType = "_clipshare._tcp"

// Advertise announces the daemon over mDNS and periodic UDP broadcasts.
// domain "" means "local".
func Advertise(ctx context.Context, name string, port int, mdns bool) error {
	if !mdns {
		log.Printf("mdns advertising disabled")
	} else {
		server, err := zeroconf.Register(name, ServiceType, "local.", port,
			[]string{"name=" + name}, nil)
		if err != nil {
			return err
		}
		go func() {
			<-ctx.Done()
			server.Shutdown()
		}()
		log.Printf("mdns: advertising %s.%s on port %d", name, ServiceType, port)
	}
	return nil
}

// Beacon sends periodic UDP JSON beacons so clients on the same subnet can
// discover the daemon without mDNS. Sends immediately, then every interval.
func Beacon(ctx context.Context, name string, port int, interval time.Duration, token string) error {
	conn, err := net.DialUDP("udp4", nil,
		&net.UDPAddr{IP: net.IPv4bcast, Port: 40404})
	if err != nil {
		return err
	}
	defer conn.Close()
	payload := beaconJSON(name, port, token)
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
