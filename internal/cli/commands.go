package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"clipshare/internal/clip"
	"clipshare/internal/config"
	"clipshare/internal/discover"
	"clipshare/internal/server"
	"clipshare/internal/version"
)

func runDaemon(noWatch bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// The clipboard backend is always initialized so remote content can be
	// written locally; only the outgoing watcher is optional.
	clipboard, err := clip.New()
	if err != nil {
		return err
	}
	defer clipboard.Close()

	var watcher *clip.Watcher
	srv := server.New(cfg, version.Version, func(content clip.Content, from string) {
		if content.Kind == clip.KindImage {
			if len(content.Image) == 0 {
				return
			}
		} else if content.Text == "" {
			return
		}
		log.Printf("remote clipboard from %q -> writing to local", from)
		if watcher != nil {
			watcher.LocalWrite(content)
		} else {
			clipboard.Write(content)
		}
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if !noWatch {
		watcher = clip.NewWatcher(clipboard, cfg.WatchInterval(), func(content clip.Content) {
			log.Printf("local clipboard changed")
			srv.BroadcastLocal(content, cfg.DeviceName)
		})
		go watcher.Run(ctx)
	} else {
		log.Printf("clipboard watching disabled (--no-watch); only receiving")
	}

	go func() {
		if err := srv.ListenAndServe(ctx); err != nil {
			log.Printf("ws server: %v", err)
		}
	}()
	go srv.RunPeers(ctx, version.Version, func(content clip.Content, from string) {
		if watcher != nil {
			watcher.LocalWrite(content)
		} else {
			clipboard.Write(content)
		}
	})

	if cfg.Connection.Mode == config.ModeDiscover {
		if err := discover.Advertise(ctx, cfg.DeviceName, cfg.Server.Port, cfg.Mdns, cfg.TLS.Enabled); err != nil {
			log.Printf("mdns advertise failed: %v", err)
		}
		go func() {
			if err := discover.Beacon(ctx, cfg.DeviceName, cfg.Server.Port,
				cfg.BroadcastInterval(), cfg.TLS.Enabled); err != nil {
				log.Printf("udp beacon failed: %v", err)
			}
		}()
	} else {
		log.Printf("connection mode %q: not advertising (whitelist-only)", cfg.Connection.Mode)
	}

	go func() {
		if err := server.ListenAPI(ctx, srv, version.Version); err != nil {
			log.Printf("api server: %v", err)
		}
	}()

	proto := "ws"
	if cfg.TLS.Enabled {
		proto = "wss"
	}
	log.Printf("clipshare %s running as %q (%s :%d, api :%d)",
		version.Version, cfg.DeviceName, proto, cfg.Server.Port, cfg.API.Port)
	<-ctx.Done()
	log.Printf("shutting down")
	return nil
}

func runSend(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: clipshare send <text>")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := apiSend(cfg, strings.Join(args, " ")); err != nil {
		return err
	}
	fmt.Println("sent")
	return nil
}

// apiSend pushes text through the running daemon's localhost API.
func apiSend(cfg *config.Config, text string) error {
	body, _ := json.Marshal(server.SendRequest{Text: text})
	resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/send", cfg.API.Port),
		"application/json", bytes.NewReader(body))
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

// runShare pushes text (or the current selection) to connected peers. If no
// daemon is running it starts a transient one, sends once a client connects,
// then exits.
func runShare(args []string) error {
	text := strings.Join(args, " ")
	if strings.TrimSpace(text) == "" {
		sel, err := clip.Selection()
		if err != nil {
			return fmt.Errorf("no text given and %w", err)
		}
		text = sel
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("nothing to share")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if err := apiSend(cfg, text); err != nil {
		if errors.Is(err, syscall.ECONNREFUSED) {
			log.Printf("no daemon running; starting transient one-shot")
			return runOneShot(cfg, clip.Content{Kind: clip.KindText, Text: text})
		}
		return err
	}
	fmt.Println("sent")
	return nil
}

// runOneShot runs a short-lived server, broadcasts content once a peer
// connects, waits briefly, then exits.
func runOneShot(cfg *config.Config, content clip.Content) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := server.New(cfg, version.Version, func(c clip.Content, from string) {
		log.Printf("received from %q during one-shot (ignored)", from)
	})
	delivered := make(chan struct{})
	var once sync.Once
	srv.SetConnectHook(func() {
		once.Do(func() {
			srv.BroadcastLocal(content, cfg.DeviceName)
			log.Printf("one-shot: delivered to client")
			close(delivered)
		})
	})
	go func() {
		if err := srv.ListenAndServe(ctx); err != nil {
			log.Printf("ws server: %v", err)
		}
	}()

	if cfg.Connection.Mode == config.ModeDiscover {
		if err := discover.Advertise(ctx, cfg.DeviceName, cfg.Server.Port, cfg.Mdns, cfg.TLS.Enabled); err != nil {
			log.Printf("mdns advertise failed: %v", err)
		}
		go func() {
			if err := discover.Beacon(ctx, cfg.DeviceName, cfg.Server.Port,
				cfg.BroadcastInterval(), cfg.TLS.Enabled); err != nil {
				log.Printf("udp beacon failed: %v", err)
			}
		}()
	}

	log.Printf("one-shot: waiting for a client to connect (max 120s)...")
	select {
	case <-ctx.Done():
		return nil
	case <-time.After(120 * time.Second):
		log.Printf("one-shot: timed out with no client")
		return nil
	case <-delivered:
		// Keep the server up briefly so the client can confirm receipt.
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
			return nil
		}
	}
}

