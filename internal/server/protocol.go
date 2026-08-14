package server

import "encoding/json"

// Protocol message types exchanged over the WebSocket.
const (
	MsgHello     = "hello"
	MsgClipboard = "clipboard"
	MsgPing      = "ping"
	MsgPong      = "pong"
	MsgError     = "error"
)

// Clipboard content kinds carried in ClipboardMsg.Type.
const (
	ContentText  = "text"
	ContentImage = "image"
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
