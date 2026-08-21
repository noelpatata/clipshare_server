package websocket

import (
	"encoding/json"

	"clipshare/src/internal/clip"
	"clipshare/src/internal/log"
	"clipshare/src/internal/protocol"
)

// Broadcast sends clipboard content to every connected client except the
// origin. skipID excludes the sending connection and skipName excludes any
// other connection from the same device (so a reconnecting client never
// receives a relay of its own message). "" means no exclusion. Dead clients
// are dropped on write failure so ghost connections cannot linger.
func (s *Server) Broadcast(content clip.Content, from, skipID, skipName string) {
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
		if skipName != "" && c.name == skipName {
			continue
		}
		if err := c.write(msg); err != nil {
			log.Errorf("broadcast to %s: %v", id, err)
			delete(s.clients, id)
		}
	}
	for pc := range s.peers {
		pc.Send(content, from)
	}
}

// BroadcastLocal fans out a local clipboard change to all connected clients.
func (s *Server) BroadcastLocal(content clip.Content, from string) {
	s.Broadcast(content, from, "", "")
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
