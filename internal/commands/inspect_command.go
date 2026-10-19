package commands

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aaron-vaz/skelly/internal/config"
	"github.com/aaron-vaz/skelly/internal/download"
	"github.com/aaron-vaz/skelly/internal/templates"
	"github.com/aaron-vaz/skelly/internal/view"
)

// inspectInput describes one input a template asks for.
type inspectInput struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Default     any      `json:"default,omitempty"`
	Choices     []string `json:"choices,omitempty"`
	// Required inputs have no default, so they must be given with -set
	Required bool `json:"required"`
	// DependsOn names the inputs whose values the description, default or choices
	// are built from, which are therefore shown here as templates
	DependsOn []string `json:"dependsOn,omitempty"`
}

type inspectExclusion struct {
	Paths []string `json:"paths"`
	// When is left out for paths that are always excluded
	When string `json:"when,omitempty"`
}

type inspectRename struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// inspectResult is the JSON printed by the inspect command.
type inspectResult struct {
	Source      string             `json:"source"`
	HasConfig   bool               `json:"hasConfig"`
	Name        string             `json:"name,omitempty"`
	Description string             `json:"description,omitempty"`
	Inputs      []inspectInput     `json:"inputs"`
	Raw         []string           `json:"raw"`
	Exclude     []inspectExclusion `json:"exclude"`
	Rename      []inspectRename    `json:"rename"`
}

// InspectCommand prints what a template asks for, as JSON, without creating
// anything, so a program can learn which inputs to pass to init.
type InspectCommand struct {
	processor  *templates.Processor
	downloader download.Downloader
	store      config.Store
	ui         view.UI
	source     string
}

// Name returns the CLI identifier for this command.
func (c *InspectCommand) Name() string {
	return "inspect"
}

// Description returns a short summary shown in the help output.
func (c *InspectCommand) Description() string {
	return "Show a template's inputs and settings as JSON"
}

// Init binds the command's flags to the InspectCommand's options.
func (c *InspectCommand) Init(flags *flag.FlagSet) {
	flags.StringVar(&c.source, "src", "", "URL, path or registered alias of the template to inspect (required, may also be given as the first argument)")
}

// Run downloads the template into a scratch directory, reads its config and
// prints a summary as JSON. Nothing is rendered or written outside that directory.
func (c *InspectCommand) Run(ctx context.Context, args []string) error {
	if c.source == "" && len(args) > 0 {
		c.source, args = args[0], args[1:]
	}
	if len(args) > 0 {
		return fmt.Errorf("invalid options: unexpected arguments: %s", strings.Join(args, " "))
	}
	if c.source == "" {
		return errors.New("invalid options: source is required")
	}

	source, _, err := lookupAlias(c.store, c.source)
	if err != nil {
		return err
	}

	workDir, err := os.MkdirTemp("", "skelly-inspect-*")
	if err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	templateDir := filepath.Join(workDir, "template")

	downloadCtx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()
	if err := c.downloader.Get(downloadCtx, source, templateDir); err != nil {
		return fmt.Errorf("failed to download template: %w", err)
	}

	tmpl, err := c.processor.CreateTemplate(workDir + "/template")
	if err != nil {
		return fmt.Errorf("failed to process template: %w", err)
	}

	return renderJSON(c.ui, newInspectResult(source, tmpl))
}

func newInspectResult(source string, tmpl *templates.ProjectTemplate) inspectResult {
	result := inspectResult{
		Source:  config.RedactSource(source),
		Inputs:  []inspectInput{},
		Raw:     []string{},
		Exclude: []inspectExclusion{},
		Rename:  []inspectRename{},
	}
	if tmpl == nil {
		return result
	}

	result.HasConfig = true
	result.Name = tmpl.Name
	result.Description = tmpl.Description
	result.Raw = append(result.Raw, tmpl.Raw...)

	deps := tmpl.InputDependencies()
	for _, name := range sortedNames(tmpl.Inputs) {
		input := tmpl.Inputs[name]
		result.Inputs = append(result.Inputs, inspectInput{
			Name:        name,
			Description: input.Description,
			Default:     input.Default,
			Choices:     input.Choices,
			Required:    input.Default == nil || input.Default == "",
			DependsOn:   deps[name],
		})
	}

	for _, exclusion := range tmpl.Exclude {
		result.Exclude = append(result.Exclude, inspectExclusion{Paths: exclusion.Patterns(), When: exclusion.When})
	}

	for _, rename := range tmpl.Rename {
		result.Rename = append(result.Rename, inspectRename{From: rename.From, To: rename.To})
	}
	return result
}

// renderJSON prints value as indented JSON.
func renderJSON(ui view.UI, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return ui.RenderInfo(string(encoded))
}

// NewInspectCommand constructs an InspectCommand that downloads templates with
// downloader and reads their config with processor.
func NewInspectCommand(
	processor *templates.Processor,
	downloader download.Downloader,
	store config.Store,
	ui view.UI,
) Command {
	return &InspectCommand{
		processor:  processor,
		downloader: downloader,
		store:      store,
		ui:         ui,
	}
}
