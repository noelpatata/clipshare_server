package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"clipshare/src/internal/consts"
)

// Client talks to a clipshare daemon's localhost control API.
type Client struct {
	baseURL string
}

// NewClient creates a client for the daemon at the given host:port.
func NewClient(host string, port int) *Client {
	return &Client{baseURL: fmt.Sprintf("http://%s:%d", host, port)}
}

// Send pushes text through the running daemon's API.
func (c *Client) Send(text string) error {
	body, _ := json.Marshal(SendRequest{Text: text})
	resp, err := http.Post(
		c.baseURL+consts.APISendPath,
		consts.ContentTypeJSON,
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daemon error %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return nil
}

// Status fetches the daemon's status.
func (c *Client) Status() (StatusResponse, error) {
	var st StatusResponse
	resp, err := http.Get(c.baseURL + consts.APIStatusPath)
	if err != nil {
		return st, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return st, fmt.Errorf("daemon error %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return st, err
	}
	return st, nil
}
