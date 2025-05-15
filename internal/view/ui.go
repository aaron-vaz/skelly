package view

import "github.com/aaron-vaz/proj/internal/templates"

type UI interface {
	RenderInputs(inputs map[string]templates.Input) error
	RenderQuestion(question string, options []string) (string, error)
	RenderInfo(message string) error
	RenderError(message string) error
}
