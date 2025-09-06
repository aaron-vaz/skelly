package commands

// Command represents an executable command in the application
type Command[T any] interface {
	// Name returns the name of the command.
	Name() string
	// Description returns a short description of the command.
	Description() string
	// Init initializes the command's flags.
	Init(flags T)
	// Run executes the command.
	Run(args []string) error
}
