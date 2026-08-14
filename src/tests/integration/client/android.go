// Package client provides a simple WebSocket client that simulates the Android
// app for integration tests.
package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coder/websocket"

	"clipshare/src/internal/clip"
	"clipshare/src/internal/consts"
	"clipshare/src/internal/protocol"
)

// Android simulates the Android ClipShare client.
type Android struct {
	conn      *websocket.Conn
	msgs      chan protocol.ClipboardMsg
	closeOnce sync.Once
	closeErr  error
}

// Dial connects to a clipshare daemon at the given host:port address.
// If tlsCfg is nil the connection is ws://, otherwise wss:// with the given config.
func Dial(ctx context.Context, addr string, tlsCfg *tls.Config, token string) (*Android, error) {
	scheme := "ws"
	opts := &websocket.DialOptions{}
	if tlsCfg != nil {
		scheme = "wss"
		opts.HTTPClient = &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsCfg},
		}
	}
	u := url.URL{Scheme: scheme, Host: addr, Path: consts.WSPath}
	if token != "" {
		q := u.Query()
		q.Set("token", token)
		u.RawQuery = q.Encode()
	}

	conn, _, err := websocket.Dial(ctx, u.String(), opts)
	if err != nil {
		return nil, err
	}
	conn.SetReadLimit(10 * 1024 * 1024)

	a := &Android{
		conn: conn,
		msgs: make(chan protocol.ClipboardMsg, 16),
	}

	if err := a.sendHello(ctx); err != nil {
		conn.Close(websocket.StatusInternalError, "hello failed")
		return nil, err
	}

	go a.readLoop()
	return a, nil
}

func (a *Android) sendHello(ctx context.Context) error {
	hello, _ := json.Marshal(protocol.Envelope{
		Type: protocol.MsgHello,
		Data: protocol.MustJSON(protocol.Hello{
			Name:     "Android-Test",
			Platform: protocol.PlatformAndroid,
			Version:  "1.0",
		}),
	})
	return a.conn.Write(ctx, websocket.MessageText, hello)
}

func (a *Android) readLoop() {
	defer close(a.msgs)
	for {
		_, data, err := a.conn.Read(context.Background())
		if err != nil {
			return
		}
		var env protocol.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}
		if env.Type != protocol.MsgClipboard {
			continue
		}
		var msg protocol.ClipboardMsg
		if err := json.Unmarshal(env.Data, &msg); err != nil {
			continue
		}
		select {
		case a.msgs <- msg:
		default:
		}
	}
}

// SendText sends a text clipboard message to the daemon.
func (a *Android) SendText(ctx context.Context, text string) error {
	payload, _ := json.Marshal(protocol.ContentMsg(
		clipContent(text),
		protocol.NowMillis(),
		"Android-Test",
	))
	msg, _ := json.Marshal(protocol.Envelope{
		Type: protocol.MsgClipboard,
		Data: payload,
	})
	return a.conn.Write(ctx, websocket.MessageText, msg)
}

// WaitForText waits up to timeout for a text message matching the predicate.
func (a *Android) WaitForText(timeout time.Duration, match func(string) bool) (string, bool) {
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			return "", false
		case msg, ok := <-a.msgs:
			if !ok {
				return "", false
			}
			if msg.Type == protocol.ContentText && match(msg.Text) {
				return msg.Text, true
			}
		}
	}
}

// Close closes the connection.
func (a *Android) Close() error {
	a.closeOnce.Do(func() {
		a.closeErr = a.conn.Close(websocket.StatusNormalClosure, "bye")
	})
	return a.closeErr
}

func clipContent(text string) clip.Content {
	return clip.Content{Kind: clip.KindText, Text: text}
}
