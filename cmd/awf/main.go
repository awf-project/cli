package main

import (
	"os"

	"github.com/vanoix/awf/internal/interfaces/cli"
)

func main() {
	cmd := cli.NewRootCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
