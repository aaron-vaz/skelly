package view

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/aaron-vaz/proj/internal/templates"
)

type StdUI struct {
	stdin  *bufio.Scanner
	stdout *os.File
	stderr *os.File
}

func (s *StdUI) RenderInputs(inputs map[string]templates.Input) error {
	// No need to render if there are no inputs defined in the config
	// this is not an error, just a no-op
	if len(inputs) == 0 {
		return nil
	}

	_, err := s.stdout.WriteString(fmt.Sprintf("Please provide values for inputs defined in %s:\n\n", templates.TemplateConfigName))
	if err != nil {
		return err
	}

	for name, input := range inputs {
		err := s.renderInputs(name, &input)
		if err != nil {
			return err
		}

		inputs[name] = input
	}

	return nil
}

func (s *StdUI) RenderQuestion(question string, options []string) (string, error) {
	_, err := s.stdout.WriteString(fmt.Sprintf("%s? [%s]\n", question, strings.Join(options, "/")))
	if err != nil {
		return "", err
	}

	return s.waitForUserInput()
}

func (s *StdUI) RenderInfo(message string) error {
	_, err := s.stdout.WriteString(fmt.Sprintf("%s\n", message))
	return err
}

func (s *StdUI) RenderError(message string) error {
	_, err := s.stderr.WriteString(fmt.Sprintf("%s\n", message))
	return err
}

func (s *StdUI) renderInputs(name string, input *templates.Input) error {
	_, err := s.stdout.WriteString(fmt.Sprintf("%s: \n%s: [%s]\n", name, input.Description, input.Default))
	if err != nil {
		return err
	}

	userInput, err := s.waitForUserInput()
	if err != nil {
		return err
	}

	if userInput == "" {
		input.Value = input.Default
	} else {
		input.Value = userInput
	}

	return nil
}

func (s *StdUI) waitForUserInput() (string, error) {
	s.stdin.Scan()

	// add new line after input
	_, err := s.stdout.WriteString("\n")
	if err != nil {
		return "", err
	}

	return s.stdin.Text(), s.stdin.Err()
}

func NewStdUI(stdin *os.File, stdout *os.File, stderr *os.File) UI {
	return &StdUI{
		stdin:  bufio.NewScanner(stdin),
		stdout: stdout,
		stderr: stderr,
	}
}
