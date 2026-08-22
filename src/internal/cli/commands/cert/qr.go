package cert

import (
	"fmt"

	qrcode "github.com/skip2/go-qrcode"

	"clipshare/src/internal/certs"
)

func runQr(dir string, args []string) error {
	f, err := parseFlags(args, flagOpts{allowOut: true})
	if err != nil {
		return err
	}
	content, err := certs.QrContent(dir, f.name, f.kind)
	if err != nil {
		return err
	}
	q, err := qrcode.New(content, qrcode.Low)
	if err != nil {
		return err
	}
	if f.out != "" {
		if err := q.WriteFile(512, f.out); err != nil {
			return err
		}
		fmt.Printf("wrote QR image %s (%s)\n", f.out, qrHint(f.kind, f.name))
		return nil
	}
	fmt.Println(q.ToSmallString(false))
	fmt.Println("scan this QR in the ClipShare app " + qrHint(f.kind, f.name))
	return nil
}

func qrHint(kind, name string) string {
	return fmt.Sprintf("to import the %s for %q", kind, name)
}
