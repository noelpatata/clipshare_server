package cert

import (
	"fmt"

	"clipshare/src/internal/certs"
)

func runList(dir string) error {
	info, err := certs.List(dir)
	if err != nil {
		return err
	}
	fmt.Print(info)
	return nil
}
