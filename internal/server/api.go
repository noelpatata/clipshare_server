package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"clipshare/internal/clip"
)

type SendRequest struct {
	Text string `json:"text"`
}

type StatusResponse struct {
	Device    string       `json:"device"`
	Version   string       `json:"version"`
	Uptime    string       `json:"uptime"`
	Peers     []ClientInfo `json:"peers"`
	TokenAuth bool         `json:"token_auth"`
	TLS       bool         `json:"tls"`
	Mode      string       `json:"mode"`
}

// ListenAPI serves the localhost control API on 127.0.0.1.
func ListenAPI(ctx context.Context, s *Server, version string) error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.cfg.API.Port)
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, StatusResponse{
			Device:    s.cfg.DeviceName,
			Version:   version,
			Uptime:    s.Uptime().Truncate(time.Second).String(),
			Peers:     s.Clients(),
			TokenAuth: s.cfg.Token != "",
			TLS:       s.cfg.TLS.Enabled,
			Mode:      s.cfg.Connection.Mode,
		})
	})
	mux.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
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
		s.BroadcastLocal(clip.Content{Kind: clip.KindText, Text: req.Text}, s.cfg.DeviceName)
		writeJSON(w, map[string]bool{"ok": true})
	})
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
	return srv.Serve(ln)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
