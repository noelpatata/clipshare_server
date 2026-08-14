package websocket

import (
	"encoding/json"

	"clipshare/src/internal/clip"
	"clipshare/src/internal/log"
	"clipshare/src/internal/protocol"
)

// Broadcast sends clipboard content to every connected client except the
// origin (skipID "" means broadcast to all).
func (s *Server) Broadcast(content clip.Content, from, skipID string) {
	if content.Kind == clip.KindImage && int64(len(content.Image)) > s.cfg.MaxImageBytes {
		log.Warnf("dropping image broadcast: %d bytes exceeds max_image_bytes=%d",
			len(content.Image), s.cfg.MaxImageBytes)
		return
	}
	payload, _ := json.Marshal(protocol.ContentMsg(content, protocol.NowMillis(), from))
	msg, _ := json.Marshal(protocol.Envelope{Type: protocol.MsgClipboard, Data: payload})
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, c := range s.clients {
		if skipID != "" && id == skipID {
			continue
		}
		if err := c.write(msg); err != nil {
			log.Errorf("broadcast to %s: %v", id, err)
		}
	}
	for pc := range s.peers {
		pc.Send(content, from)
	}
}

// BroadcastLocal fans out a local clipboard change to all connected clients.
func (s *Server) BroadcastLocal(content clip.Content, from string) {
	s.Broadcast(content, from, "")
}

func (s *Server) sendError(c *Client, code, msg string) {
	payload, _ := json.Marshal(protocol.ErrorMsg{Code: code, Msg: msg})
	m, _ := json.Marshal(protocol.Envelope{Type: protocol.MsgError, Data: payload})
	if err := c.write(m); err != nil {
		log.Errorf("error send: %v", err)
	}
}

func (s *Server) pong(c *Client) {
	m, _ := json.Marshal(protocol.Envelope{Type: protocol.MsgPong})
	c.write(m)
}
