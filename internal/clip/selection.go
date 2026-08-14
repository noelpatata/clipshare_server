package clip

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

// Selection reads the currently selected text (primary selection) from
// Wayland or X11, falling back to the clipboard selection (last Ctrl+C) as a
// last resort. It returns an error when nothing is selected.
func Selection() (string, error) {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if out, err := exec.Command("wl-paste", "--primary").Output(); err == nil {
			if s := strings.TrimSpace(string(out)); s != "" {
				return s, nil
			}
		}
	}
	if out, err := exec.Command("xclip", "-selection", "primary", "-o").Output(); err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			return s, nil
		}
	}
	if out, err := exec.Command("xsel", "--primary", "--output").Output(); err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			return s, nil
		}
	}
	if out, err := exec.Command("wl-paste", "--no-newline").Output(); err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			return s, nil
		}
	}
	return "", errors.New("no text selected (primary selection empty)")
}
