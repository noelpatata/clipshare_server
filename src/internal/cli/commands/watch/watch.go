// Package watch implements the `clipshare watch` subcommand.
package watch

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"clipshare/src/internal/clip"
	"clipshare/src/internal/config"
	"clipshare/src/internal/discover"
	"clipshare/src/internal/log"
	"clipshare/src/internal/version"
	"clipshare/src/internal/websocket"
)

// Command waits for a single remote push and writes it to the local clipboard.
type Command struct{}

func (Command) Run(cfg *config.Config, args []string) error {
	timeout := time.Duration(0)
	for i := 0; i < len(args); i++ {
		if args[i] == "--timeout" && i+1 < len(args) {
			i++
			s, err := time.ParseDuration(args[i])
			if err != nil {
				return fmt.Errorf("bad --timeout: %v", err)
			}
			timeout = s
		} else {
			return fmt.Errorf("usage: clipshare watch [--timeout <dur>]")
		}
	}
	return run(cfg, timeout)
}

// run starts a temporary server that writes the first incoming push to the
// local clipboard, then exits.
func run(cfg *config.Config, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = cfg.Timing.WatchRemoteTimeout
	}
	clipboard, err := clip.NewForConfig(cfg)
	if err != nil {
		return err
	}
	defer clipboard.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	received := make(chan struct{})
	srv := websocket.New(cfg, version.Version, func(content clip.Content, from string) {
		if content.Kind == clip.KindImage {
			if len(content.Image) == 0 {
				return
			}
		} else if content.Text == "" {
			return
		}
		log.Debugf("received from %q -> writing to local clipboard", from)
		if err := clipboard.Write(content); err != nil {
			log.Errorf("clipboard write: %v", err)
			return
		}
		fmt.Printf("received from %s\n", from)
		if content.Kind == clip.KindImage {
			fmt.Printf("[image %s, %d bytes]\n", content.Mime, len(content.Image))
		} else {
			fmt.Println(content.Text)
		}
		closeOnce(received)
	})
	srv.SetMaxClients(1)
	go func() {
		if err := srv.ListenAndServe(ctx); err != nil {
			log.Errorf("ws server: %v", err)
		}
	}()

	if err := discover.Start(ctx, cfg); err != nil {
		log.Errorf("discovery: %v", err)
	}

	log.Infof("watch: listening for a push (max %s, Ctrl-C to cancel)", timeout)
	select {
	case <-ctx.Done():
		return nil
	case <-received:
		return nil
	case <-time.After(timeout):
		log.Infof("watch: timed out with no push")
		return nil
	}
}

func closeOnce(c chan struct{}) {
	select {
	case <-c:
	default:
		close(c)
	}
}
