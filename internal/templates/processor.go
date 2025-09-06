package templates

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"
)

const (
	pathSeparator = string(os.PathSeparator)
)

type Processor struct {
	renderer *RendererService
}

func (p *Processor) ProcessTemplate(destination string) (*ProjectTemplate, error) {
	configBytes, err := os.ReadFile(filepath.Join(destination, TemplateConfigName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var config ProjectTemplate
	if err := yaml.Unmarshal(configBytes, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (p *Processor) ApplyTemplate(config ProjectTemplate, destination string) error {
	// Phase 1: Render file contents in-place
	err := filepath.WalkDir(destination, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories, That will be handled in phase 2
		if d.IsDir() {
			return nil
		}

		// Skip the template config file itself
		if filepath.Base(path) == TemplateConfigName {
			return nil
		}

		return p.renderer.RenderFileContents(config, path)
	})
	if err != nil {
		return fmt.Errorf("error rendering file contents: %w", err)
	}

	// Phase 2: Rename files and directories from the bottom up
	pathsToRename := make([]string, 0)
	err = filepath.WalkDir(destination, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		pathsToRename = append(pathsToRename, path)
		return nil
	})
	if err != nil {
		return err
	}

	// Sort paths by depth, deepest first, to rename files before their parent directories
	sort.Slice(pathsToRename, func(i, j int) bool {
		return strings.Count(pathsToRename[i], pathSeparator) > strings.Count(pathsToRename[j], pathSeparator)
	})

	for _, path := range pathsToRename {
		// Skip the template config file itself
		if filepath.Base(path) == TemplateConfigName {
			continue
		}
		if _, err := p.renderer.RenderFile(config, path); err != nil {
			// It's possible the file was already moved as part of a parent directory rename, so ignore "not exist" errors
			if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("error renaming path %s: %w", path, err)
			}
		}
	}

	// Finally, delete the template config file
	return os.Remove(filepath.Join(destination, TemplateConfigName))
}

func NewTemplateProcessor(renderer *RendererService) *Processor {
	return &Processor{
		renderer: renderer,
	}
}
