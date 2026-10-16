package commands

import (
	"github.com/aaron-vaz/skelly/internal/config"
	"github.com/aaron-vaz/skelly/internal/view"
)

// NewSourcesCommand returns the "sources" group, which manages the template
// sources registered under an alias in the user's config.
func NewSourcesCommand(store config.Store, ui view.UI) Command {
	return NewGroup(
		"sources",
		"Manage template sources registered under an alias",
		ui,
		NewSourcesAddCommand(store, ui),
		NewSourcesListCommand(store, ui),
		NewSourcesRemoveCommand(store, ui),
	)
}
