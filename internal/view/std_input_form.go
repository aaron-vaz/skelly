package view

import (
	"bufio"
	"fmt"
	"os"

	"github.com/aaron-vaz/proj/internal/templates"
)

type StdInputView struct{}

func (s *StdInputView) Render(inputs map[string]templates.Input) error {
	// No need to render if there are no inputs defined in the config
	// this is not an error, just a no-op
	if len(inputs) == 0 {
		return nil
	}

	os.Stdout.WriteString(fmt.Sprintf("Please provide values for inputs defined in %s:\n", ".proj.yml"))
	os.Stdout.WriteString("\n")

	for name, input := range inputs {
		err := renderView(name, &input)
		if err != nil {
			return err
		}

		inputs[name] = input
	}

	return nil
}

func renderView(name string, input *templates.Input) error {
	// Render the input
	os.Stdout.WriteString(fmt.Sprintf("%s: \n", name))
	os.Stdout.WriteString(fmt.Sprintf("%s: [%s]\n", input.Description, input.Default))

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()

	userInput := scanner.Text()
	if userInput == "" {
		input.Value = input.Default
	} else {
		input.Value = userInput
	}

	if scanner.Err() != nil {
		return scanner.Err()
	}

	return nil
}

func NewStdInputView() InputView {
	return &StdInputView{}
}
