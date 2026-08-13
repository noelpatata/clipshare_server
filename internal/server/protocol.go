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

type Hello struct {
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Version  string `json:"version"`
}

type ClipboardMsg struct {
	Text string `json:"text"`
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
