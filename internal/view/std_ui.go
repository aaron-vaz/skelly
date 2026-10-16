package view

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/aaron-vaz/skelly/internal/templates"
)

// ErrNoInput is returned when stdin is closed before the user provided an answer.
var ErrNoInput = errors.New("input closed before a value was provided")

// StdUI is a standard I/O implementation of the UI interface, facilitating
// interaction with the user via the provided streams.
type StdUI struct {
	stdin  *bufio.Scanner
	stdout io.Writer
	stderr io.Writer
}

// RenderInputs interactively prompts the user for all parameters defined in the template config.
func (s *StdUI) RenderInputs(inputs map[string]templates.Input) error {
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
		if err := s.renderInput(name, &input); err != nil {
			return err
		}

		inputs[name] = input
	}

	return nil
}

// RenderQuestion poses a question to the user and awaits a formatted string response.
func (s *StdUI) RenderQuestion(question string, options []string) (string, error) {
	_, err := fmt.Fprintf(s.stdout, "%s [%s]\n", question, strings.Join(options, "/"))
	if err != nil {
		return "", err
	}

	return s.waitForUserInput()
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

func (s *StdUI) renderInput(name string, input *templates.Input) error {
	required := input.Default == nil || input.Default == ""
	shownDefault := input.Default
	if required {
		shownDefault = "*"
	}

	for {
		_, err := fmt.Fprintf(s.stdout, "%s: \n%s: [%v]\n", name, input.Description, shownDefault)
		if err != nil {
			return fmt.Errorf("failed to write prompt for '%s': %w", name, err)
		}

		userInput, err := s.waitForUserInput()
		if err != nil {
			return fmt.Errorf("failed to read user input for '%s': %w", name, err)
		}

		switch {
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

func (s *StdUI) waitForUserInput() (string, error) {
	if !s.stdin.Scan() {
		if err := s.stdin.Err(); err != nil {
			return "", err
		}
		return "", ErrNoInput
	}

	// add new line after input
	if _, err := io.WriteString(s.stdout, "\n"); err != nil {
		return "", err
	}

	return strings.TrimSpace(s.stdin.Text()), nil
}

// NewStdUI returns a ready-to-use StdUI backed by the provided streams.
func NewStdUI(stdin io.Reader, stdout io.Writer, stderr io.Writer) UI {
	return &StdUI{
		stdin:  bufio.NewScanner(stdin),
		stdout: stdout,
		stderr: stderr,
	}
}
