package server

import (
	"context"
	"encoding/json"
	"log"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// PeerClient connects out to a remote clipshare daemon (desktop-to-desktop).
type PeerClient struct {
	srv      *Server
	conn     *websocket.Conn
	connMu   sync.Mutex
	onRemote func(text, from string)
}

// RunPeers dials each configured peer and reconnects with backoff until ctx
// is done. onRemote receives clipboard content pushed by the remote daemon.
func (s *Server) RunPeers(ctx context.Context, version string, onRemote func(text, from string)) {
	for _, host := range s.cfg.Peers {
		host := strings.TrimSpace(host)
		if host == "" {
			continue
		}
		pc := &PeerClient{srv: s, onRemote: onRemote}
		go pc.run(ctx, host, version)
	}
}

func (pc *PeerClient) run(ctx context.Context, host, version string) {
	addr := host
	if !strings.Contains(addr, ":") {
		addr += ":" + strconv.Itoa(pc.srv.cfg.Server.Port)
	}
	u := url.URL{Scheme: "ws", Host: addr, Path: "/ws"}
	if pc.srv.cfg.Token != "" {
		q := u.Query()
		q.Set("token", pc.srv.cfg.Token)
		u.RawQuery = q.Encode()
	}
	backoff := time.Second
	for {
		conn, _, err := websocket.Dial(ctx, u.String(), nil)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("peer %s: dial failed: %v (retry in %s)", host, err, backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
		pc.connMu.Lock()
		pc.conn = conn
		pc.connMu.Unlock()
		pc.srv.registerPeer(pc)
		log.Printf("peer %s: connected", host)

		hello, _ := json.Marshal(Envelope{Type: MsgHello, Data: mustJSON(Hello{
			Name: pc.srv.cfg.DeviceName, Platform: "desktop", Version: version,
		})})
		if err := conn.Write(ctx, websocket.MessageText, hello); err != nil {
			pc.close()
			pc.srv.unregisterPeer(pc)
			continue
		}

		go pc.keepalive(ctx, conn)
		pc.readLoop(ctx, conn, host)
		pc.close()
		pc.srv.unregisterPeer(pc)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
	}
}

func (pc *PeerClient) readLoop(ctx context.Context, conn *websocket.Conn, host string) {
	for {
		_, msg, err := conn.Read(ctx)
		if err != nil {
			log.Printf("peer %s: read: %v", host, err)
			return
		}
		var env Envelope
		if err := json.Unmarshal(msg, &env); err != nil {
			continue
		}
		if env.Type != MsgClipboard {
			continue
		}
		var clip ClipboardMsg
		if err := json.Unmarshal(env.Data, &clip); err != nil {
			continue
		}
		if clip.Text != "" && pc.onRemote != nil {
			pc.onRemote(clip.Text, clip.From)
		}
	}
}

func (pc *PeerClient) keepalive(ctx context.Context, conn *websocket.Conn) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	ping, _ := json.Marshal(Envelope{Type: MsgPing})
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			conn.Write(ctx, websocket.MessageText, ping)
		}
	}
}

// Send pushes clipboard text to the connected peer, if any.
func (pc *PeerClient) Send(text, from string) {
	pc.connMu.Lock()
	conn := pc.conn
	pc.connMu.Unlock()
	if conn == nil {
		return
	}
	payload, _ := json.Marshal(ClipboardMsg{Text: text, Ts: time.Now().UnixMilli(), From: from})
	msg, _ := json.Marshal(Envelope{Type: MsgClipboard, Data: payload})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn.Write(ctx, websocket.MessageText, msg)
}

func (pc *PeerClient) close() {
	pc.connMu.Lock()
	defer pc.connMu.Unlock()
	if pc.conn != nil {
		pc.conn.Close(websocket.StatusNormalClosure, "bye")
		pc.conn = nil
	}
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
