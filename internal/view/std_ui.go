package view

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"

	"github.com/aaron-vaz/skelly/internal/templates"
)

// ErrNoInput is returned when stdin is closed before the user provided an answer.
var ErrNoInput = errors.New("input closed before a value was provided")

// StdUI is a standard I/O implementation of the UI interface, facilitating
// interaction with the user via the provided streams.
type StdUI struct {
	stdin       *bufio.Scanner
	stdout      io.Writer
	stderr      io.Writer
	lines       chan scannedLine
	startReader sync.Once
}

type scannedLine struct {
	text string
	err  error
}

// RenderInputs interactively prompts the user for all parameters defined in the template config.
func (s *StdUI) RenderInputs(ctx context.Context, inputs map[string]templates.Input) error {
	// No need to render if there are no inputs defined in the config
	// this is not an error, just a no-op
	if len(inputs) == 0 {
		return nil
	}

	_, err := fmt.Fprintf(s.stdout, "Please provide values for inputs defined in %s:\n\n", templates.TemplateConfigName)
	if err != nil {
		return err
	}

	// Sort the keys to ensure a consistent order
	keys := make([]string, 0, len(inputs))
	for k := range inputs {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, name := range keys {
		input := inputs[name]
		if err := s.renderInput(ctx, name, &input); err != nil {
			return err
		}

		inputs[name] = input
	}

	return nil
}

// RenderQuestion poses a question to the user and awaits a formatted string response.
func (s *StdUI) RenderQuestion(ctx context.Context, question string, options []string) (string, error) {
	_, err := fmt.Fprintf(s.stdout, "%s [%s]\n", question, strings.Join(options, "/"))
	if err != nil {
		return "", err
	}

	return s.waitForUserInput(ctx)
}

// RenderInfo writes an informational message to stdout.
func (s *StdUI) RenderInfo(message string) error {
	_, err := fmt.Fprintf(s.stdout, "%s\n", message)
	return err
}

// RenderError writes an error message to stderr.
func (s *StdUI) RenderError(message string) error {
	_, err := fmt.Fprintf(s.stderr, "%s\n", message)
	return err
}

func (s *StdUI) renderInput(ctx context.Context, name string, input *templates.Input) error {
	required := input.Default == nil || input.Default == ""
	shownDefault := input.Default
	if required {
		shownDefault = "*"
	}

	for {
		_, err := fmt.Fprintf(s.stdout, "%s: \n%s: [%v]%s\n", name, input.Description, shownDefault, choicesHint(input.Choices))
		if err != nil {
			return fmt.Errorf("failed to write prompt for '%s': %w", name, err)
		}

		userInput, err := s.waitForUserInput(ctx)
		if err != nil {
			return fmt.Errorf("failed to read user input for '%s': %w", name, err)
		}

		switch {
		case userInput != "" && !validChoice(input.Choices, userInput):
			_ = s.RenderError(fmt.Sprintf("This input '%s' must be one of: %s\n", name, strings.Join(input.Choices, ", ")))
		case userInput != "":
			input.Value = userInput
			return nil
		case !required:
			input.Value = input.Default
			return nil
		}

		_ = s.RenderError(fmt.Sprintf("This input '%s' is required. Please provide a value.\n", name))
	}
}

func choicesHint(choices []string) string {
	if len(choices) == 0 {
		return ""
	}
	return " (one of: " + strings.Join(choices, ", ") + ")"
}

// validChoice reports whether value is allowed; no choices means anything goes.
func validChoice(choices []string, value string) bool {
	return len(choices) == 0 || slices.Contains(choices, value)
}

// waitForUserInput reads stdin on a background goroutine because a blocked
// read cannot be interrupted; a cancelled ctx abandons the read instead.
func (s *StdUI) waitForUserInput(ctx context.Context) (string, error) {
	s.startReader.Do(func() { go s.readLines() })

	var line scannedLine
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case l, ok := <-s.lines:
		if !ok {
			return "", ErrNoInput
		}
		line = l
	}
	if line.err != nil {
		return "", line.err
	}

	// add new line after input
	if _, err := io.WriteString(s.stdout, "\n"); err != nil {
		return "", err
	}

	return strings.TrimSpace(line.text), nil
}

func (s *StdUI) readLines() {
	defer close(s.lines)
	for s.stdin.Scan() {
		s.lines <- scannedLine{text: s.stdin.Text()}
	}
	if err := s.stdin.Err(); err != nil {
		s.lines <- scannedLine{err: err}
	}
}

// NewStdUI returns a ready-to-use StdUI backed by the provided streams.
func NewStdUI(stdin io.Reader, stdout io.Writer, stderr io.Writer) UI {
	return &StdUI{
		stdin:  bufio.NewScanner(stdin),
		stdout: stdout,
		stderr: stderr,
		lines:  make(chan scannedLine),
	}
}
