package commands

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/aaron-vaz/skelly/internal/templates"
)

// answerFlag collects repeated "-set name=value" flags.
type answerFlag map[string]string

// String renders the collected answers, as the flag package requires.
func (a *answerFlag) String() string {
	if a == nil {
		return ""
	}
	pairs := make([]string, 0, len(*a))
	for name, value := range *a {
		pairs = append(pairs, name+"="+value)
	}
	slices.Sort(pairs)
	return strings.Join(pairs, ",")
}

// Set records one "name=value" answer; a later answer for the same name wins.
func (a *answerFlag) Set(raw string) error {
	name, value, ok := strings.Cut(raw, "=")
	if !ok || strings.TrimSpace(name) == "" {
		return fmt.Errorf("%q is not in name=value form", raw)
	}
	if *a == nil {
		*a = answerFlag{}
	}
	(*a)[strings.TrimSpace(name)] = value
	return nil
}

// collectInputs fills in the value of every input. Answers given up front with
// -set are used as they are, and the rest are asked for, or, without prompting,
// taken from their defaults.
func (c *InitCommand) collectInputs(ctx context.Context, inputs map[string]templates.Input) error {
	if err := applyAnswers(inputs, c.options.answers); err != nil {
		return err
	}

	pending := make(map[string]templates.Input)
	for name, input := range inputs {
		if _, answered := c.options.answers[name]; !answered {
			pending[name] = input
		}
	}

	if !c.options.noInput {
		if err := c.ui.RenderInputs(ctx, pending); err != nil {
			return err
		}
	} else if err := useDefaults(pending); err != nil {
		return err
	}

	for name, input := range pending {
		inputs[name] = input
	}
	return nil
}

// applyAnswers validates the answers against the template's inputs, reporting
// every problem at once, and stores them as the inputs' values.
func applyAnswers(inputs map[string]templates.Input, answers map[string]string) error {
	names := make([]string, 0, len(answers))
	for name := range answers {
		names = append(names, name)
	}
	slices.Sort(names)

	var errs []error
	for _, name := range names {
		input, ok := inputs[name]
		if !ok {
			errs = append(errs, fmt.Errorf("unknown input %q (the template has: %s)", name, joinedNames(inputs)))
			continue
		}
		if answers[name] == "" {
			errs = append(errs, fmt.Errorf("input %q must not be empty", name))
			continue
		}
		if len(input.Choices) > 0 && !slices.Contains(input.Choices, answers[name]) {
			errs = append(errs, fmt.Errorf("input %q must be one of: %s", name, strings.Join(input.Choices, ", ")))
			continue
		}
		input.Value = answers[name]
		inputs[name] = input
	}
	return errors.Join(errs...)
}

// useDefaults answers every remaining input with its default, and names the
// ones that have none, which can only be answered by the user.
func useDefaults(inputs map[string]templates.Input) error {
	var missing []string
	for _, name := range sortedNames(inputs) {
		input := inputs[name]
		if input.Default == nil || input.Default == "" {
			missing = append(missing, name)
			continue
		}
		input.Value = input.Default
		inputs[name] = input
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required inputs: %s (pass them with -set name=value)", strings.Join(missing, ", "))
	}
	return nil
}

func sortedNames(inputs map[string]templates.Input) []string {
	names := make([]string, 0, len(inputs))
	for name := range inputs {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func joinedNames(inputs map[string]templates.Input) string {
	if len(inputs) == 0 {
		return "no inputs"
	}
	return strings.Join(sortedNames(inputs), ", ")
}
