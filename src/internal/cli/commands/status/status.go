// Package status implements the `clipshare status` subcommand.
package status

import (
	"fmt"

	"clipshare/src/internal/api"
	"clipshare/src/internal/config"
	"clipshare/src/internal/consts"
)

// Command shows the running daemon's status and connected clients.
type Command struct{}

func (Command) Run(cfg *config.Config, args []string) error {
	client := api.NewClient(consts.Localhost, cfg.API.Port)
	st, err := client.Status()
	if err != nil {
		return fmt.Errorf("daemon not running? (%v)", err)
	}
	fmt.Printf("device:   %s\n", st.Device)
	fmt.Printf("version:  %s\n", st.Version)
	fmt.Printf("uptime:   %s\n", st.Uptime)
	fmt.Printf("token:    %v\n", st.TokenAuth)
	fmt.Printf("tls:      %v\n", st.TLS)
	fmt.Printf("mode:     %s\n", st.Mode)
	if len(st.Peers) == 0 {
		fmt.Println("peers:    none connected")
	} else {
		fmt.Printf("peers:    %d connected\n", len(st.Peers))
		for _, p := range st.Peers {
			fmt.Printf("  - %s (%s v%s) %s\n", p.Name, p.Platform, p.Version, p.IP)
		}
	}
	return nil
}
