package templates

import "fmt"

// Renderer provides functionality for rendering project templates
type Renderer interface {
	// Supports checks if this renderer can handle the given project template configuration
	Supports(config ProjectTemplate) bool
	// RenderFile processes a template file at the given path and returns the rendered content
	RenderFile(config ProjectTemplate, path string) (string, error)
	// RenderFileContents processes and writes the template contents to the destination
	RenderFileContents(config ProjectTemplate, path string) error
}

type RendererService struct {
	renderers []Renderer
}

func (rs *RendererService) RenderFile(config ProjectTemplate, path string) (string, error) {
	r, err := rs.applicableRenderer(config)
	if err != nil {
		return "", err
	}
	return r.RenderFile(config, path)
}

func (rs *RendererService) RenderFileContents(config ProjectTemplate, path string) error {
	r, err := rs.applicableRenderer(config)
	if err != nil {
		return err
	}
	return r.RenderFileContents(config, path)
}

func (rs *RendererService) applicableRenderer(config ProjectTemplate) (Renderer, error) {
	for _, renderer := range rs.renderers {
		if renderer.Supports(config) {
			return renderer, nil
		}
	}

	return nil, fmt.Errorf("no renderer found for project config, renderer = %s", config.Renderer)
}

func NewRendererService() *RendererService {
	return &RendererService{
		renderers: []Renderer{
			&StdRenderer{}, // always added last so that a default renderer is always available
		},
	}
}
