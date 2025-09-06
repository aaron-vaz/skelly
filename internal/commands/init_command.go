package commands

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/aaron-vaz/skelly/internal/download"
	"github.com/aaron-vaz/skelly/internal/templates"
	"github.com/aaron-vaz/skelly/internal/view"
)

type InitOptions struct {
	source      string
	destination string
}

type InitCommand struct {
	processor  *templates.Processor
	downloader download.Downloader
	ui         view.UI
	options    InitOptions
}

func (c *InitCommand) Name() string {
	return "init"
}

func (c *InitCommand) Description() string {
	return "Initialize a new project from a template"
}

func (c *InitCommand) Init(flags *flag.FlagSet) {
	flags.StringVar(&c.options.source, "src", "", "URL to template that will be used to init the new project (required)")
	flags.StringVar(&c.options.destination, "dst", ".", "Destination dir where the project will be initialised to (Defaults to current directory)")
}

func (c *InitCommand) Run(args []string) error {
	if err := c.validateOptions(); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Check if destination exists and is not empty
	// Ask user if they want to overwrite the destination
	if _, err := os.Stat(c.options.destination); err == nil {
		yesNo, err := c.ui.RenderQuestion(fmt.Sprintf("Destination %s already exists, would you like to overwrite it?", c.options.destination), []string{"y", "n"})
		if err != nil {
			return fmt.Errorf("failed to get user input: %w", err)
		}

		if yesNo != "y" {
			return c.ui.RenderInfo("Exiting....")
		}

		// If yes we can delete the destination
		if err = os.RemoveAll(c.options.destination); err != nil {
			return fmt.Errorf("failed to clean destination directory: %w", err)
		}
	}

	// Download with context
	if err := c.downloader.Get(ctx, c.options.source, c.options.destination); err != nil {
		return fmt.Errorf("failed to download template: %w", err)
	}

	config, err := c.processor.ProcessTemplate(c.options.destination)
	if err != nil {
		return fmt.Errorf("failed to process template: %w", err)
	}

	// If no config found we can exit as we have nothing to process
	if config == nil {
		return c.ui.RenderInfo(fmt.Sprintf("No %s file found in %s, exiting....\n", templates.TemplateConfigName, c.options.destination))
	}

	// Collect user input defined in the config
	if err := c.ui.RenderInputs(config.Inputs); err != nil {
		return fmt.Errorf("failed to collect inputs: %w", err)
	}

	// Process all files in the template
	return c.processor.ApplyTemplate(*config, c.options.destination)
}

func (c *InitCommand) validateOptions() error {
	if c.options.source == "" {
		return errors.New("source is required")
	}

	if c.options.destination == "" {
		return errors.New("destination path is required")
	}

	return nil
}

func NewInitCommand(
	processor *templates.Processor,
	downloader download.Downloader,
	ui view.UI,
) Command[*flag.FlagSet] {
	return &InitCommand{
		processor:  processor,
		downloader: downloader,
		ui:         ui,
		options:    InitOptions{},
	}
}
