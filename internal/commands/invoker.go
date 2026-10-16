package commands

import "context"

// Invoker executes commands with provided arguments
type Invoker interface {
	// Execute runs a command with the given arguments and returns an error if the execution fails
	Execute(ctx context.Context, args []string) error
}
