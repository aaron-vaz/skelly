package commands

type Invoker interface {
	Execute(args []string) error
}
