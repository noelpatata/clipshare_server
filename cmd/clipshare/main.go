package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"clipshare/internal/clip"
	"clipshare/internal/config"
	"clipshare/internal/discover"
	"clipshare/internal/server"
)

const version = "0.1.0"

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("[clipshare] ")

	if len(os.Args) < 2 {
		usage()
		return
	}
	var err error
	switch os.Args[1] {
	case "daemon":
		noWatch := len(os.Args) >= 3 && os.Args[2] == "--no-watch"
		err = runDaemon(noWatch)
	case "send":
		err = runSend(os.Args[2:])
	case "share":
		err = runShare(os.Args[2:])
	case "copy":
		err = runCopy(os.Args[2:])
	case "watch":
		err = runWatch()
	case "status":
		err = runStatus()
	case "config":
		err = runConfig(os.Args[2:])
	default:
		usage()
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `clipshare %s - LAN clipboard sharing daemon

Usage:
  clipshare daemon [--no-watch]  run server + clipboard watcher (foreground).
                                 --no-watch: never broadcast the local clipboard;
                                 only receive and write remote content.
  clipshare share [<text>]       push text (or your primary selection) to peers.
                                 Starts a transient daemon if none is running.
  clipshare send <text>          push text via a running daemon to connected peers
  clipshare copy <text>          set the local clipboard only
  clipshare watch                print clipboard changes until interrupted
  clipshare status               show daemon status + connected clients
  clipshare config --init        write default config to %s
`, version, config.Path())
}

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
	srv := server.New(cfg, version, func(text, from string) {
		if text == "" {
			return
		}
		log.Printf("remote clipboard from %q -> writing to local", from)
		if watcher != nil {
			watcher.LocalWrite(text)
		} else {
			clipboard.Write(text)
		}
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if !noWatch {
		watcher = clip.NewWatcher(clipboard, cfg.WatchInterval(), func(text string) {
			log.Printf("local clipboard changed")
			srv.BroadcastLocal(text, cfg.DeviceName)
		})
		go watcher.Run(ctx)
	} else {
		log.Printf("clipboard watching disabled (--no-watch); only receiving")
	}

	go srv.ListenAndServe(ctx)
	go srv.RunPeers(ctx, version, func(text, from string) {
		if watcher != nil {
			watcher.LocalWrite(text)
		} else {
			clipboard.Write(text)
		}
	})

	if err := discover.Advertise(ctx, cfg.DeviceName, cfg.Server.Port, cfg.Mdns); err != nil {
		log.Printf("mdns advertise failed: %v", err)
	}
	go func() {
		if err := discover.Beacon(ctx, cfg.DeviceName, cfg.Server.Port,
			cfg.BroadcastInterval(), cfg.Token); err != nil {
			log.Printf("udp beacon failed: %v", err)
		}
	}()

	go server.ListenAPI(ctx, srv, version)

	log.Printf("clipshare %s running as %q (ws :%d, api :%d)",
		version, cfg.DeviceName, cfg.Server.Port, cfg.API.Port)
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
		sel, err := readSelection()
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
			return runOneShot(cfg, text)
		}
		return err
	}
	fmt.Println("sent")
	return nil
}

// runOneShot runs a short-lived server, broadcasts text once a peer connects,
// waits briefly, then exits.
func runOneShot(cfg *config.Config, text string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := server.New(cfg, version, func(t, from string) {
		log.Printf("received from %q during one-shot (ignored)", from)
	})
	delivered := make(chan struct{})
	var once sync.Once
	srv.SetConnectHook(func() {
		once.Do(func() {
			srv.BroadcastLocal(text, cfg.DeviceName)
			log.Printf("one-shot: delivered to client")
			close(delivered)
		})
	})
	go srv.ListenAndServe(ctx)

	if err := discover.Advertise(ctx, cfg.DeviceName, cfg.Server.Port, cfg.Mdns); err != nil {
		log.Printf("mdns advertise failed: %v", err)
	}
	go func() {
		if err := discover.Beacon(ctx, cfg.DeviceName, cfg.Server.Port,
			cfg.BroadcastInterval(), cfg.Token); err != nil {
			log.Printf("udp beacon failed: %v", err)
		}
	}()

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

// readSelection grabs the currently selected text (primary selection) from
// Wayland or X11, falling back to the clipboard selection as a last resort.
func readSelection() (string, error) {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if out, err := exec.Command("wl-paste", "--primary").Output(); err == nil {
			if s := strings.TrimSpace(string(out)); s != "" {
				return s, nil
			}
		}
	}
	if out, err := exec.Command("xclip", "-selection", "primary", "-o").Output(); err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			return s, nil
		}
	}
	if out, err := exec.Command("xsel", "--primary", "--output").Output(); err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			return s, nil
		}
	}
	// Last resort: the clipboard selection (content of the last Ctrl+C).
	if out, err := exec.Command("wl-paste", "--no-newline").Output(); err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			return s, nil
		}
	}
	return "", errors.New("no text selected (primary selection empty)")
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
	if err := clipboard.Write(strings.Join(args, " ")); err != nil {
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
	w := clip.NewWatcher(clipboard, 300*time.Millisecond, func(text string) {
		fmt.Println("---- clipboard ----")
		fmt.Println(text)
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
	if len(st.Peers) == 0 {
		fmt.Println("peers:    none connected")
	} else {
		fmt.Printf("peers:    %d connected\n", len(st.Peers))
		for _, p := range st.Peers {
			fmt.Printf("  - %s (%s v%s)\n", p.Name, p.Platform, p.Version)
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
