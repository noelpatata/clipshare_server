package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	DeviceName    string     `toml:"device_name"`
	Server        Server     `toml:"server"`
	API           API        `toml:"api"`
	Mdns          bool       `toml:"mdns"`
	Broadcast     int        `toml:"broadcast"`
	Watch         int        `toml:"watch"`
	Token         string     `toml:"token"`
	Peers         []string   `toml:"peers"`
	Connection    Connection `toml:"connection"`
	TLS           TLS        `toml:"tls"`
	MaxImageBytes int64      `toml:"max_image_bytes"`
}

type Server struct {
	Port int `toml:"port"`
}

type API struct {
	Port int `toml:"port"`
}

// Connection governs how devices find and trust each other. The same options
// (mode + whitelist) exist on the Android app.
type Connection struct {
	Mode      string           `toml:"mode"` // "discover" | "whitelist"
	Whitelist []WhitelistEntry `toml:"whitelist"`
}

type WhitelistEntry struct {
	Name string `toml:"name"`
	IP   string `toml:"ip"`
}

// TLS enables mutual TLS. CA is the shared trust root; Cert/Key are this
// device's identity (a server cert, SANs covering this device's LAN IPs).
type TLS struct {
	Enabled bool   `toml:"enabled"`
	CA      string `toml:"ca"`
	Cert    string `toml:"cert"`
	Key     string `toml:"key"`
}

const (
	ModeDiscover  = "discover"
	ModeWhitelist = "whitelist"
)

func Default() *Config {
	host, _ := os.Hostname()
	certsDir := filepath.Join(filepath.Dir(Path()), "certs")
	return &Config{
		DeviceName: host,
		Server:     Server{Port: 40403},
		API:        API{Port: 40405},
		Mdns:       true,
		Broadcast:  5,
		Watch:      300,
		Token:      "",
		Peers:      []string{},
		Connection: Connection{Mode: ModeDiscover},
		TLS: TLS{
			Enabled: false,
			CA:      filepath.Join(certsDir, "ca.pem"),
			Cert:    filepath.Join(certsDir, "server.pem"),
			Key:     filepath.Join(certsDir, "server.key"),
		},
		MaxImageBytes: 10 * 1024 * 1024,
	}
}

func Path() string {
	if cfg := os.Getenv("CLIPSHARE_CONFIG"); cfg != "" {
		return cfg
	}
	return filepath.Join(os.Getenv("HOME"), ".config", "clipshare", "config.toml")
}

func Load() (*Config, error) {
	cfg := Default()
	_, err := toml.DecodeFile(Path(), cfg)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if cfg.Watch < 50 {
		cfg.Watch = 300
	}
	if cfg.Connection.Mode == "" {
		cfg.Connection.Mode = ModeDiscover
	}
	if cfg.MaxImageBytes <= 0 {
		cfg.MaxImageBytes = 10 * 1024 * 1024
	}
	return cfg, nil
}

func (c *Config) Save() error {
	dir := filepath.Dir(Path())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	return writeToml(c)
}

// InWhitelist reports whether a device is allowed to connect when in whitelist
// mode. It matches by IP or name (the name each device declares in hello).
func (c *Config) InWhitelist(ip, name string) bool {
	for _, e := range c.Connection.Whitelist {
		if e.IP != "" && e.IP == ip {
			return true
		}
		if e.Name != "" && e.Name == name {
			return true
		}
	}
	return false
}

func writeToml(c *Config) error {
	data := fmt.Sprintf(`device_name = %q
mdns = %v
broadcast = %d
watch = %d
token = %q
peers = %s
max_image_bytes = %d

[connection]
mode = %q
whitelist = %s

[server]
port = %d

[api]
port = %d

[tls]
enabled = %v
ca = %q
cert = %q
key = %q
`, c.DeviceName, c.Mdns, c.Broadcast, c.Watch, c.Token, tomlSlice(c.Peers),
		c.MaxImageBytes, c.Connection.Mode, tomlWhitelist(c.Connection.Whitelist),
		c.Server.Port, c.API.Port,
		c.TLS.Enabled, c.TLS.CA, c.TLS.Cert, c.TLS.Key)

	f, err := os.Create(Path())
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(data); err != nil {
		return err
	}
	return f.Sync()
}

func tomlSlice(s []string) string {
	if len(s) == 0 {
		return "[]"
	}
	out := "["
	for i, v := range s {
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("%q", v)
	}
	return out + "]"
}

func tomlWhitelist(w []WhitelistEntry) string {
	if len(w) == 0 {
		return "[]"
	}
	out := "["
	for i, e := range w {
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("{ name = %q, ip = %q }", e.Name, e.IP)
	}
	return out + "]"
}

func (c *Config) BroadcastInterval() time.Duration {
	if c.Broadcast <= 0 {
		return 5 * time.Second
	}
	return time.Duration(c.Broadcast) * time.Second
}

func (c *Config) WatchInterval() time.Duration {
	if c.Watch <= 0 {
		return 300 * time.Millisecond
	}
	return time.Duration(c.Watch) * time.Millisecond
}
