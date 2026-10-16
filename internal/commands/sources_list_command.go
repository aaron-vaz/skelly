package commands

import (
	"context"
	"flag"
	"fmt"
	"slices"
	"strings"

	"github.com/aaron-vaz/skelly/internal/config"
	"github.com/aaron-vaz/skelly/internal/view"
)

// SourcesListCommand shows the template sources registered under an alias.
type SourcesListCommand struct {
	store config.Store
	ui    view.UI
}

// Name returns the CLI identifier for this command.
func (c *SourcesListCommand) Name() string {
	return "list"
}

// Description returns a short summary shown in the help output.
func (c *SourcesListCommand) Description() string {
	return "List registered template aliases"
}

// Init is a no-op as the sources list command has no flags.
func (c *SourcesListCommand) Init(flags *flag.FlagSet) {}

// Run prints every registered alias with its source, sorted by alias.
func (c *SourcesListCommand) Run(ctx context.Context, args []string) error {
	cfg, err := c.store.Load()
	if err != nil {
		return err
	}

	if len(cfg.Sources) == 0 {
		return c.ui.RenderInfo("No templates registered, add one with: skelly sources add <alias> <src>")
	}

	aliases := make([]string, 0, len(cfg.Sources))
	width := 0
	for alias := range cfg.Sources {
		aliases = append(aliases, alias)
		width = max(width, len(alias))
	}
	slices.Sort(aliases)

	var b strings.Builder
	for i, alias := range aliases {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%-*s  %s", width, alias, cfg.Sources[alias].URL)
	}
	return c.ui.RenderInfo(b.String())
}

// NewSourcesListCommand returns a SourcesListCommand that reads aliases from store.
func NewSourcesListCommand(store config.Store, ui view.UI) Command {
	return &SourcesListCommand{
		store: store,
		ui:    ui,
	}
}
