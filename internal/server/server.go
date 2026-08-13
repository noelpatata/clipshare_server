package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/coder/websocket"

	"clipshare/internal/config"
)

// Client represents a connected peer.
type Client struct {
	id       string
	conn     *websocket.Conn
	name     string
	platform string
	version  string
	mu       sync.Mutex
}

// ClientInfo is the public snapshot of a connected peer.
type ClientInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Version  string `json:"version"`
}

// Server accepts WebSocket peers and brokers clipboard messages.
type Server struct {
	cfg              *config.Config
	version          string
	clients          map[string]*Client
	peers            map[*PeerClient]struct{}
	mu               sync.Mutex
	onRemoteClip     func(text, from string)
	onClientConnect  func()
	lastBroadcast    string
	lastBroadcastT   time.Time
	started          time.Time
}

func New(cfg *config.Config, version string, onRemote func(text, from string)) *Server {
	return &Server{
		cfg:          cfg,
		version:      version,
		clients:      make(map[string]*Client),
		peers:        make(map[*PeerClient]struct{}),
		onRemoteClip: onRemote,
		started:      time.Now(),
	}
}

// SetConnectHook registers a callback fired after a client completes its
// hello handshake. Used by the one-shot share mode.
func (s *Server) SetConnectHook(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onClientConnect = fn
}

func (s *Server) registerPeer(pc *PeerClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.peers[pc] = struct{}{}
}

func (s *Server) unregisterPeer(pc *PeerClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.peers, pc)
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	addr := fmt.Sprintf("0.0.0.0:%d", s.cfg.Server.Port)
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWS)
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		<-ctx.Done()
		srv.Close()
	}()
	log.Printf("ws server listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Token != "" && r.URL.Query().Get("token") != s.cfg.Token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("ws accept: %v", err)
		return
	}
	client := &Client{id: newID(), conn: c}
	s.mu.Lock()
	s.clients[client.id] = client
	s.mu.Unlock()
	log.Printf("client connected: %s", client.id)
	s.handleClient(r.Context(), client)
}

func (s *Server) handleClient(ctx context.Context, c *Client) {
	defer func() {
		s.mu.Lock()
		delete(s.clients, c.id)
		s.mu.Unlock()
		c.conn.Close(websocket.StatusNormalClosure, "bye")
		log.Printf("client disconnected: %s", c.id)
	}()

	readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, msg, err := c.conn.Read(readCtx)
	if err != nil {
		return
	}
	var env Envelope
	if err := json.Unmarshal(msg, &env); err != nil || env.Type != MsgHello {
		s.sendError(c, "bad_hello", "expected hello message")
		return
	}
	var hello Hello
	if err := json.Unmarshal(env.Data, &hello); err != nil {
		s.sendError(c, "bad_hello", "invalid hello payload")
		return
	}
	c.name = hello.Name
	c.platform = hello.Platform
	c.version = hello.Version
	log.Printf("hello from %q (%s v%s)", hello.Name, hello.Platform, hello.Version)

	s.mu.Lock()
	hook := s.onClientConnect
	s.mu.Unlock()
	if hook != nil {
		hook()
	}

	// Reply with our own identity so clients can show the connected device name.
	reply, _ := json.Marshal(Envelope{Type: MsgHello, Data: mustJSON(Hello{
		Name: s.cfg.DeviceName, Platform: "desktop", Version: s.version,
	})})
	c.write(reply)

	for {
		_, msg, err := c.conn.Read(context.Background())
		if err != nil {
			return
		}
		var env Envelope
		if err := json.Unmarshal(msg, &env); err != nil {
			s.sendError(c, "bad_json", "malformed message")
			continue
		}
		switch env.Type {
		case MsgPing:
			s.pong(c)
		case MsgPong:
		case MsgClipboard:
			var clip ClipboardMsg
			if err := json.Unmarshal(env.Data, &clip); err != nil {
				s.sendError(c, "bad_clipboard", "invalid clipboard payload")
				continue
			}
			s.handleClipboard(c, clip)
		default:
			s.sendError(c, "unknown_type", "unrecognized message type")
		}
	}
}

func (s *Server) handleClipboard(from *Client, clip ClipboardMsg) {
	if clip.Text == "" {
		return
	}
	// Loop protection: ignore content identical to what we last wrote locally
	// (echo of our own broadcast).
	s.mu.Lock()
	isEcho := clip.Text == s.lastBroadcast && time.Since(s.lastBroadcastT) < 30*time.Second
	if !isEcho {
		s.lastBroadcast = clip.Text
		s.lastBroadcastT = time.Now()
	}
	s.mu.Unlock()
	if isEcho {
		return
	}
	if s.onRemoteClip != nil {
		s.onRemoteClip(clip.Text, clip.From)
	}
	s.Broadcast(clip.Text, clip.From, from.id)
}

// Broadcast sends clipboard content to every connected client except the
// origin (skipID "" means broadcast to all).
func (s *Server) Broadcast(text, from, skipID string) {
	payload, _ := json.Marshal(ClipboardMsg{Text: text, Ts: time.Now().UnixMilli(), From: from})
	msg, _ := json.Marshal(Envelope{Type: MsgClipboard, Data: payload})
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, c := range s.clients {
		if skipID != "" && id == skipID {
			continue
		}
		if err := c.write(msg); err != nil {
			log.Printf("broadcast to %s: %v", id, err)
		}
	}
	for pc := range s.peers {
		pc.Send(text, from)
	}
}

// BroadcastLocal fans out a local clipboard change to all connected clients.
func (s *Server) BroadcastLocal(text, from string) {
	s.Broadcast(text, from, "")
}

func (s *Client) write(msg []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return s.conn.Write(ctx, websocket.MessageText, msg)
}

func (s *Server) sendError(c *Client, code, msg string) {
	payload, _ := json.Marshal(ErrorMsg{Code: code, Msg: msg})
	m, _ := json.Marshal(Envelope{Type: MsgError, Data: payload})
	if err := c.write(m); err != nil {
		log.Printf("error send: %v", err)
	}
}

func (s *Server) pong(c *Client) {
	m, _ := json.Marshal(Envelope{Type: MsgPong})
	c.write(m)
}

// Clients returns a snapshot of connected clients.
func (s *Server) Clients() []ClientInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ClientInfo, 0, len(s.clients))
	for _, c := range s.clients {
		out = append(out, ClientInfo{
			ID: c.id, Name: c.name, Platform: c.platform, Version: c.version,
		})
	}
	return out
}

// Uptime returns how long the server has been running.
func (s *Server) Uptime() time.Duration { return time.Since(s.started) }

var idMu sync.Mutex
var idSeq uint64

func newID() string {
	idMu.Lock()
	defer idMu.Unlock()
	idSeq++
	return strconv.FormatUint(idSeq, 36)
}
