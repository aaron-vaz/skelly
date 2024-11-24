package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/aaron-vaz/proj/internal/download"
	"github.com/aaron-vaz/proj/internal/templates"
	"gopkg.in/yaml.v2"
)

const (
	templateConfig = ".proj.yml"
)

type InitOptions struct {
	Source      string
	Destination string
}

type InitCommand struct {
	renderer   *templates.RendererService
	downloader download.Downloader
	options    InitOptions
}

func (cmd InitCommand) Execute() error {
	err := cmd.downloader.Get(cmd.options.Source, cmd.options.Destination)
	if err != nil {
		return err
	}

	configBytes, err := os.ReadFile(filepath.Join(cmd.options.Destination, templateConfig))

	// No need to continue if the file doesn't exist
	// Since the repo has been downloaded we can exit
	if errors.Is(err, os.ErrNotExist) {
		fmt.Printf("No %s file found in %s, exiting....\n", templateConfig, cmd.options.Destination)
		return nil
	} else if err != nil {
		return err
	}

	var config templates.ProjectTemplate
	err = yaml.Unmarshal(configBytes, &config)
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

func NewInitCommand(downloader download.Downloader, renderer *templates.RendererService, options InitOptions) InitCommand {
	return InitCommand{
		downloader: downloader,
		renderer:   renderer,
		options:    options,
	}
}
