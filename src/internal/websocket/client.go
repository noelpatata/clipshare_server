package websocket

import (
	"context"
	"sync"
	"time"

	"github.com/coder/websocket"

	"clipshare/src/internal/config"
)

// Client represents a connected peer.
type Client struct {
	id        string
	conn      *websocket.Conn
	ip        string
	name      string
	platform  string
	version   string
	cfg       *config.Config
	mu        sync.Mutex
	lastInKey uint64
	lastInAt  time.Time
}

func (s *Client) write(msg []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.Timing.WriteTimeout)
	defer cancel()
	return s.conn.Write(ctx, websocket.MessageText, msg)
}

// noteIncoming records the content key of a clip received from this client.
func (s *Client) noteIncoming(key uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastInKey = key
	s.lastInAt = time.Now()
}

// recentlySent reports whether this client sent us [key] within [window].
func (s *Client) recentlySent(key uint64, window time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastInKey == key && time.Since(s.lastInAt) < window
}
