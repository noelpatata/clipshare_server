package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"clipshare/src/internal/certs"
	"clipshare/src/internal/config"
	"clipshare/src/internal/consts"
)

// certCmd manages the mTLS private CA and certificates.
type certCmd struct{}

func (certCmd) Run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf(`usage: clipshare cert <init|issue|export|list>

  clipshare cert init                                  create the private CA
  clipshare cert issue --name X --type server|client   issue a certificate
                       [--ip 192.168.1.10,192.168.1.11]  (SAN IPs, servers only)
  clipshare cert export --name X --type server|client  write a PKCS#12 bundle
                       [--out path.p12]
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
