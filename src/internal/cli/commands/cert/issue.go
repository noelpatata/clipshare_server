package cert

import (
	"fmt"

	"clipshare/src/internal/certs"
)

func runIssue(dir string, args []string) error {
	f, err := parseFlags(args, flagOpts{allowIP: true})
	if err != nil {
		return err
	}
	if err := certs.Issue(dir, f.name, f.kind, f.ips); err != nil {
		return err
	}
	fmt.Printf("issued %s certificate for %q in %s\n", f.kind, f.name, dir)
	return nil
}
