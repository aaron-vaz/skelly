package commands

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/aaron-vaz/skelly/internal/download"
	"github.com/aaron-vaz/skelly/internal/templates"
	"github.com/aaron-vaz/skelly/internal/view"
)

type InitOptions struct {
	Source      string
	Destination string
}

type InitCommand struct {
	processor  *templates.TemplateProcessor
	downloader download.Downloader
	ui         view.UI
	options    InitOptions
}

func (cmd InitCommand) validateOptions() error {
	if cmd.options.Source == "" {
		return errors.New("source URL is required")
	}

	// Validate source URL format
	_, err := url.Parse(cmd.options.Source)
	if err != nil {
		return fmt.Errorf("invalid source URL format: %w", err)
	}

	if cmd.options.Destination == "" {
		return errors.New("destination path is required")
	}

	// Validate and normalize destination path
	dest := filepath.Clean(cmd.options.Destination)
	if !filepath.IsAbs(dest) {
		cmd.options.Destination, err = filepath.Abs(dest)
		if err != nil {
			return fmt.Errorf("invalid destination path: %w", err)
		}
	}

	return nil
}

func (cmd InitCommand) Execute() error {
	if err := cmd.validateOptions(); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Check if destination exists and is not empty
	// Ask user if they want to overwrite the destination
	if _, err := os.Stat(cmd.options.Destination); err == nil {
		yesNo, err := cmd.ui.RenderQuestion(fmt.Sprintf("Destination %s already exists, would you like to overwrite it?", cmd.options.Destination), []string{"y", "n"})
		if err != nil {
			return fmt.Errorf("failed to get user input: %w", err)
		}

		if yesNo != "y" {
			return cmd.ui.RenderInfo("Exiting....")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// If yes we can delete the destination
			if err = os.RemoveAll(cmd.options.Destination); err != nil {
				return fmt.Errorf("failed to clean destination directory: %w", err)
			}
		}
	}

	// Download with context
	if err := cmd.downloader.Get(cmd.options.Source, cmd.options.Destination); err != nil {
		return fmt.Errorf("failed to download template: %w", err)
	}

	config, err := cmd.processor.ProcessTemplate(cmd.options.Destination)
	if err != nil {
		return fmt.Errorf("failed to process template: %w", err)
	}

	if config == nil {
		return cmd.ui.RenderInfo(fmt.Sprintf("No %s file found in %s, exiting....\n", templates.TemplateConfigName, cmd.options.Destination))
	}

	// Collect user input defined in the config
	if err := cmd.ui.RenderInputs(config.Inputs); err != nil {
		return fmt.Errorf("failed to collect inputs: %w", err)
	}

	// Process all files in the template
	return cmd.processor.ApplyTemplate(*config, cmd.options.Destination)
}

func NewInitCommand(
	processor *templates.TemplateProcessor,
	downloader download.Downloader,
	options InitOptions,
	ui view.UI,
) InitCommand {
	return InitCommand{
		processor:  processor,
		downloader: downloader,
		options:    options,
		ui:         ui,
	}
}
