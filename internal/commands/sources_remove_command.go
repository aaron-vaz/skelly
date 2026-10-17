package commands

import (
	"context"
	"errors"
	"flag"
	"fmt"

	"github.com/aaron-vaz/skelly/internal/config"
	"github.com/aaron-vaz/skelly/internal/view"
)

// SourcesRemoveCommand removes a template source registered under an alias.
type SourcesRemoveCommand struct {
	store config.Store
	ui    view.UI
}

// Name returns the CLI identifier for this command.
func (c *SourcesRemoveCommand) Name() string {
	return "remove"
}

// Description returns a short summary shown in the help output.
func (c *SourcesRemoveCommand) Description() string {
	return "Remove a registered template alias: remove <alias>"
}

// Init is a no-op as the sources remove command has no flags.
func (c *SourcesRemoveCommand) Init(flags *flag.FlagSet) {}

// Run deletes the alias from the user's config.
func (c *SourcesRemoveCommand) Run(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("invalid options: expected a single alias")
	}
	alias := args[0]

	cfg, err := c.store.Load()
	if err != nil {
		return err
	}

	if _, ok := cfg.Sources[alias]; !ok {
		return fmt.Errorf("alias %q is not registered", alias)
	}
	delete(cfg.Sources, alias)

	if err := c.store.Save(cfg); err != nil {
		return err
	}

	return c.ui.RenderInfo(fmt.Sprintf("Removed %s", alias))
}

// NewSourcesRemoveCommand returns a SourcesRemoveCommand that removes aliases from store.
func NewSourcesRemoveCommand(store config.Store, ui view.UI) Command {
	return &SourcesRemoveCommand{
		store: store,
		ui:    ui,
	}
}
