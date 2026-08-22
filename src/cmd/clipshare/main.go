package main

import (
	"os"

	"clipshare/src/internal/cli"
	"clipshare/src/internal/log"
)

func main() {
	log.Default()
	os.Exit(cli.Main(os.Args[1:]))
}
