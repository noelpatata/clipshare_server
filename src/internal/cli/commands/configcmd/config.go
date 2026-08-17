// Package configcmd implements the `clipshare config` subcommand.
package configcmd

import (
	"fmt"

	"clipshare/src/internal/config"
)

// Command writes or re-saves the config file.
type Command struct{}

func (Command) Run(cfg *config.Config, args []string) error {
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
