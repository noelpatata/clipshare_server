package commands

import (
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"

	qrcode "github.com/skip2/go-qrcode"

	"clipshare/src/internal/certs"
	"clipshare/src/internal/config"
	"clipshare/src/internal/consts"
)

// certCmd manages the mTLS private CA and certificates.
type certCmd struct{}

func (certCmd) Run(args []string) error {
if len(args) < 1 {
		return fmt.Errorf(`usage: clipshare cert <init|issue|export|qr|list>

  clipshare cert init                                  create the private CA
  clipshare cert issue --name X --type server|client   issue a certificate
                       [--ip 192.168.1.10,192.168.1.11]  (SAN IPs, servers only)
  clipshare cert export --name X --type server|client  write a PKCS#12 bundle
                       [--out path.p12]
  clipshare cert qr --name X --type server|client      print a QR the ClipShare
                       [--out path.png]                  app can scan to import
                                                         this device identity
  clipshare cert qr --type ca [--out path.png]         print a QR for the CA
                                                       (import once to trust the
                                                       server)
  clipshare cert list                                  show issued certificates`)
	}
	dir, err := certDir()
	if err != nil {
		return err
	}
	switch args[0] {
	case "init":
		return certs.Init(dir)
	case "issue":
		name, kind, ips, err := parseCertArgs(args[1:])
		if err != nil {
			return err
		}
		if err := certs.Issue(dir, name, kind, ips); err != nil {
			return err
		}
		fmt.Printf("issued %s certificate for %q in %s\n", kind, name, dir)
		return nil
	case "export":
		name, kind, out, err := parseExportArgs(args[1:])
		if err != nil {
			return err
		}
		if out == "" {
			out = filepath.Join(dir, name+"-"+kind+".p12")
		}
		if err := certs.ExportP12(dir, name, kind, out); err != nil {
			return err
		}
		fmt.Printf("wrote %s (import on the phone; PKCS#12 password: %q)\n", out, consts.P12Password)
		return nil
	case "qr":
		name, kind, out, err := parseQrArgs(args[1:])
		if err != nil {
			return err
		}
		var content string
		if kind == "ca" {
			pem, err := certs.QrCaContent(dir)
			if err != nil {
				return err
			}
			content = "clipshare-ca:" + base64.RawStdEncoding.EncodeToString(pem)
		} else {
			content, err = QrContent(dir, name, kind)
			if err != nil {
				return err
			}
		}
		q, err := qrcode.New(content, qrcode.Low)
		if err != nil {
			return err
		}
		if out != "" {
			if err := q.WriteFile(512, out); err != nil {
				return err
			}
			if kind == "ca" {
				fmt.Printf("wrote QR image %s (scan it in the ClipShare app to trust this server's CA)\n", out)
			} else {
				fmt.Printf("wrote QR image %s (scan it in the ClipShare app to import the %s for %q)\n",
					out, kind, name)
			}
			return nil
		}
		fmt.Println(q.ToSmallString(false))
		if kind == "ca" {
			fmt.Println("scan this QR in the ClipShare app to trust this server's CA")
		} else {
			fmt.Printf("scan this QR in the ClipShare app to import the %s for %q\n", kind, name)
		}
		return nil
	case "list":
		info, err := certs.List(dir)
		if err != nil {
			return err
		}
		fmt.Print(info)
		return nil
	default:
		return fmt.Errorf("unknown cert subcommand %q", args[0])
	}
}

// certDir is where the private CA and issued certificates live.
func certDir() (string, error) {
	return filepath.Join(filepath.Dir(config.Path()), "certs"), nil
}

func parseCertArgs(args []string) (name, kind string, ips []string, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			i++
			if i >= len(args) {
				return "", "", nil, fmt.Errorf("--name requires a value")
			}
			name = args[i]
		case "--type":
			i++
			if i >= len(args) {
				return "", "", nil, fmt.Errorf("--type requires a value")
			}
			kind = args[i]
		case "--ip":
			i++
			if i >= len(args) {
				return "", "", nil, fmt.Errorf("--ip requires a value")
			}
			ips = strings.Split(args[i], ",")
		default:
			return "", "", nil, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if name == "" {
		return "", "", nil, fmt.Errorf("--name is required")
	}
	if kind != "server" && kind != "client" {
		return "", "", nil, fmt.Errorf("--type must be 'server' or 'client'")
	}
	return name, kind, ips, nil
}

func parseExportArgs(args []string) (name, kind, out string, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			i++
			if i >= len(args) {
				return "", "", "", fmt.Errorf("--name requires a value")
			}
			name = args[i]
		case "--type":
			i++
			if i >= len(args) {
				return "", "", "", fmt.Errorf("--type requires a value")
			}
			kind = args[i]
		case "--out":
			i++
			if i >= len(args) {
				return "", "", "", fmt.Errorf("--out requires a value")
			}
			out = args[i]
		default:
			return "", "", "", fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if name == "" {
		return "", "", "", fmt.Errorf("--name is required")
	}
	if kind != "server" && kind != "client" {
		return "", "", "", fmt.Errorf("--type must be 'server' or 'client'")
	}
	return name, kind, out, nil
}

// QrContent returns the QR payload encoding a device's certificate identity
// (key + leaf, without the CA) for the ClipShare app to import. The
// "clipshare-p12:" prefix is parsed by the app; the payload is unpadded base64
// of the compact gzipped envelope from certs.QrBytes, which keeps the QR small
// enough to scan reliably. The server CA is shared separately via
// "clipshare-ca:" (cert qr --type ca).
func QrContent(dir, name, kind string) (string, error) {
	qr, err := certs.QrBytes(dir, name, kind)
	if err != nil {
		return "", err
	}
	return "clipshare-p12:" + base64.RawStdEncoding.EncodeToString(qr), nil
}

func parseQrArgs(args []string) (name, kind, out string, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			i++
			if i >= len(args) {
				return "", "", "", fmt.Errorf("--name requires a value")
			}
			name = args[i]
		case "--type":
			i++
			if i >= len(args) {
				return "", "", "", fmt.Errorf("--type requires a value")
			}
			kind = args[i]
		case "--out":
			i++
			if i >= len(args) {
				return "", "", "", fmt.Errorf("--out requires a value")
			}
			out = args[i]
		default:
			return "", "", "", fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if kind == "ca" {
		return name, kind, out, nil
	}
	if kind != "server" && kind != "client" {
		return "", "", "", fmt.Errorf("--type must be 'server', 'client' or 'ca'")
	}
	if name == "" {
		return "", "", "", fmt.Errorf("--name is required")
	}
	return name, kind, out, nil
}
