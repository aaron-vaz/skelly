package commands

import (
	"context"
	"flag"
	"fmt"
	"slices"
	"strings"

	"github.com/aaron-vaz/skelly/internal/view"
)

// HelpCommand handles the display of usage instructions and command descriptions.
type HelpCommand struct {
	commands map[string]Command
	ui       view.UI
}

// Name returns the CLI identifier for this command.
func (c *HelpCommand) Name() string {
	return "help"
}

// Description returns a short summary shown in the help output.
func (c *HelpCommand) Description() string {
	return "Show command help"
}

// Init is a no-op as the help command has no flags.
func (c *HelpCommand) Init(flags *flag.FlagSet) {}

// Run prints the application help structure or the specific help documentation
// for a requested sub-command.
func (c *HelpCommand) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return c.ui.RenderInfo(c.usage())
	}

	cmdName := args[0]
	cmd, ok := c.commands[cmdName]
	if !ok {
		_ = c.ui.RenderInfo(c.usage())
		return fmt.Errorf("unknown command: %s", cmdName)
	}

	var defaults strings.Builder
	flags := flag.NewFlagSet(cmd.Name(), flag.ContinueOnError)
	flags.SetOutput(&defaults)
	cmd.Init(flags)
	flags.PrintDefaults()

	return c.ui.RenderInfo(fmt.Sprintf("Usage: skelly %s [options]\n\n%s\n\nOPTIONS:\n%s", cmd.Name(), cmd.Description(), defaults.String()))
}

func (c *HelpCommand) usage() string {
	var b strings.Builder
	b.WriteString("USAGE:\n")
	b.WriteString("    skelly COMMAND [OPTIONS]\n\n")
	b.WriteString("COMMANDS:\n")

	// Map iteration order is random, so sort for stable output
	names := make([]string, 0, len(c.commands))
	for name := range c.commands {
		names = append(names, name)
	}
	slices.Sort(names)

	for _, name := range names {
		fmt.Fprintf(&b, "    %-11s %s\n", name, c.commands[name].Description())
	}

	b.WriteString("\nRun 'skelly help COMMAND' for command-specific help")
	return b.String()
}

// NewHelpCommand returns a configured HelpCommand that understands the
// full map of available commands in order to print their documentation.
func NewHelpCommand(commands map[string]Command, ui view.UI) Command {
	return &HelpCommand{
		commands: commands,
		ui:       ui,
	}
}
