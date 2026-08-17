// Package daemon implements the `clipshare daemon` subcommand.
package daemon

import (
	"context"
	"os/signal"
	"syscall"

	"clipshare/src/internal/cli/app"
	"clipshare/src/internal/config"
	"clipshare/src/internal/version"
)

// Command runs the persistent server + clipboard watcher.
type Command struct{}

func (Command) Run(cfg *config.Config, args []string) error {
	noWatch := len(args) >= 1 && args[0] == "--no-watch"
	a, err := app.New(cfg, version.Version)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return a.Run(ctx, app.RunOptions{NoWatch: noWatch})
}
