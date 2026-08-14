package main

import (
	"log"
	"os"

	"clipshare/internal/cli"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("[clipshare] ")
	os.Exit(cli.Main(os.Args[1:]))
}
