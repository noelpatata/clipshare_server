package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"clipshare/src/internal/api"
	"clipshare/src/internal/clip"
	"clipshare/src/internal/config"
	"clipshare/src/internal/protocol"
)

type fakeStatusSource struct {
	device    string
	version   string
	uptime    time.Duration
	clients   []protocol.ClientInfo
	tokenAuth bool
	tls       bool
	mode      string
}

func (f *fakeStatusSource) DeviceName() string        { return f.device }
func (f *fakeStatusSource) Version() string           { return f.version }
func (f *fakeStatusSource) Uptime() time.Duration     { return f.uptime }
func (f *fakeStatusSource) Clients() []protocol.ClientInfo { return f.clients }
func (f *fakeStatusSource) TokenAuth() bool           { return f.tokenAuth }
func (f *fakeStatusSource) TLS() bool                 { return f.tls }
func (f *fakeStatusSource) Mode() string              { return f.mode }

type fakeBroadcaster struct {
	calls []struct {
		content clip.Content
		from    string
	}
}

func (f *fakeBroadcaster) BroadcastLocal(content clip.Content, from string) {
	f.calls = append(f.calls, struct {
		content clip.Content
		from    string
	}{content, from})
}

func newTestServer(t *testing.T, src api.StatusSource, bcast api.Broadcaster) *httptest.Server {
	t.Helper()
	cfg := config.Default()
	srv := api.NewServer(cfg, src, bcast, "test-version")
	return httptest.NewServer(srv.Handler())
}

func TestHandleStatus(t *testing.T) {
	src := &fakeStatusSource{
		device:  "dev",
		version: "1.2.3",
		uptime:  5 * time.Minute,
		clients: []protocol.ClientInfo{{ID: "1", Name: "phone", Platform: "android", Version: "1", IP: "10.0.0.2"}},
		tls:     true,
		mode:    config.ModeDiscover,
	}
	bcast := &fakeBroadcaster{}
	server := newTestServer(t, src, bcast)
	defer server.Close()

	resp, err := http.Get(server.URL + "/status")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}

	var got api.StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Device != "dev" {
		t.Errorf("device: got %q, want dev", got.Device)
	}
	if got.Version != "test-version" {
		t.Errorf("version: got %q, want test-version", got.Version)
	}
	if got.TLS != true {
		t.Errorf("tls: got %v, want true", got.TLS)
	}
	if got.Mode != config.ModeDiscover {
		t.Errorf("mode: got %q, want discover", got.Mode)
	}
	if len(got.Peers) != 1 || got.Peers[0].Name != "phone" {
		t.Errorf("peers: got %+v", got.Peers)
	}
}

func TestHandleStatusMethodNotAllowed(t *testing.T) {
	server := newTestServer(t, &fakeStatusSource{}, &fakeBroadcaster{})
	defer server.Close()

	resp, err := http.Post(server.URL+"/status", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want 405", resp.StatusCode)
	}
}

func TestHandleSend(t *testing.T) {
	bcast := &fakeBroadcaster{}
	src := &fakeStatusSource{device: "dev"}
	server := newTestServer(t, src, bcast)
	defer server.Close()

	body, _ := json.Marshal(api.SendRequest{Text: "hello"})
	resp, err := http.Post(server.URL+"/send", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}

	if len(bcast.calls) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(bcast.calls))
	}
	if bcast.calls[0].content.Text != "hello" {
		t.Errorf("text: got %q, want hello", bcast.calls[0].content.Text)
	}
	if bcast.calls[0].from != "dev" {
		t.Errorf("from: got %q, want dev", bcast.calls[0].from)
	}
}

func TestHandleSendEmptyText(t *testing.T) {
	server := newTestServer(t, &fakeStatusSource{}, &fakeBroadcaster{})
	defer server.Close()

	body, _ := json.Marshal(api.SendRequest{Text: ""})
	resp, err := http.Post(server.URL+"/send", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleSendMethodNotAllowed(t *testing.T) {
	server := newTestServer(t, &fakeStatusSource{}, &fakeBroadcaster{})
	defer server.Close()

	resp, err := http.Get(server.URL + "/send")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want 405", resp.StatusCode)
	}
}

func TestHandleSendBadJSON(t *testing.T) {
	server := newTestServer(t, &fakeStatusSource{}, &fakeBroadcaster{})
	defer server.Close()

	resp, err := http.Post(server.URL+"/send", "application/json", strings.NewReader("not json"))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}
