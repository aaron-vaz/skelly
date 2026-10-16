package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/aaron-vaz/skelly/internal/templates"
)

type mockUI struct {
	infoMsgs []string
}

func (m *mockUI) RenderInputs(ctx context.Context, inputs map[string]templates.Input) error {
	return nil
}
func (m *mockUI) RenderQuestion(ctx context.Context, question string, options []string) (string, error) {
	return "", nil
}
func (m *mockUI) RenderInfo(message string) error {
	m.infoMsgs = append(m.infoMsgs, message)
	return nil
}
func (m *mockUI) RenderError(message string) error { return nil }

func TestVersionCommand_Run(t *testing.T) {
	// Given
	ui := &mockUI{
		infoMsgs: make([]string, 0),
	}
	cmd := NewVersionCommand(ui)

	// When
	err := cmd.Run(context.Background(), nil)

	// Then
	if err != nil {
		t.Errorf("Run() error = %v, wantErr false", err)
	}

	// And
	if len(ui.infoMsgs) != 1 {
		t.Errorf("expected 1 info message, got %d", len(ui.infoMsgs))
	} else if !strings.Contains(ui.infoMsgs[0], "skelly version dev") {
		t.Errorf("expected version output, got %s", ui.infoMsgs[0])
	}
}
