package commands

import (
	"clipshare/src/internal/cli/commands/cert"
	"clipshare/src/internal/cli/commands/configcmd"
	"clipshare/src/internal/cli/commands/daemon"
	"clipshare/src/internal/cli/commands/send"
	"clipshare/src/internal/cli/commands/status"
	"clipshare/src/internal/cli/commands/watch"
	"clipshare/src/internal/config"
	"clipshare/src/internal/log"
)

// Command is a CLI subcommand that does not need a loaded config.
type Command interface {
	Run(args []string) error
}

// ConfigCommand is a CLI subcommand that needs a loaded config.
type ConfigCommand interface {
	Run(cfg *config.Config, args []string) error
}

// withConfig wraps a ConfigCommand so config.Load() happens once and logging
// is set up before the subcommand runs.
func withConfig(cmd ConfigCommand) Command {
	return &configLoader{cmd: cmd}
}

type configLoader struct {
	cmd ConfigCommand
}

func (w *configLoader) Run(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	lv, err := log.ParseLevel(cfg.Log.Level)
	if err != nil {
		return err
	}
	if err := log.Setup(lv, cfg.Log.File); err != nil {
		return err
	}
	return w.cmd.Run(cfg, args)
}

var Registry = map[string]Command{
	"daemon": withConfig(daemon.Command{}),
	"send":   withConfig(send.Command{}),
	"watch":  withConfig(watch.Command{}),
	"status": withConfig(status.Command{}),
	"config": withConfig(configcmd.Command{}),
	"cert":   cert.Command{},
}
