package commands

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"clipshare/src/internal/clip"
	"clipshare/src/internal/config"
	"clipshare/src/internal/discover"
	"clipshare/src/internal/websocket"
	"clipshare/src/internal/version"
)

// watchCmd dispatches local or remote clipboard watching.
type watchCmd struct{}

func (watchCmd) Run(cfg *config.Config, args []string) error {
	if len(args) >= 1 && args[0] == "--remote" {
		timeout := time.Duration(0)
		for i := 1; i < len(args); i++ {
			if args[i] == "--timeout" && i+1 < len(args) {
				i++
				s, err := time.ParseDuration(args[i])
				if err != nil {
					return fmt.Errorf("bad --timeout: %v", err)
				}
				timeout = s
			}
		}
		return runWatchRemote(cfg, timeout)
	}
	if len(args) > 0 {
		return fmt.Errorf("usage: clipshare watch [--remote [--timeout <dur>]]")
	}
	return runWatch(cfg)
}

// runWatch prints clipboard changes until interrupted.
func runWatch(cfg *config.Config) error {
	clipboard, err := clip.NewForConfig(cfg)
	if err != nil {
		return err
	}
	defer clipboard.Close()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	w := clip.NewWatcher(clipboard, cfg.WatchInterval(), func(c clip.Content) {
		fmt.Println("---- clipboard ----")
		if c.Kind == clip.KindImage {
			fmt.Printf("[image %s, %d bytes]\n", c.Mime, len(c.Image))
		} else {
			fmt.Println(c.Text)
		}
	})
	w.Run(ctx)
	return nil
}

// runWatchRemote starts a temporary server that writes the first incoming
// push to the local clipboard and exits.
func runWatchRemote(cfg *config.Config, timeout time.Duration) error {
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
		log.Printf("received from %q -> writing to local clipboard", from)
		if err := clipboard.Write(content); err != nil {
			log.Printf("clipboard write: %v", err)
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
	go func() {
		if err := srv.ListenAndServe(ctx); err != nil {
			log.Printf("ws server: %v", err)
		}
	}()

	if err := discover.Start(ctx, cfg); err != nil {
		log.Printf("discovery: %v", err)
	}

	log.Printf("watch --remote: listening for a push (max %s, Ctrl-C to cancel)", timeout)
	select {
	case <-ctx.Done():
		return nil
	case <-received:
		return nil
	case <-time.After(timeout):
		log.Printf("watch --remote: timed out with no push")
		return nil
	}
}

// closeOnce closes c exactly once; safe to call from multiple goroutines.
func closeOnce(c chan struct{}) {
	select {
	case <-c:
	default:
		close(c)
	}
}
