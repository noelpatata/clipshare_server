package clip

import (
	"fmt"

	"golang.design/x/clipboard"
)

type winClip struct{}

// New returns a clipboard backend for the current platform.
func New() (Interface, error) {
	if err := clipboard.Init(); err != nil {
		return nil, fmt.Errorf("clipboard init: %w", err)
	}
	return &winClip{}, nil
}

func (c *winClip) Read() (Content, error) {
	if img := clipboard.Read(clipboard.FmtImage); len(img) > 0 {
		return Content{Kind: KindImage, Image: img, Mime: "image/png"}, nil
	}
	return Content{Kind: KindText, Text: string(clipboard.Read(clipboard.FmtText))}, nil
}

func (c *winClip) Write(content Content) error {
	if content.Kind == KindImage {
		<-clipboard.Write(clipboard.FmtImage, content.Image)
		return nil
	}
	<-clipboard.Write(clipboard.FmtText, []byte(content.Text))
	return nil
}

func (c *winClip) Close() {}
