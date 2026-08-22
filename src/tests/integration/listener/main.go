package main

import (
	"context"
	"encoding/json"
	"flag"
	"net"
	"os"
	"time"

	"github.com/grandcat/zeroconf"
)

type event struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
	Port int    `json:"port,omitempty"`
	TLS  bool   `json:"tls,omitempty"`
	Text string `json:"text,omitempty"`
}

func main() {
	service := flag.String("service", "_clipshare._tcp", "mDNS service type")
	beaconPort := flag.Int("beacon-port", 40404, "UDP beacon port")
	timeout := flag.Duration("timeout", 10*time.Second, "how long to listen")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	enc := json.NewEncoder(os.Stdout)
	mdnsDone := make(chan struct{})
	beaconDone := make(chan struct{})

	go listenMDNS(ctx, enc, *service, mdnsDone)
	go listenBeacon(ctx, enc, *beaconPort, beaconDone)

	<-ctx.Done()
	<-mdnsDone
	<-beaconDone
}

func listenMDNS(ctx context.Context, enc *json.Encoder, service string, done chan<- struct{}) {
	defer close(done)

	entries := make(chan *zeroconf.ServiceEntry)
	go func() {
		for e := range entries {
			_ = enc.Encode(event{
				Type: "mdns",
				Name: e.Instance,
				Port: e.Port,
				Text: e.HostName,
			})
		}
	}()

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		_ = enc.Encode(event{Type: "error", Text: "mdns resolver: " + err.Error()})
		return
	}
	if err := resolver.Browse(ctx, service, "local.", entries); err != nil {
		_ = enc.Encode(event{Type: "error", Text: "mdns browse: " + err.Error()})
	}
	<-ctx.Done()
}

func listenBeacon(ctx context.Context, enc *json.Encoder, port int, done chan<- struct{}) {
	defer close(done)

	addr := &net.UDPAddr{Port: port}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		_ = enc.Encode(event{Type: "error", Text: "beacon listen: " + err.Error()})
		return
	}
	defer conn.Close()

	buf := make([]byte, 1500)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return
		}
		var msg struct {
			Name string `json:"name"`
			Port int    `json:"port"`
			TLS  bool   `json:"tls"`
		}
		if err := json.Unmarshal(buf[:n], &msg); err != nil {
			continue
		}
		_ = enc.Encode(event{
			Type: "beacon",
			Name: msg.Name,
			Port: msg.Port,
			TLS:  msg.TLS,
		})
	}
}
