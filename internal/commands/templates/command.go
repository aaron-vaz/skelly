package templates

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/aaron-vaz/proj/internal/template"
	"github.com/hashicorp/go-getter"
	"gopkg.in/yaml.v2"
)

const (
	templateConfig = ".proj.yml"
)

type InitCommand struct{}

func (cmd InitCommand) Name() string {
	return "init"
}

func (cmd InitCommand) Execute(args []string) error {
	var src string
	var dst string

	sub := flag.NewFlagSet(cmd.Name(), flag.ExitOnError)
	sub.StringVar(&src, "src", "", "URL to template that will be used to init the new project")
	sub.StringVar(&dst, "dst", ".", "Destination dir where the project will be initialised to (Defaults to current directory)")

	if err := sub.Parse(args); err != nil {
		return err
	}

	if err := getter.GetAny(dst, src); err != nil {
		return err
	}

	configBytes, err := os.ReadFile(filepath.Join(dst, templateConfig))

	// No need to continue if the file doesn't exist
	if errors.Is(err, os.ErrNotExist) {
		fmt.Printf("No %s file found in %s, exiting....\n", templateConfig, dst)
		return nil
	} else if err != nil {
		return err
	}

	var config template.ProjectTemplate
	err = yaml.Unmarshal(configBytes, &config)
	if err != nil {
		return err
	}

	// walk through all files in the destination dir
	return filepath.WalkDir(dst, func(path string, d fs.DirEntry, err error) error {
		// a reason for WalkDir to walk a file that doesn't exist is if the file was renamed when reading
		if errors.Is(err, os.ErrNotExist) {
			return nil
			// if path errored straight away return immediately
		} else if err != nil {
			return err
		}

		// check if filename was templated
		tFilePath, err := config.ProcessTemplatedFileName(path)
		if err != nil {
			return err
		}

		// if a directory we don't do anything after the name
		if d.IsDir() {
			return nil
		}

		// template file contents
		return config.ProcessTemplatedFile(tFilePath)
	})
}
