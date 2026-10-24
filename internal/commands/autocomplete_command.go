package commands

import (
	"context"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/aaron-vaz/skelly/internal/autocomplete"
	"github.com/aaron-vaz/skelly/internal/config"
	"github.com/aaron-vaz/skelly/internal/view"
)

// completionHint says what the flag values and the positional arguments of a command are
// completed with, which the command's own flag set cannot tell.
type completionHint struct {
	flags map[string]autocomplete.Kind
	args  []autocomplete.Kind
}

// completionHints is keyed by the full name of a command, such as "sources add". A
// test checks that every flag named here still exists.
var completionHints = map[string]completionHint{
	"init": {
		flags: map[string]autocomplete.Kind{"src": autocomplete.Source, "dst": autocomplete.Dir},
		args:  []autocomplete.Kind{autocomplete.Source},
	},
	"inspect": {
		flags: map[string]autocomplete.Kind{"src": autocomplete.Source},
		args:  []autocomplete.Kind{autocomplete.Source},
	},
	"sources add":    {args: []autocomplete.Kind{autocomplete.None, autocomplete.File}},
	"sources remove": {args: []autocomplete.Kind{autocomplete.Alias}},
	"help":           {args: []autocomplete.Kind{autocomplete.Command}},
}

// AutocompleteShellCommand prints the completion script of one shell.
type AutocompleteShellCommand struct {
	shell string
	ui    view.UI
}

// Name returns the CLI identifier for this command, which is the name of the shell.
func (c *AutocompleteShellCommand) Name() string {
	return c.shell
}

// Description returns a short summary shown in the help output.
func (c *AutocompleteShellCommand) Description() string {
	return "Print the " + c.shell + " completion script"
}

// Init is a no-op as printing a script has no flags.
func (c *AutocompleteShellCommand) Init(flags *flag.FlagSet) {}

// Run prints the script to stdout, so it can be evaluated by the startup file of the shell.
func (c *AutocompleteShellCommand) Run(ctx context.Context, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(args, " "))
	}
	script, err := autocomplete.Script(c.shell)
	if err != nil {
		return err
	}
	return c.ui.RenderInfo(strings.TrimSuffix(script, "\n"))
}

// AutocompleteCompleteCommand prints the candidates for a command line. The completion
// scripts call it, with the words typed so far after "--".
type AutocompleteCompleteCommand struct {
	commands map[string]Command
	store    config.Store
	ui       view.UI
}

// Name returns the CLI identifier for this command.
func (c *AutocompleteCompleteCommand) Name() string {
	return "complete"
}

// Description returns a short summary shown in the help output.
func (c *AutocompleteCompleteCommand) Description() string {
	return "Print the completions of the words after --, used by the scripts"
}

// Init is a no-op as the command has no flags.
func (c *AutocompleteCompleteCommand) Init(flags *flag.FlagSet) {}

// Run prints one candidate per line and a last line with what else to offer.
func (c *AutocompleteCompleteCommand) Run(ctx context.Context, args []string) error {
	result := autocomplete.Complete(BuildCompletionSpec(c.commands), args, c.aliases)
	return c.ui.RenderInfo(result.String())
}

// aliases lists the registered template aliases. A config that cannot be read means
// there is nothing to suggest, and a completion must never fail loudly.
func (c *AutocompleteCompleteCommand) aliases() []string {
	cfg, err := c.store.Load()
	if err != nil {
		return nil
	}
	aliases := make([]string, 0, len(cfg.Sources))
	for alias := range cfg.Sources {
		// A config edited by hand can hold keys that no alias could be, such as one with a
		// newline, which would be read as more than one candidate
		if config.IsAlias(alias) {
			aliases = append(aliases, alias)
		}
	}
	slices.Sort(aliases)
	return aliases
}

// BuildCompletionSpec describes commands, their flags and their subcommands for the
// completion, read from the commands themselves so that it cannot drift from them.
func BuildCompletionSpec(commands map[string]Command) autocomplete.Spec {
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	slices.Sort(names)

	spec := autocomplete.Spec{}
	for _, name := range names {
		spec.Commands = append(spec.Commands, completionCmd(commands[name], name))
	}
	return spec
}

func completionCmd(cmd Command, path string) autocomplete.Cmd {
	hint := completionHints[path]
	result := autocomplete.Cmd{
		Name:        cmd.Name(),
		Description: cmd.Description(),
		Args:        hint.args,
	}

	flags := flag.NewFlagSet(cmd.Name(), flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	cmd.Init(flags)
	flags.VisitAll(func(f *flag.Flag) {
		boolean, ok := f.Value.(interface{ IsBoolFlag() bool })
		result.Flags = append(result.Flags, autocomplete.Flag{
			Name:       f.Name,
			Usage:      f.Usage,
			TakesValue: !ok || !boolean.IsBoolFlag(),
			Value:      hint.flags[f.Name],
		})
	})

	if group, ok := cmd.(*Group); ok {
		for _, sub := range group.Subcommands() {
			result.Subcommands = append(result.Subcommands, completionCmd(sub, path+" "+sub.Name()))
		}
	}
	return result
}

// NewAutocompleteCommand returns the "autocomplete" group: a script for each shell, and
// the command those scripts call. commands is the full set of commands to complete.
func NewAutocompleteCommand(commands map[string]Command, store config.Store, ui view.UI) Command {
	subcommands := make([]Command, 0, len(autocomplete.Shells())+1)
	for _, shell := range autocomplete.Shells() {
		subcommands = append(subcommands, &AutocompleteShellCommand{shell: shell, ui: ui})
	}
	subcommands = append(subcommands, &AutocompleteCompleteCommand{commands: commands, store: store, ui: ui})

	return NewGroup(
		"autocomplete",
		"Print shell completion scripts for bash, zsh and fish",
		ui,
		subcommands...,
	)
}
