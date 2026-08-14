package cli

import (
	"fmt"
	"os"
	"time"

	"clipshare/internal/config"
	"clipshare/internal/version"
)

// Main dispatches the clipshare command line and returns the process exit code.
func Main(args []string) int {
	if len(args) < 1 {
		usage()
		return 0
	}
	if args[0] == "--version" || args[0] == "-v" || args[0] == "version" {
		printVersion()
		return 0
	}
	var err error
	switch args[0] {
	case "daemon":
		noWatch := len(args) >= 2 && args[1] == "--no-watch"
		err = runDaemon(noWatch)
	case "send":
		err = runSend(args[1:])
	case "share":
		err = runShare(args[1:])
	case "copy":
		err = runCopy(args[1:])
	case "watch":
		err = runWatchCmd(args[1:])
	case "status":
		err = runStatus()
	case "config":
		err = runConfig(args[1:])
	case "cert":
		err = runCert(args[1:])
	default:
		usage()
		return 0
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

// runWatchCmd dispatches "watch" and "watch --remote".
func runWatchCmd(args []string) error {
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
		return runWatchRemote(timeout)
	}
	if len(args) > 0 {
		return fmt.Errorf("usage: clipshare watch [--remote [--timeout <dur>]]")
	}
	return runWatch()
}

func printVersion() {
	fmt.Println(version.Version)
}

func usage() {
	fmt.Fprintf(os.Stderr, `clipshare %s - LAN clipboard sharing daemon

Usage:
  clipshare version              print the current version
  clipshare --version, -v        print the current version
  clipshare daemon [--no-watch]  run server + clipboard watcher (foreground).
                                 --no-watch: never broadcast the local clipboard;
                                 only receive and write remote content.
  clipshare share [<text>]       push text (or your primary selection) to peers.
                                 Starts a transient daemon if none is running.
  clipshare send <text>          push text via a running daemon to connected peers
  clipshare watch                print clipboard changes until interrupted
  clipshare watch --remote       temporarily listen (max 120s, --timeout to
                                 change) and write the first incoming push to the
                                 local clipboard, then exit
  clipshare copy <text>          set the local clipboard only
  clipshare status               show daemon status + connected clients
  clipshare config --init        write default config to %s
  clipshare cert                 manage the mTLS private CA and certificates
                                 (init / issue / export / list)
`, version.Version, config.Path())
}
