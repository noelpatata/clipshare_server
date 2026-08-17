// Package send implements the `clipshare send` subcommand.
package send

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"clipshare/src/internal/api"
	"clipshare/src/internal/clip"
	"clipshare/src/internal/config"
	"clipshare/src/internal/consts"
	"clipshare/src/internal/discover"
	"clipshare/src/internal/log"
	"clipshare/src/internal/version"
	"clipshare/src/internal/websocket"
)

// Command pushes text (or the current clipboard) to peers. It sends via the
// running daemon by default, or runs a transient one-shot server with
// --oneshot (or automatically when no daemon is up).
type Command struct{}

func (Command) Run(cfg *config.Config, args []string) error {
	var forceOneShot bool
	var textArgs []string
	for _, arg := range args {
		if arg == "--oneshot" {
			forceOneShot = true
		} else {
			textArgs = append(textArgs, arg)
		}
	}

	text := strings.Join(textArgs, " ")
	if strings.TrimSpace(text) == "" {
		cb, err := clip.NewForConfig(cfg)
		if err != nil {
			return fmt.Errorf("clipboard: %w", err)
		}
		defer cb.Close()
		content, err := cb.Read()
		if err != nil {
			return fmt.Errorf("clipboard read: %w", err)
		}
		if content.Kind != clip.KindText || strings.TrimSpace(content.Text) == "" {
			return errors.New("clipboard is empty")
		}
		text = content.Text
	}

	content := clip.Content{Kind: clip.KindText, Text: text}

	if forceOneShot {
		return runOneShot(cfg, content)
	}

	client := api.NewClient(consts.Localhost, cfg.API.Port)
	if err := client.Send(text); err != nil {
		if errors.Is(err, syscall.ECONNREFUSED) {
			log.Infof("no daemon running; starting transient one-shot")
			return runOneShot(cfg, content)
		}
		return err
	}
	fmt.Println("sent")
	return nil
}

// runOneShot starts a short-lived server, broadcasts the content once a peer
// connects, then exits after a grace period.
func runOneShot(cfg *config.Config, content clip.Content) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := websocket.New(cfg, version.Version, func(c clip.Content, from string) {
		log.Debugf("received from %q during one-shot (ignored)", from)
	})
	srv.SetMaxClients(1)
	delivered := make(chan struct{})
	var once sync.Once
	srv.SetConnectHook(func() {
		once.Do(func() {
			srv.BroadcastLocal(content, cfg.DeviceName)
			log.Infof("one-shot: delivered to client")
			close(delivered)
		})
	})
	go func() {
		if err := srv.ListenAndServe(ctx); err != nil {
			log.Errorf("ws server: %v", err)
		}
	}()

	if err := discover.Start(ctx, cfg); err != nil {
		log.Errorf("discovery: %v", err)
	}

	timeout := cfg.Timing.OneShotTimeout
	grace := cfg.Timing.OneShotGrace
	log.Infof("one-shot: waiting for a client to connect (max %s)...", timeout)
	select {
	case <-ctx.Done():
		return nil
	case <-time.After(timeout):
		log.Infof("one-shot: timed out with no client")
		return nil
	case <-delivered:
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(grace):
			return nil
		}
	}
}
