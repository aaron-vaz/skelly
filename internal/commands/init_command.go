package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/aaron-vaz/proj/internal/templates"
	"github.com/hashicorp/go-getter"
	"gopkg.in/yaml.v2"
)

const (
	templateConfig = ".proj.yml"
)

var getters = map[string]getter.Getter{
	"file":  &getter.FileGetter{Copy: true},
	"git":   new(getter.GitGetter),
	"https": &getter.HttpGetter{Netrc: true},
}

type InitCommand struct {
	src string
	dst string

	renderer *templates.RendererService
}

func (cmd InitCommand) Execute() error {
	err := getter.GetAny(cmd.dst, cmd.src, getter.WithGetters(getters))
	if err != nil {
		return err
	}

	configBytes, err := os.ReadFile(filepath.Join(cmd.dst, templateConfig))

	// No need to continue if the file doesn't exist
	// Since the repo has been downloaded we can exit
	if errors.Is(err, os.ErrNotExist) {
		fmt.Printf("No %s file found in %s, exiting....\n", templateConfig, cmd.dst)
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
	return filepath.WalkDir(cmd.dst, func(path string, d fs.DirEntry, err error) error {
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

func NewInitCommand(src string, dst string, renderer *templates.RendererService) InitCommand {
	return InitCommand{
		src:      src,
		dst:      dst,
		renderer: renderer,
	}
}
