package cli

import (
	"fmt"
	"os"

	"clipshare/src/internal/cli/commands"
	"clipshare/src/internal/config"
	"clipshare/src/internal/version"
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

	cmd, ok := commands.Registry[args[0]]
	if !ok {
		usage()
		return 0
	}

	if err := cmd.Run(args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
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
  clipshare send [<text>]        push text (or your clipboard) to peers.
                                 [--oneshot]: run as transient one-shot server,
                                 independent from daemon
	clipshare watch                print clipboard changes until interrupted
	clipshare watch --remote       temporarily listen (max 120s, --timeout to
                                 change) and write the first incoming push to the
                                 local clipboard, then exit
  
  clipshare status               show daemon status + connected clients
  clipshare config --init        write default config to %s
  clipshare cert                 manage the mTLS private CA and certificates
                                 (init / issue / export / list)
`, version.Version, config.Path())
}
