package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	DeviceName string   `toml:"device_name"`
	Server     Server   `toml:"server"`
	API        API      `toml:"api"`
	Mdns       bool     `toml:"mdns"`
	Broadcast  int      `toml:"broadcast"`
	Watch      int      `toml:"watch"`
	Token      string   `toml:"token"`
	Peers      []string `toml:"peers"`
}

type Server struct {
	Port int `toml:"port"`
}

type API struct {
	Port int `toml:"port"`
}

func Default() *Config {
	host, _ := os.Hostname()
	return &Config{
		DeviceName: host,
		Server:     Server{Port: 40403},
		API:        API{Port: 40405},
		Mdns:       true,
		Broadcast:  5,
		Watch:      300,
		Token:      "",
		Peers:      []string{},
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
	return cfg, nil
}

func (c *Config) Save() error {
	dir := filepath.Dir(Path())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	return writeToml(c)
}

func writeToml(c *Config) error {
	data := fmt.Sprintf(`device_name = %q
mdns = %v
broadcast = %d
watch = %d
token = %q
peers = %s

[server]
port = %d

[api]
port = %d
`, c.DeviceName, c.Mdns, c.Broadcast, c.Watch, c.Token, tomlSlice(c.Peers), c.Server.Port, c.API.Port)

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
