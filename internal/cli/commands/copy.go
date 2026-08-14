package commands

import (
	"fmt"
	"strings"

	"clipshare/internal/clip"
)

// copyCmd writes text to the local clipboard only.
type copyCmd struct{}

func (copyCmd) Run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: clipshare copy <text>")
	}
	clipboard, err := clip.New()
	if err != nil {
		return err
	}
	defer clipboard.Close()
	if err := clipboard.Write(clip.Content{Kind: clip.KindText, Text: strings.Join(args, " ")}); err != nil {
		return err
	}
	fmt.Println("copied")
	return nil
}
