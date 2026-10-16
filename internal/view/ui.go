package view

import (
	"context"

	"github.com/aaron-vaz/skelly/internal/templates"
)

// UI provides methods for rendering user interface elements and handling user interactions
type UI interface {
	// RenderInputs displays and processes a map of input fields to the user,
	// giving up with ctx.Err() if ctx is cancelled while waiting for an answer
	RenderInputs(ctx context.Context, inputs map[string]templates.Input) error
	// RenderQuestion presents a question to the user with multiple choice options and returns the selected option,
	// giving up with ctx.Err() if ctx is cancelled while waiting for an answer
	RenderQuestion(ctx context.Context, question string, options []string) (string, error)
	// RenderInfo displays an informational message to the user
	RenderInfo(message string) error
	// RenderError displays an error message to the user
	RenderError(message string) error
}
