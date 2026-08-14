// Package api implements the localhost HTTP control API used by the CLI.
package api

import (
	"time"

	"clipshare/internal/clip"
	"clipshare/internal/protocol"
)

// SendRequest is the body of POST /send.
type SendRequest struct {
	Text string `json:"text"`
}

// StatusResponse is returned by GET /status.
type StatusResponse struct {
	Device    string                `json:"device"`
	Version   string                `json:"version"`
	Uptime    string                `json:"uptime"`
	Peers     []protocol.ClientInfo `json:"peers"`
	TokenAuth bool                  `json:"token_auth"`
	TLS       bool                  `json:"tls"`
	Mode      string                `json:"mode"`
}

// Broadcaster is the subset of the WebSocket server that the API needs to
// inject locally-originated clipboard content.
type Broadcaster interface {
	BroadcastLocal(content clip.Content, from string)
}

// StatusSource provides the information exposed by GET /status.
type StatusSource interface {
	DeviceName() string
	Uptime() time.Duration
	Clients() []protocol.ClientInfo
	TokenAuth() bool
	TLS() bool
	Mode() string
}
