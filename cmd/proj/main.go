package main

import (
	"os"

	"github.com/aaron-vaz/proj/internal/commands"
)

func main() {
	if err := commands.Invoke(os.Args); err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}

	os.Exit(0)
}
