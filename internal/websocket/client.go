package websocket

import (
	"context"
	"sync"

	"github.com/coder/websocket"

	"clipshare/internal/config"
)

// Client represents a connected peer.
type Client struct {
	id       string
	conn     *websocket.Conn
	ip       string
	name     string
	platform string
	version  string
	cfg      *config.Config
	mu       sync.Mutex
}

func (s *Client) write(msg []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.Timing.WriteTimeout)
	defer cancel()
	return s.conn.Write(ctx, websocket.MessageText, msg)
}
