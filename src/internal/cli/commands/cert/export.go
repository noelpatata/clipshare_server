package cert

import (
	"fmt"
	"path/filepath"

	"clipshare/src/internal/certs"
	"clipshare/src/internal/consts"
)

func runExport(dir string, args []string) error {
	f, err := parseFlags(args, flagOpts{allowOut: true})
	if err != nil {
		return err
	}
	if f.out == "" {
		f.out = filepath.Join(dir, f.name+"-"+f.kind+".p12")
	}
	if err := certs.ExportP12(dir, f.name, f.kind, f.out); err != nil {
		return err
	}
	fmt.Printf("wrote %s (import on the phone; PKCS#12 password: %q)\n", f.out, consts.P12Password)
	return nil
}
