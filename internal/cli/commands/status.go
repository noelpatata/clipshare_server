package commands

import (
	"fmt"

	"clipshare/internal/api"
	"clipshare/internal/config"
	"clipshare/internal/consts"
)

// statusCmd queries the running daemon's status.
type statusCmd struct{}

func (statusCmd) Run(cfg *config.Config, args []string) error {
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
