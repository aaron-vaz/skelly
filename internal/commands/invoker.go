package commands

import "github.com/aaron-vaz/proj/internal/commands/templates"

var commandsList []Command = []Command{
	templates.InitCommand{},
}

var availableCommands map[string]Command = map[string]Command{}

func init() {
	for _, cmd := range commandsList {
		availableCommands[cmd.Name()] = cmd
	}
}

func Invoke(args []string) error {
	cmd := availableCommands[args[1]]
	return cmd.Execute(args[2:])
}
