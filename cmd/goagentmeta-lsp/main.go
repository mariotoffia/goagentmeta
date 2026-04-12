package main

import (
	"os"

	"github.com/mariotoffia/goagentmeta/internal/lsp"
)

func main() {
	server := lsp.NewServer(os.Stdin, os.Stdout)
	if err := server.Run(); err != nil {
		os.Exit(1)
	}
	os.Exit(server.ExitCode())
}
