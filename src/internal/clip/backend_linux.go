package clip

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"clipshare/src/internal/config"
	"clipshare/src/internal/consts"
)

// backend abstracts the underlying clipboard mechanism (Wayland or X11).
type backend interface {
	readText() string
	readImage() ([]byte, string, bool)
	writeText(text string) error
	writeImage(data []byte, mime string) error
}

// selectBackend picks a backend based on cfg, or auto-detects when cfg is nil
// or requests "auto".
func selectBackend(cfg *config.Config) (backend, error) {
	forced := ""
	if cfg != nil {
		forced = cfg.Clipboard.Backend
	}

	switch forced {
	case "wayland":
		if haveCmd("wl-copy") && haveCmd("wl-paste") {
			return waylandBackend{}, nil
		}
		return nil, errors.New("clipboard backend 'wayland' requested but wl-copy/wl-paste not found")
	case "xclip":
		if haveCmd("xclip") {
			return x11Backend{bin: "xclip"}, nil
		}
		return nil, errors.New("clipboard backend 'xclip' requested but xclip not found")
	case "xsel":
		if haveCmd("xsel") {
			return x11Backend{bin: "xsel"}, nil
		}
		return nil, errors.New("clipboard backend 'xsel' requested but xsel not found")
	}

	// Auto-detect.
	if os.Getenv("WAYLAND_DISPLAY") != "" && haveCmd("wl-copy") && haveCmd("wl-paste") {
		return waylandBackend{}, nil
	}
	if haveCmd("xclip") {
		return x11Backend{bin: "xclip"}, nil
	}
	if haveCmd("xsel") {
		return x11Backend{bin: "xsel"}, nil
	}
	return nil, errors.New("no clipboard tool found (need wl-copy/wl-paste or xclip)")
}

func haveCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// waylandBackend uses wl-copy/wl-paste on Wayland, with xclip/xsel fallbacks.
type waylandBackend struct{}

func (waylandBackend) readText() string {
	if out, err := runText("wl-paste", "--no-newline"); err == nil {
		return out
	}
	if out, err := runText("xclip", "-selection", "clipboard", "-o"); err == nil {
		return out
	}
	if out, err := runText("xsel", "--clipboard", "--output"); err == nil {
		return out
	}
	return ""
}

func (waylandBackend) readImage() ([]byte, string, bool) {
	types, err := runText("wl-paste", "--list-types")
	if err != nil {
		return nil, "", false
	}
	mime, ok := chooseImageType(parseTypes(types))
	if !ok {
		return nil, "", false
	}
	data, err := runBin("wl-paste", []string{"--type", mime})
	if err != nil || len(data) == 0 {
		return nil, "", false
	}
	return data, mime, true
}

func (waylandBackend) writeText(text string) error {
	if err := runStdin("wl-copy", nil, text); err == nil {
		return nil
	}
	if err := runStdin("xclip", []string{"-selection", "clipboard"}, text); err == nil {
		return nil
	}
	if err := runStdin("xsel", []string{"--clipboard", "--input"}, text); err == nil {
		return nil
	}
	return errors.New("wl-copy failed and no xclip/xsel fallback")
}

func (waylandBackend) writeImage(data []byte, mime string) error {
	if err := runStdin("wl-copy", []string{"--type", mime}, string(data)); err == nil {
		return nil
	}
	if err := runStdin("xclip", []string{"-selection", "clipboard", "-t", mime}, string(data)); err == nil {
		return nil
	}
	return errors.New("wl-copy failed and no xclip fallback")
}

// x11Backend uses xclip or xsel on X11.
type x11Backend struct {
	bin string
}

func (b x11Backend) readText() string {
	if b.bin == "xclip" {
		if out, err := runText("xclip", "-selection", "clipboard", "-o"); err == nil {
			return out
		}
		return ""
	}
	if out, err := runText("xsel", "--clipboard", "--output"); err == nil {
		return out
	}
	return ""
}

func (b x11Backend) readImage() ([]byte, string, bool) {
	if b.bin != "xclip" {
		// xsel cannot read arbitrary binary targets reliably.
		return nil, "", false
	}
	out, err := runText("xclip", "-selection", "clipboard", "-t", "TARGETS", "-o")
	if err != nil {
		return nil, "", false
	}
	mime, ok := chooseImageType(parseTypes(out))
	if !ok {
		return nil, "", false
	}
	data, err := runBin("xclip", []string{"-selection", "clipboard", "-t", mime, "-o"})
	if err != nil || len(data) == 0 {
		return nil, "", false
	}
	return data, mime, true
}

func (b x11Backend) writeText(text string) error {
	if b.bin == "xclip" {
		return runStdin("xclip", []string{"-selection", "clipboard"}, text)
	}
	return runStdin("xsel", []string{"--clipboard", "--input"}, text)
}

func (b x11Backend) writeImage(data []byte, mime string) error {
	if b.bin != "xclip" {
		return errors.New("xsel cannot write images")
	}
	return runStdin("xclip", []string{"-selection", "clipboard", "-t", mime}, string(data))
}

// runText runs a command and returns its stdout as a string.
func runText(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", withStderr(err, stderr.String())
	}
	return string(out), nil
}

// runBin runs a command and returns its stdout as raw bytes.
func runBin(name string, args []string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, withStderr(err, stderr.String())
	}
	return out, nil
}

// runStdin runs a command with the given text on stdin.
func runStdin(name string, args []string, text string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(text)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return withStderr(err, stderr.String())
	}
	return nil
}

// withStderr appends the command's captured stderr to err so the caller sees
// the real failure (e.g. wl-copy's message) instead of a bare "exit status 1".
func withStderr(err error, stderr string) error {
	if msg := strings.TrimSpace(stderr); msg != "" {
		return fmt.Errorf("%v: %s", err, msg)
	}
	return err
}

// parseTypes splits command output into non-empty MIME type strings.
func parseTypes(out string) []string {
	var types []string
	for _, l := range strings.Split(out, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			types = append(types, t)
		}
	}
	return types
}

// chooseImageType returns the preferred image MIME from the offered types.
func chooseImageType(types []string) (string, bool) {
	preferred := ""
	for _, t := range types {
		if t == consts.DefaultImageMime {
			return consts.DefaultImageMime, true
		}
		if strings.HasPrefix(t, "image/") && preferred == "" {
			preferred = t
		}
	}
	if preferred != "" {
		return preferred, true
	}
	return "", false
}
