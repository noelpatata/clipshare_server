package clip

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

// Interface abstracts reading/writing the system clipboard.
type linuxClip struct {
	wl bool
}

// New returns a clipboard backend for the current platform.
func New() (Interface, error) {
	if os.Getenv("WAYLAND_DISPLAY") != "" && haveCmd("wl-copy") && haveCmd("wl-paste") {
		return &linuxClip{wl: true}, nil
	}
	if haveCmd("xclip") || haveCmd("xsel") {
		return &linuxClip{wl: false}, nil
	}
	return nil, errors.New("no clipboard tool found (need wl-copy/wl-paste or xclip)")
}

func haveCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func (c *linuxClip) Read() (string, error) {
	if c.wl {
		out, err := exec.Command("wl-paste", "--no-newline").Output()
		if err == nil {
			return string(out), nil
		}
		// wl-paste fails with exit 1 when the clipboard is empty - that is a
		// legitimate state, not an error.
		if haveCmd("xclip") {
			out, xerr := exec.Command("xclip", "-selection", "clipboard", "-o").Output()
			if xerr == nil {
				return string(out), nil
			}
		}
		if haveCmd("xsel") {
			out, xerr := exec.Command("xsel", "--clipboard", "--output").Output()
			if xerr == nil {
				return string(out), nil
			}
		}
		return "", nil
	}
	if haveCmd("xclip") {
		out, err := exec.Command("xclip", "-selection", "clipboard", "-o").Output()
		if err == nil {
			return string(out), nil
		}
	}
	out, err := exec.Command("xsel", "--clipboard", "--output").Output()
	return string(out), err
}

func (c *linuxClip) Write(text string) error {
	if c.wl {
		cmd := exec.Command("wl-copy")
		cmd.Stdin = strings.NewReader(text)
		if cmd.Run() == nil {
			return nil
		}
		if haveCmd("xclip") {
			return xWrite("xclip", []string{"-selection", "clipboard"}, text)
		}
		if haveCmd("xsel") {
			return xWrite("xsel", []string{"--clipboard", "--input"}, text)
		}
		return errors.New("wl-copy failed and no xclip/xsel fallback")
	}
	if haveCmd("xclip") {
		if err := xWrite("xclip", []string{"-selection", "clipboard"}, text); err == nil {
			return nil
		}
	}
	return xWrite("xsel", []string{"--clipboard", "--input"}, text)
}

func xWrite(bin string, args []string, text string) error {
	cmd := exec.Command(bin, args...)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func (c *linuxClip) Close() {}
