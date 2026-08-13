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

func (c *winClip) Read() (string, error) {
	return string(clipboard.Read(clipboard.FmtText)), nil
}

func (c *winClip) Write(text string) error {
	<-clipboard.Write(clipboard.FmtText, []byte(text))
	return nil
}

func (c *winClip) Close() {}
