package commands

import (
	"fmt"
	"strings"

	"clipshare/internal/api"
	"clipshare/internal/config"
	"clipshare/internal/consts"
)

// sendCmd pushes text through the running daemon's localhost API.
type sendCmd struct{}

func (sendCmd) Run(cfg *config.Config, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: clipshare send <text>")
	}
	client := api.NewClient(consts.Localhost, cfg.API.Port)
	if err := client.Send(strings.Join(args, " ")); err != nil {
		return err
	}
	fmt.Println("sent")
	return nil
}
