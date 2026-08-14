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

func (c *linuxClip) Read() (Content, error) {
	if c.wl {
		if mime, ok := imageType(c.wlTypes()); ok {
			if out, err := exec.Command("wl-paste", "--type", mime).Output(); err == nil && len(out) > 0 {
				return Content{Kind: KindImage, Image: out, Mime: mime}, nil
			}
		}
		return Content{Kind: KindText, Text: c.readTextWL()}, nil
	}
	if mime, ok := imageType(c.xTargets()); ok {
		if out, err := exec.Command("xclip", "-selection", "clipboard", "-t", mime, "-o").Output(); err == nil && len(out) > 0 {
			return Content{Kind: KindImage, Image: out, Mime: mime}, nil
		}
	}
	return Content{Kind: KindText, Text: c.readTextX()}, nil
}

// wlTypes lists the MIME types currently offered by the Wayland clipboard.
func (c *linuxClip) wlTypes() []string {
	out, err := exec.Command("wl-paste", "--list-types").Output()
	if err != nil {
		return nil
	}
	var types []string
	for _, l := range strings.Split(string(out), "\n") {
		if t := strings.TrimSpace(l); t != "" {
			types = append(types, t)
		}
	}
	return types
}

// xTargets lists the targets offered by the X11 clipboard.
func (c *linuxClip) xTargets() []string {
	out, err := exec.Command("xclip", "-selection", "clipboard", "-t", "TARGETS", "-o").Output()
	if err != nil {
		return nil
	}
	var types []string
	for _, l := range strings.Split(string(out), "\n") {
		if t := strings.TrimSpace(l); t != "" {
			types = append(types, t)
		}
	}
	return types
}

// imageType returns the preferred image MIME offered by the clipboard, or
// ("", false) when the clipboard holds no image.
func imageType(types []string) (string, bool) {
	preferred := ""
	for _, t := range types {
		if t == "image/png" {
			return "image/png", true
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

func (c *linuxClip) readTextWL() string {
	out, err := exec.Command("wl-paste", "--no-newline").Output()
	if err == nil {
		return string(out)
	}
	// wl-paste fails with exit 1 when the clipboard is empty - that is a
	// legitimate state, not an error.
	if haveCmd("xclip") {
		if out, xerr := exec.Command("xclip", "-selection", "clipboard", "-o").Output(); xerr == nil {
			return string(out)
		}
	}
	if haveCmd("xsel") {
		if out, xerr := exec.Command("xsel", "--clipboard", "--output").Output(); xerr == nil {
			return string(out)
		}
	}
	return ""
}

func (c *linuxClip) readTextX() string {
	if haveCmd("xclip") {
		if out, err := exec.Command("xclip", "-selection", "clipboard", "-o").Output(); err == nil {
			return string(out)
		}
	}
	out, err := exec.Command("xsel", "--clipboard", "--output").Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func (c *linuxClip) Write(content Content) error {
	if content.Kind == KindImage {
		return c.writeImage(content)
	}
	return c.writeText(content.Text)
}

func (c *linuxClip) writeImage(content Content) error {
	mime := content.Mime
	if mime == "" {
		mime = "image/png"
	}
	if c.wl {
		cmd := exec.Command("wl-copy", "--type", mime)
		cmd.Stdin = strings.NewReader(string(content.Image))
		if cmd.Run() == nil {
			return nil
		}
		if haveCmd("xclip") {
			return xWriteBin("xclip", []string{"-selection", "clipboard", "-t", mime}, content.Image)
		}
		return errors.New("wl-copy failed and no xclip fallback")
	}
	if haveCmd("xclip") {
		if err := xWriteBin("xclip", []string{"-selection", "clipboard", "-t", mime}, content.Image); err == nil {
			return nil
		}
	}
	return errors.New("no image-capable clipboard tool found (need wl-copy or xclip)")
}

func (c *linuxClip) writeText(text string) error {
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

func xWriteBin(bin string, args []string, data []byte) error {
	cmd := exec.Command(bin, args...)
	cmd.Stdin = strings.NewReader(string(data))
	return cmd.Run()
}

func (c *linuxClip) Close() {}
