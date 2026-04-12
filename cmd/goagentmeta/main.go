package main

import (
	"os"

	"github.com/mariotoffia/goagentmeta/internal/adapter/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
