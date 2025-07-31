package templates

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type TemplateProcessor struct {
	renderer *RendererService
}

func (p *TemplateProcessor) ProcessTemplate(destination string) (*ProjectTemplate, error) {
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

func (p *TemplateProcessor) ApplyTemplate(config ProjectTemplate, destination string) error {
	return filepath.WalkDir(destination, func(path string, d fs.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		} else if err != nil {
			return err
		}

		tFilePath, err := p.renderer.RenderFile(config, path)
		if err != nil {
			return fmt.Errorf("failed to render filename: %w", err)
		}

		if d.IsDir() {
			return nil
		}

		return p.renderer.RenderFileContents(config, tFilePath)
	})
}

func NewTemplateProcessor(renderer *RendererService) *TemplateProcessor {
	return &TemplateProcessor{
		renderer: renderer,
	}
}
