package commands

import (
	"context"
	"os/signal"
	"syscall"

	"clipshare/internal/cli/app"
	"clipshare/internal/config"
	"clipshare/internal/version"
)

// daemonCmd runs the persistent daemon.
type daemonCmd struct{}

func (daemonCmd) Run(cfg *config.Config, args []string) error {
	noWatch := len(args) >= 1 && args[0] == "--no-watch"
	a, err := app.New(cfg, version.Version)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return a.Run(ctx, app.RunOptions{NoWatch: noWatch})
}
