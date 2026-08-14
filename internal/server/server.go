package server

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"clipshare/internal/certs"
	"clipshare/internal/clip"
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
	mu       sync.Mutex
}

// ClientInfo is the public snapshot of a connected peer.
type ClientInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Version  string `json:"version"`
	IP       string `json:"ip"`
}

// Server accepts WebSocket peers and brokers clipboard messages.
type Server struct {
	cfg             *config.Config
	version         string
	clients         map[string]*Client
	peers           map[*PeerClient]struct{}
	mu              sync.Mutex
	onRemoteClip    func(content clip.Content, from string)
	onClientConnect func()
	lastBroadcast   uint64
	lastBroadcastT  time.Time
	started         time.Time
}

func New(cfg *config.Config, version string, onRemote func(content clip.Content, from string)) *Server {
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
	srv := &http.Server{Handler: mux}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	scheme := "ws"
	if s.cfg.TLS.Enabled {
		tlsCfg, err := s.tlsConfig()
		if err != nil {
			ln.Close()
			return err
		}
		ln = tls.NewListener(ln, tlsCfg)
		scheme = "wss"
	}
	go func() {
		<-ctx.Done()
		srv.Close()
		ln.Close()
	}()
	log.Printf("%s server listening on %s", scheme, addr)
	if err := srv.Serve(ln); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}

// tlsConfig builds the server-side TLS config: this device's cert plus a
// private CA pool and mandatory client certificates (mutual TLS).
func (s *Server) tlsConfig() (*tls.Config, error) {
	cert, err := certs.LoadKeyPair(s.cfg.TLS.Cert, s.cfg.TLS.Key)
	if err != nil {
		return nil, err
	}
	pool, err := certs.LoadPool(s.cfg.TLS.CA)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// peerTLSConfig builds a client-side TLS config for outbound peer dials:
// this device's cert is presented to the remote daemon and the CA pool
// verifies the remote daemon's cert.
func (s *Server) peerTLSConfig() (*tls.Config, error) {
	cert, err := certs.LoadKeyPair(s.cfg.TLS.Cert, s.cfg.TLS.Key)
	if err != nil {
		return nil, err
	}
	pool, err := certs.LoadPool(s.cfg.TLS.CA)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		MinVersion:   tls.VersionTLS12,
	}, nil
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
	client := &Client{id: newID(), conn: c, ip: clientIP(r.RemoteAddr)}
	s.mu.Lock()
	s.clients[client.id] = client
	s.mu.Unlock()
	log.Printf("client connected: %s (%s)", client.id, client.ip)
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

	// Whitelist mode: only devices matching the whitelist by IP or name are
	// allowed to talk to us (in addition to token/TLS checks).
	if s.cfg.Connection.Mode == config.ModeWhitelist && !s.cfg.InWhitelist(c.ip, hello.Name) {
		s.sendError(c, "not_whitelisted", "device not in whitelist")
		log.Printf("rejected non-whitelisted hello from %q (%s)", hello.Name, c.ip)
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
			var cm ClipboardMsg
			if err := json.Unmarshal(env.Data, &cm); err != nil {
				s.sendError(c, "bad_clipboard", "invalid clipboard payload")
				continue
			}
			s.handleClipboard(c, cm)
		default:
			s.sendError(c, "unknown_type", "unrecognized message type")
		}
	}
}

func (s *Server) handleClipboard(from *Client, msg ClipboardMsg) {
	content, ok := msgToContent(msg)
	if !ok {
		return
	}
	// Loop protection: ignore content identical to what we last wrote locally
	// (echo of our own broadcast).
	key := contentKey(content)
	s.mu.Lock()
	isEcho := key == s.lastBroadcast && time.Since(s.lastBroadcastT) < 30*time.Second
	if !isEcho {
		s.lastBroadcast = key
		s.lastBroadcastT = time.Now()
	}
	s.mu.Unlock()
	if isEcho {
		return
	}
	if s.onRemoteClip != nil {
		s.onRemoteClip(content, msg.From)
	}
	s.Broadcast(content, msg.From, from.id)
}

// Broadcast sends clipboard content to every connected client except the
// origin (skipID "" means broadcast to all).
func (s *Server) Broadcast(content clip.Content, from, skipID string) {
	if content.Kind == clip.KindImage && int64(len(content.Image)) > s.cfg.MaxImageBytes {
		log.Printf("dropping image broadcast: %d bytes exceeds max_image_bytes=%d",
			len(content.Image), s.cfg.MaxImageBytes)
		return
	}
	payload, _ := json.Marshal(contentMsg(content, time.Now().UnixMilli(), from))
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
		pc.Send(content, from)
	}
}

// BroadcastLocal fans out a local clipboard change to all connected clients.
func (s *Server) BroadcastLocal(content clip.Content, from string) {
	s.Broadcast(content, from, "")
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
			ID: c.id, Name: c.name, Platform: c.platform, Version: c.version, IP: c.ip,
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

// contentMsg converts clip.Content into a wire ClipboardMsg.
func contentMsg(c clip.Content, ts int64, from string) ClipboardMsg {
	if c.Kind == clip.KindImage {
		mime := c.Mime
		if mime == "" {
			mime = "image/png"
		}
		return ClipboardMsg{
			Type: ContentImage,
			Data: base64.StdEncoding.EncodeToString(c.Image),
			Mime: mime,
			Ts:   ts,
			From: from,
		}
	}
	return ClipboardMsg{Type: ContentText, Text: c.Text, Ts: ts, From: from}
}

// msgToContent converts a wire ClipboardMsg into clip.Content, or reports
// false when the payload is empty.
func msgToContent(m ClipboardMsg) (clip.Content, bool) {
	switch m.Type {
	case ContentImage:
		data, err := base64.StdEncoding.DecodeString(m.Data)
		if err != nil || len(data) == 0 {
			return clip.Content{}, false
		}
		mime := m.Mime
		if mime == "" {
			mime = "image/png"
		}
		return clip.Content{Kind: clip.KindImage, Image: data, Mime: mime}, true
	default: // "" or "text"
		if m.Text == "" {
			return clip.Content{}, false
		}
		return clip.Content{Kind: clip.KindText, Text: m.Text}, true
	}
}

// contentKey produces a hash identifying content for loop protection.
func contentKey(c clip.Content) uint64 {
	h := fnv.New64a()
	if c.Kind == clip.KindImage {
		h.Write([]byte{0})
		h.Write(c.Image)
		h.Write([]byte(c.Mime))
	} else {
		h.Write([]byte{1})
		h.Write([]byte(c.Text))
	}
	return h.Sum64()
}

// clientIP extracts the IP from a net/http RemoteAddr string.
func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return strings.Trim(host, "[]")
}
