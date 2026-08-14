package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"clipshare/internal/clip"
	"clipshare/internal/config"
	"clipshare/internal/consts"
)

// Server serves the localhost control API.
type Server struct {
	cfg    *config.Config
	src    StatusSource
	bcast  Broadcaster
	version string
}

// NewServer creates an API server bound to the configured API address.
func NewServer(cfg *config.Config, src StatusSource, bcast Broadcaster, version string) *Server {
	return &Server{cfg: cfg, src: src, bcast: bcast, version: version}
}

// Listen starts the API server and blocks until ctx is cancelled.
func (s *Server) Listen(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.API.Bind, s.cfg.API.Port)
	mux := http.NewServeMux()
	mux.HandleFunc(consts.APIStatusPath, s.handleStatus)
	mux.HandleFunc(consts.APISendPath, s.handleSend)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: mux}
	go func() {
		<-ctx.Done()
		srv.Close()
	}()
	log.Printf("api listening on %s", addr)
	if err := srv.Serve(ln); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, StatusResponse{
		Device:    s.src.DeviceName(),
		Version:   s.version,
		Uptime:    s.src.Uptime().Truncate(time.Second).String(),
		Peers:     s.src.Clients(),
		TokenAuth: s.src.TokenAuth(),
		TLS:       s.src.TLS(),
		Mode:      s.src.Mode(),
	})
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req SendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.Text == "" {
		http.Error(w, "text required", http.StatusBadRequest)
		return
	}
	s.bcast.BroadcastLocal(clip.Content{Kind: clip.KindText, Text: req.Text}, s.src.DeviceName())
	writeJSON(w, map[string]bool{"ok": true})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", consts.ContentTypeJSON)
	json.NewEncoder(w).Encode(v)
}
