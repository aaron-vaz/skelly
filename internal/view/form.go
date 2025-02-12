package view

import "github.com/aaron-vaz/proj/internal/templates"

type InputView interface {
	Render(inputs map[string]templates.Input) error
}
