package templates

import "fmt"

type Renderer interface {
	Supports(config ProjectTemplate) bool
	RenderFile(config ProjectTemplate, path string) (string, error)
	RenderFileContents(config ProjectTemplate, path string) error
}

type RendererService struct {
	renderers []Renderer
}

func (rs *RendererService) RenderFile(config ProjectTemplate, path string) (string, error) {
	return rs.applicableRenderer(config).RenderFile(config, path)
}

func (rs *RendererService) RenderFileContents(config ProjectTemplate, path string) error {
	return rs.applicableRenderer(config).RenderFileContents(config, path)
}

func (rs *RendererService) applicableRenderer(config ProjectTemplate) Renderer {
	for _, renderer := range rs.renderers {
		if renderer.Supports(config) {
			return renderer
		}
	}

	// should never happen as a default renderer is always added
	panic(fmt.Sprintf("No renderer found for project config, renderer = %s", config.Renderer))
}

func NewRendererService() *RendererService {
	return &RendererService{
		renderers: []Renderer{
			&StdRenderer{}, // always added last so that a default renderer is always available
		},
	}
}
