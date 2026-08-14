package commands

import (
	"fmt"

	"clipshare/internal/config"
)

// configCmd initializes or re-saves the config file.
type configCmd struct{}

func (configCmd) Run(cfg *config.Config, args []string) error {
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
