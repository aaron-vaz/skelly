package commands

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aaron-vaz/skelly/internal/config"
	"github.com/aaron-vaz/skelly/internal/view"
)

// SourcesAddOptions holds the parsed flag inputs for the sources add command.
type SourcesAddOptions struct {
	force bool
}

// SourcesAddCommand saves a template source under an alias, so that
// "skelly init <alias>" can be used instead of the full source.
type SourcesAddCommand struct {
	store   config.Store
	ui      view.UI
	options SourcesAddOptions
}

// Name returns the CLI identifier for this command.
func (c *SourcesAddCommand) Name() string {
	return "add"
}

// Description returns a short summary shown in the help output.
func (c *SourcesAddCommand) Description() string {
	return "Register a template source under an alias: add <alias> <src>"
}

// Init binds the command's flags to the SourcesAddCommand's options.
func (c *SourcesAddCommand) Init(flags *flag.FlagSet) {
	flags.BoolVar(&c.options.force, "force", false, "Replace the source of an alias that is already registered")
}

// Run validates the alias and source and stores them in the user's config.
func (c *SourcesAddCommand) Run(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("invalid options: expected an alias and a source, e.g. skelly sources add go github.com/user/go-template")
	}
	alias, source := args[0], strings.TrimSpace(args[1])

	if err := config.ValidateAlias(alias); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}
	if source == "" {
		return errors.New("invalid options: source is required")
	}

	// A relative local path would point somewhere else once init runs from
	// another directory, so pin it down now
	if _, err := os.Stat(source); err == nil {
		abs, err := filepath.Abs(source)
		if err != nil {
			return fmt.Errorf("failed to resolve source path: %w", err)
		}
		source = abs
	}

	cfg, err := c.store.Load()
	if err != nil {
		return err
	}

	if existing, ok := cfg.Sources[alias]; ok && existing.URL != source && !c.options.force {
		return fmt.Errorf("alias %q is already registered to %s, use -force to replace it", alias, existing.URL)
	}

	if cfg.Sources == nil {
		cfg.Sources = make(map[string]config.Source)
	}
	cfg.Sources[alias] = config.Source{URL: source}

	if err := c.store.Save(cfg); err != nil {
		return err
	}

	return c.ui.RenderInfo(fmt.Sprintf("Registered %s -> %s", alias, source))
}

// NewSourcesAddCommand returns a SourcesAddCommand that saves aliases to store.
func NewSourcesAddCommand(store config.Store, ui view.UI) Command {
	return &SourcesAddCommand{
		store:   store,
		ui:      ui,
		options: SourcesAddOptions{},
	}
}
