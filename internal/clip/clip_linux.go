package clip

import (
	"clipshare/internal/config"
	"clipshare/internal/consts"
)

// linuxClip is the Linux clipboard wrapper. It delegates all work to a
// platform-specific backend selected by backend_linux.go.
type linuxClip struct {
	backend backend
}

// New auto-detects the clipboard backend for the current platform.
func New() (Interface, error) {
	return NewForConfig(nil)
}

// NewForConfig selects a clipboard backend based on cfg, or auto-detects when
// cfg is nil or requests "auto".
func NewForConfig(cfg *config.Config) (Interface, error) {
	b, err := selectBackend(cfg)
	if err != nil {
		return nil, err
	}
	return &linuxClip{backend: b}, nil
}

func (c *linuxClip) Read() (Content, error) {
	if data, mime, ok := c.backend.readImage(); ok {
		return Content{Kind: KindImage, Image: data, Mime: mime}, nil
	}
	return Content{Kind: KindText, Text: c.backend.readText()}, nil
}

func (c *linuxClip) Write(content Content) error {
	if content.Kind == KindImage {
		mime := content.Mime
		if mime == "" {
			mime = consts.DefaultImageMime
		}
		return c.backend.writeImage(content.Image, mime)
	}
	return c.backend.writeText(content.Text)
}

func (c *linuxClip) Close() {}
