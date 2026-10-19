package commands

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
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

// collectInputs fills in the value of every input, in the order they can be
// answered: an input whose description, default or choices use other inputs
// comes after them, and is rendered with their values. Answers given up front
// with -set are used as they are, and the rest are asked for, or, without
// prompting, taken from their defaults. Every problem is reported at once.
func (c *InitCommand) collectInputs(ctx context.Context, tmpl *templates.ProjectTemplate) error {
	layers, err := tmpl.InputLayers()
	if err != nil {
		return err
	}
	deps := tmpl.InputDependencies()

	errs := unknownAnswers(tmpl.Inputs, c.options.answers)
	var missing []string
	blocked := make(map[string]bool)

	for _, layer := range layers {
		pending := make(map[string]templates.Input)

		for _, name := range layer {
			// Without the value of something it uses, an input cannot be rendered,
			// and that problem has already been reported
			if slices.ContainsFunc(deps[name], func(dep string) bool { return blocked[dep] }) {
				blocked[name] = true
				continue
			}

			input, err := c.processor.RenderInput(*tmpl, name)
			if err != nil {
				errs = append(errs, err)
				blocked[name] = true
				continue
			}

			answer, answered := c.options.answers[name]
			if !answered {
				pending[name] = input
				continue
			}
			if err := checkAnswer(name, input, answer); err != nil {
				errs = append(errs, err)
				blocked[name] = true
				continue
			}
			if input.Value, err = typedAnswer(name, input.Default, answer); err != nil {
				errs = append(errs, err)
				blocked[name] = true
				continue
			}
			tmpl.Inputs[name] = input
		}

		if len(pending) == 0 {
			continue
		}

		if c.options.noInput {
			for _, name := range useDefaults(pending) {
				missing = append(missing, name)
				blocked[name] = true
			}
		} else if err := c.ui.RenderInputs(ctx, pending); err != nil {
			return err
		}
		for name, input := range pending {
			// What the user typed is text, but a default of true or 3 is not
			if text, ok := input.Value.(string); ok && !blocked[name] {
				if input.Value, err = typedAnswer(name, input.Default, text); err != nil {
					errs = append(errs, err)
					blocked[name] = true
				}
			}
			tmpl.Inputs[name] = input
		}
	}

	if len(missing) > 0 {
		slices.Sort(missing)
		errs = append(errs, fmt.Errorf("missing required inputs: %s (pass them with -set name=value)", strings.Join(missing, ", ")))
	}
	return errors.Join(errs...)
}

// typedAnswer reads an answer as the type of the input's default, so that
// "-set enabled=false" is the false that a default of false would be, and not
// the text "false", which a template's {{if}} treats as true.
func typedAnswer(name string, def any, answer string) (any, error) {
	switch def.(type) {
	case bool:
		value, err := strconv.ParseBool(answer)
		if err != nil {
			return nil, fmt.Errorf("input %q must be true or false", name)
		}
		return value, nil
	case int, int64, uint64:
		value, err := strconv.Atoi(answer)
		if err != nil {
			return nil, fmt.Errorf("input %q must be a whole number", name)
		}
		return value, nil
	case float64:
		value, err := strconv.ParseFloat(answer, 64)
		if err != nil {
			return nil, fmt.Errorf("input %q must be a number", name)
		}
		return value, nil
	}
	return answer, nil
}

// unknownAnswers reports the answers that name no input of the template.
func unknownAnswers(inputs map[string]templates.Input, answers map[string]string) []error {
	var errs []error
	for _, name := range slices.Sorted(maps.Keys(answers)) {
		if _, ok := inputs[name]; !ok {
			errs = append(errs, fmt.Errorf("unknown input %q (the template has: %s)", name, joinedNames(inputs)))
		}
	}
	return errs
}

// checkAnswer rejects an answer the user could not have typed in at the prompt.
func checkAnswer(name string, input templates.Input, answer string) error {
	if answer == "" {
		return fmt.Errorf("input %q must not be empty", name)
	}
	if len(input.Choices) > 0 && !slices.Contains(input.Choices, answer) {
		return fmt.Errorf("input %q must be one of: %s", name, strings.Join(input.Choices, ", "))
	}
	return nil
}

// useDefaults answers every input with its default, and returns the names of
// the ones that have none, which can only be answered by the user.
func useDefaults(inputs map[string]templates.Input) (missing []string) {
	for _, name := range sortedNames(inputs) {
		input := inputs[name]
		if input.Default == nil || input.Default == "" {
			missing = append(missing, name)
			continue
		}
		input.Value = input.Default
		inputs[name] = input
	}
	return missing
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
