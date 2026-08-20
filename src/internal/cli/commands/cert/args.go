package cert

import (
	"fmt"
	"path/filepath"
	"strings"

	"clipshare/src/internal/config"
)

type flags struct {
	name string
	kind string
	out  string
	ips  []string
}

type flagOpts struct {
	allowOut bool
	allowIP  bool
}

// certDir is where the private CA and issued certificates live.
func certDir() (string, error) {
	return filepath.Join(filepath.Dir(config.Path()), "certs"), nil
}

// parseFlags parses the shared --name/--type/--out/--ip flags and validates
// them for the subcommand described by opts.
func parseFlags(args []string, opts flagOpts) (flags, error) {
	var f flags
	for i := 0; i < len(args); i++ {
		next := func() (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s requires a value", args[i])
			}
			i++
			return args[i], nil
		}
		var val string
		var err error
		switch args[i] {
		case "--name":
			val, err = next()
			f.name = val
		case "--type":
			val, err = next()
			f.kind = val
		case "--out":
			if !opts.allowOut {
				return f, fmt.Errorf("unknown flag %q", args[i])
			}
			val, err = next()
			f.out = val
		case "--ip":
			if !opts.allowIP {
				return f, fmt.Errorf("unknown flag %q", args[i])
			}
			val, err = next()
			f.ips = strings.Split(val, ",")
		default:
			return f, fmt.Errorf("unknown flag %q", args[i])
		}
		if err != nil {
			return f, err
		}
	}
	if err := validate(f); err != nil {
		return f, err
	}
	return f, nil
}

func validate(f flags) error {
	if f.kind != "server" && f.kind != "client" {
		return fmt.Errorf("--type must be 'server' or 'client'")
	}
	if f.name == "" {
		return fmt.Errorf("--name is required")
	}
	return nil
}
