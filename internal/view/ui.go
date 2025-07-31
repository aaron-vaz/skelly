package view

import "github.com/aaron-vaz/proj/internal/templates"

// UI provides methods for rendering user interface elements and handling user interactions
type UI interface {
	// RenderInputs displays and processes a map of input fields to the user
	RenderInputs(inputs map[string]templates.Input) error
	// RenderQuestion presents a question to the user with multiple choice options and returns the selected option
	RenderQuestion(question string, options []string) (string, error)
	// RenderInfo displays an informational message to the user
	RenderInfo(message string) error
	// RenderError displays an error message to the user
	RenderError(message string) error
}
