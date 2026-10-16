package commands

import (
	"context"
	"flag"
)

// Command represents an executable command in the application
type Command interface {
	// Name returns the name of the command.
	Name() string
	// Description returns a short description of the command.
	Description() string
	// Init registers the command's flags on the given flag set.
	Init(flags *flag.FlagSet)
	// Run executes the command with the positional arguments left after flag parsing.
	Run(ctx context.Context, args []string) error
}
