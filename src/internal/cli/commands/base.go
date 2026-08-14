package commands

import (
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

// withConfig wraps a ConfigCommand so it loads the config once before running.
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

// Registry maps subcommand names to their implementations. Config-requiring
// commands are wrapped with withConfig() so config.Load() happens once and
// errors are handled in a single place.
var Registry = map[string]Command{
	"daemon": withConfig(daemonCmd{}),
	"send":   withConfig(sendCmd{}),
	"watch":  withConfig(watchCmd{}),
	"status": withConfig(statusCmd{}),
	"config": withConfig(configCmd{}),
	"cert":   certCmd{},
}
