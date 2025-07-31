package commands

// Invoker executes commands with provided arguments
type Invoker interface {
	// Execute runs a command with the given arguments and returns an error if the execution fails
	Execute(args []string) error
}
