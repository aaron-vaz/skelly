package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/aaron-vaz/proj/internal/download"
	"github.com/aaron-vaz/proj/internal/templates"
	"github.com/aaron-vaz/proj/internal/view"
	"gopkg.in/yaml.v2"
)

type InitOptions struct {
	Source      string
	Destination string
}

type InitCommand struct {
	renderer   *templates.RendererService
	downloader download.Downloader
	ui         view.UI
	options    InitOptions
}

func (cmd InitCommand) Execute() error {
	// Check if destination is not empty
	// As user if they want to overwrite the destination
	if _, err := os.Stat(cmd.options.Destination); err == nil {
		yesNo, err := cmd.ui.RenderQuestion(fmt.Sprintf("Destination %s Already exists, would you like to overwrite it?", cmd.options.Destination), []string{"y", "n"})
		if err != nil {
			return err
		}

		if yesNo != "y" {
			return cmd.ui.RenderInfo("Exiting....")
		}

		// If yes we can delete the destination
		err = os.RemoveAll(cmd.options.Destination)
		if err != nil {
			return err
		}
	}

	err := cmd.downloader.Get(cmd.options.Source, cmd.options.Destination)
	if err != nil {
		return err
	}

	configBytes, err := os.ReadFile(filepath.Join(cmd.options.Destination, templates.TemplateConfigName))

	// No need to continue if the file doesn't exist
	// Since the repo has been downloaded we can exit
	if errors.Is(err, os.ErrNotExist) {
		return cmd.ui.RenderInfo(fmt.Sprintf("No %s file found in %s, exiting....\n", templates.TemplateConfigName, cmd.options.Destination))
	} else if err != nil {
		return err
	}

	var config templates.ProjectTemplate
	err = yaml.Unmarshal(configBytes, &config)
	if err != nil {
		return err
	}

	// Collect user input defined in the config
	err = cmd.ui.RenderInputs(config.Inputs)
	if err != nil {
		return err
	}

	// walk through all files in the destination dir
	return filepath.WalkDir(cmd.options.Destination, func(path string, d fs.DirEntry, err error) error {
		// a reason for WalkDir to walk a file that doesn't exist is if the file was renamed when reading
		if errors.Is(err, os.ErrNotExist) {
			return nil
			// if path errored straight away return immediately
		} else if err != nil {
			return err
		}

		// check if filename was templated
		tFilePath, err := cmd.renderer.RenderFile(config, path)
		if err != nil {
			return err
		}

		// if a directory we don't do anything after the name
		if d.IsDir() {
			return nil
		}

		// template file contents
		return cmd.renderer.RenderFileContents(config, tFilePath)
	})
}

func NewInitCommand(
	downloader download.Downloader,
	renderer *templates.RendererService,
	options InitOptions,
	ui view.UI,
) InitCommand {
	return InitCommand{
		downloader: downloader,
		renderer:   renderer,
		options:    options,
		ui:         ui,
	}
}