// runWatchRemote starts a temporary server that writes the first incoming
// push to the local clipboard and exits.
func runWatchRemote(timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	clipboard, err := clip.New()
	if err != nil {
		return err
	}
	defer clipboard.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var once sync.Once
	received := make(chan struct{})
	srv := server.New(cfg, version.Version, func(content clip.Content, from string) {
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
		once.Do(func() { close(received) })
	})
	go func() {
		if err := srv.ListenAndServe(ctx); err != nil {
			log.Printf("ws server: %v", err)
		}
	}()

	if cfg.Connection.Mode == config.ModeDiscover {
		if err := discover.Advertise(ctx, cfg.DeviceName, cfg.Server.Port, cfg.Mdns, cfg.TLS.Enabled); err != nil {
			log.Printf("mdns advertise failed: %v", err)
		}
		go func() {
			if err := discover.Beacon(ctx, cfg.DeviceName, cfg.Server.Port,
				cfg.BroadcastInterval(), cfg.TLS.Enabled); err != nil {
				log.Printf("udp beacon failed: %v", err)
			}
		}()
	} else {
		log.Printf("connection mode %q: not advertising (whitelist-only)", cfg.Connection.Mode)
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

func runCopy(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: clipshare copy <text>")
	}
	clipboard, err := clip.New()
	if err != nil {
		return err
	}
	defer clipboard.Close()
	if err := clipboard.Write(clip.Content{Kind: clip.KindText, Text: strings.Join(args, " ")}); err != nil {
		return err
	}
	fmt.Println("copied")
	return nil
}

func runWatch() error {
	clipboard, err := clip.New()
	if err != nil {
		return err
	}
	defer clipboard.Close()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	w := clip.NewWatcher(clipboard, 300*time.Millisecond, func(c clip.Content) {
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

func runStatus() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/status", cfg.API.Port))
	if err != nil {
		return fmt.Errorf("daemon not running? (%v)", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon error %s", resp.Status)
	}
	var st server.StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return err
	}
	fmt.Printf("device:   %s\n", st.Device)
	fmt.Printf("version:  %s\n", st.Version)
	fmt.Printf("uptime:   %s\n", st.Uptime)
	fmt.Printf("token:    %v\n", st.TokenAuth)
	fmt.Printf("tls:      %v\n", st.TLS)
	fmt.Printf("mode:     %s\n", st.Mode)
	if len(st.Peers) == 0 {
		fmt.Println("peers:    none connected")
	} else {
		fmt.Printf("peers:    %d connected\n", len(st.Peers))
		for _, p := range st.Peers {
			fmt.Printf("  - %s (%s v%s) %s\n", p.Name, p.Platform, p.Version, p.IP)
		}
	}
	return nil
}

func runConfig(args []string) error {
	if len(args) >= 1 && args[0] == "--init" {
		cfg := config.Default()
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("wrote %s\n", config.Path())
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return cfg.Save()
}
