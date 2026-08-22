// Package log provides leveled logging backed by the standard library logger.
// The destination (stderr or a file) and verbosity threshold are configured
// from the [log] section of the TOML config.
package log

import (
	"fmt"
	"io"
	stdlog "log"
	"os"
	"strings"
)

// Level is a verbosity threshold: messages below it are suppressed.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var current = LevelInfo

// ParseLevel maps a config string to a Level. "" means info.
func ParseLevel(s string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return LevelInfo, nil
	case "debug":
		return LevelDebug, nil
	case "warn", "warning":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	default:
		return LevelInfo, fmt.Errorf("unknown log level %q (want debug, info, warn, or error)", s)
	}
}

// Setup configures the global logger. An empty file keeps stderr output; a
// non-empty file is opened append-only and owned by the daemon.
func Setup(level Level, file string) error {
	w := io.Writer(os.Stderr)
	if file != "" {
		f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return fmt.Errorf("open log file: %w", err)
		}
		w = f
	}
	stdlog.SetOutput(w)
	stdlog.SetFlags(stdlog.LstdFlags | stdlog.Lmsgprefix)
	stdlog.SetPrefix("[clipshare] ")
	current = level
	return nil
}

// Default configures the logger with the built-in defaults: info to stderr.
func Default() {
	_ = Setup(LevelInfo, "")
}

func Debugf(format string, args ...any) {
	if current > LevelDebug {
		return
	}
	stdlog.Output(2, fmt.Sprintf(format, args...))
}

func Infof(format string, args ...any) {
	if current > LevelInfo {
		return
	}
	stdlog.Output(2, fmt.Sprintf(format, args...))
}

func Warnf(format string, args ...any) {
	if current > LevelWarn {
		return
	}
	stdlog.Output(2, fmt.Sprintf(format, args...))
}

func Errorf(format string, args ...any) {
	if current > LevelError {
		return
	}
	stdlog.Output(2, fmt.Sprintf(format, args...))
}
