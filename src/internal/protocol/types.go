// Package protocol defines the JSON-over-WebSocket wire format shared by the
// desktop daemon, its localhost API, and outbound peer connections.
package protocol

import "encoding/json"

// Message types exchanged over the WebSocket.
const (
	MsgHello     = "hello"
	MsgClipboard = "clipboard"
	MsgPing      = "ping"
	MsgPong      = "pong"
	MsgError     = "error"
)

// Content kinds carried in ClipboardMsg.Type.
const (
	ContentText  = "text"
	ContentImage = "image"
)

// Platform strings announced in hello messages.
const (
	PlatformDesktop = "desktop"
	PlatformAndroid = "android"
)

// Error codes returned in ErrorMsg.Code.
const (
	ErrBadHello     = "bad_hello"
	ErrNotWhitelisted = "not_whitelisted"
	ErrBadJSON      = "bad_json"
	ErrBadClipboard = "bad_clipboard"
	ErrUnknownType  = "unknown_type"
)

type Hello struct {
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Version  string `json:"version"`
}

// ClipboardMsg carries text or an image (base64-encoded bytes). A missing
// Type is treated as text by receivers for backward compatibility.
type ClipboardMsg struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
	Data string `json:"data,omitempty"` // base64-encoded bytes when Type == image
	Mime string `json:"mime,omitempty"` // e.g. "image/png" when Type == image
	Ts   int64  `json:"ts"`
	From string `json:"from"`
}

type ErrorMsg struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}

type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// ClientInfo is the public snapshot of a connected peer.
type ClientInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Version  string `json:"version"`
	IP       string `json:"ip"`
}
