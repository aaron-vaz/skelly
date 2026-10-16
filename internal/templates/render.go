package templates

import "fmt"

// Renderer provides functionality for rendering project templates
type Renderer interface {
	// Supports checks if this renderer can handle the given project template configuration
	Supports(config ProjectTemplate) bool
	// Render executes text as a template identified by name and returns the result
	Render(config ProjectTemplate, name string, text string) (string, error)
}

// RendererService abstracts available individual renderers and determines
// the appropriate execution strategy.
type RendererService struct {
	renderers []Renderer
}

// Render delegates to the first Renderer that supports the given config.
func (rs *RendererService) Render(config ProjectTemplate, name string, text string) (string, error) {
	r, err := rs.applicableRenderer(config)
	if err != nil {
		return "", err
	}
	return r.Render(config, name, text)
}

func (rs *RendererService) applicableRenderer(config ProjectTemplate) (Renderer, error) {
	for _, renderer := range rs.renderers {
		if renderer.Supports(config) {
			return renderer, nil
		}
	}

	return nil, fmt.Errorf("no renderer found for project config, renderer = %s", config.Renderer)
}

// NewRendererService constructs a core rendering service populated
// by default with the reliable internal StdRenderer.
func NewRendererService() *RendererService {
	return &RendererService{
		renderers: []Renderer{
			&StdRenderer{}, // always added last so that a default renderer is always available
		},
	}
}
