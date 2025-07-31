package commands

// Command represents an executable command in the application
type Command interface {
	// Execute runs the command and returns an error if the execution fails
	Execute() error
}
