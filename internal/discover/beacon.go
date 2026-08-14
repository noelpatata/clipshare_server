package discover

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"time"

	"clipshare/internal/config"
)

// BeaconMsg is the JSON broadcast sent over UDP to announce the daemon.
type BeaconMsg struct {
	Name string `json:"name"`
	Port int    `json:"port"`
	TLS  bool   `json:"tls,omitempty"`
	TS   int64  `json:"ts"`
}

// Beacon sends periodic UDP JSON beacons so clients on the same subnet can
// discover the daemon without mDNS. Sends immediately, then every interval.
// The shared token is deliberately NOT included (plaintext leak on the LAN);
// authentication is handled over the TLS handshake or token query.
type Beacon struct {
	Name     string
	Port     int
	Addr     string
	Interval time.Duration
	TLS      bool
}

// Run starts the beacon loop and blocks until ctx is cancelled.
func (b *Beacon) Run(ctx context.Context) error {
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.ParseIP(b.Addr), Port: b.Port})
	if err != nil {
		return err
	}
	defer conn.Close()

	payload := b.payload()
	t := time.NewTicker(b.Interval)
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
			payload = b.payload()
			send()
		}
	}
}

func (b *Beacon) payload() []byte {
	msg := BeaconMsg{Name: b.Name, Port: b.Port, TLS: b.TLS, TS: time.Now().Unix()}
	data, err := json.Marshal(msg)
	if err != nil {
		return []byte("{}")
	}
	return data
}

// beaconIfEnabled starts UDP beacon broadcasting when configured to do so.
func beaconIfEnabled(ctx context.Context, cfg *config.Config) error {
	if !cfg.Discovery.Beacon {
		log.Printf("udp beacon disabled")
		return nil
	}
	b := &Beacon{
		Name:     cfg.DeviceName,
		Port:     cfg.Server.Port,
		Addr:     cfg.Discovery.BeaconAddr,
		Interval: cfg.BroadcastInterval(),
		TLS:      cfg.TLS.Enabled,
	}
	return b.Run(ctx)
}
