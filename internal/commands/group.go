package commands

import (
	"context"
	"flag"
	"fmt"
	"slices"
	"strings"

	"github.com/aaron-vaz/skelly/internal/view"
)

// Group is a command made of subcommands, e.g. "skelly sources add". The
// invoker runs the subcommand named after it, so Group itself only runs when
// none was named, or the one named does not exist.
type Group struct {
	name        string
	description string
	subcommands map[string]Command
	ui          view.UI
}

// Name returns the CLI identifier for this command.
func (g *Group) Name() string {
	return g.name
}

// Description returns a short summary shown in the help output.
func (g *Group) Description() string {
	return g.description
}

// Init is a no-op as a group has no flags of its own.
func (g *Group) Init(flags *flag.FlagSet) {}

// Run shows the group's usage, failing if args name an unknown subcommand. The usage goes
// to stderr for that failure, so that nothing but the error is left for a program (such as
// an eval of 'skelly autocomplete <shell>') to read from stdout.
func (g *Group) Run(ctx context.Context, args []string) error {
	if len(args) > 0 {
		_ = g.ui.RenderError(g.Usage())
		return fmt.Errorf("unknown command: %s %s", g.name, args[0])
	}
	return g.ui.RenderInfo(g.Usage())
}

// Subcommand returns the subcommand called name.
func (g *Group) Subcommand(name string) (Command, bool) {
	cmd, ok := g.subcommands[name]
	return cmd, ok
}

// Subcommands returns the subcommands, sorted by name.
func (g *Group) Subcommands() []Command {
	names := make([]string, 0, len(g.subcommands))
	for name := range g.subcommands {
		names = append(names, name)
	}
	slices.Sort(names)

	subcommands := make([]Command, 0, len(names))
	for _, name := range names {
		subcommands = append(subcommands, g.subcommands[name])
	}
	return subcommands
}

// Usage describes the group and lists its subcommands.
func (g *Group) Usage() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Usage: skelly %s COMMAND [OPTIONS]\n\n%s\n\nCOMMANDS:\n", g.name, g.description)

	names := make([]string, 0, len(g.subcommands))
	for name := range g.subcommands {
		names = append(names, name)
	}
	slices.Sort(names)

	for _, name := range names {
		fmt.Fprintf(&b, "    %-12s %s\n", name, g.subcommands[name].Description())
	}

	fmt.Fprintf(&b, "\nRun 'skelly help %s COMMAND' for command-specific help", g.name)
	return b.String()
}

// Resolve follows args down through groups to the command they name. It
// returns that command, its full name such as "sources add", and the args
// left over for it.
func Resolve(cmd Command, args []string) (Command, string, []string) {
	path := cmd.Name()
	for len(args) > 0 {
		group, ok := cmd.(*Group)
		if !ok {
			break
		}
		sub, ok := group.Subcommand(args[0])
		if !ok {
			break
		}
		cmd, path, args = sub, path+" "+sub.Name(), args[1:]
	}
	return cmd, path, args
}

// NewGroup returns a command that dispatches to subcommands.
func NewGroup(name string, description string, ui view.UI, subcommands ...Command) Command {
	group := &Group{
		name:        name,
		description: description,
		subcommands: make(map[string]Command, len(subcommands)),
		ui:          ui,
	}
	for _, cmd := range subcommands {
		group.subcommands[cmd.Name()] = cmd
	}
	return group
}
