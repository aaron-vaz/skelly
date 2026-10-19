package templates

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Input defines interactive user fields typically consumed via CLI dialog.
type Input struct {
	Description string `yaml:"description"`
	Default     any    `yaml:"default,omitempty"`
	// Choices restricts the accepted values; empty means any value is accepted.
	Choices []string `yaml:"choices,omitempty"`
	Value   any      `yaml:"-"`
}

// Exclusion removes paths from the generated project when its condition holds,
// e.g. to leave out the server code of a template when the user picked "cli".
type Exclusion struct {
	// Path is a glob matched like Raw patterns: against a file's relative path,
	// its name, or any parent directory.
	Path string `yaml:"path,omitempty"`
	// Paths lists several globs that share the same condition.
	Paths []string `yaml:"paths,omitempty"`
	// When is rendered as a template and must produce "true" or "false".
	When string `yaml:"when"`
}

// Patterns returns every glob of the exclusion, whether it was written as
// path or as paths.
func (e Exclusion) Patterns() []string {
	if e.Path == "" {
		return e.Paths
	}
	return append([]string{e.Path}, e.Paths...)
}

// label names the exclusion in error messages.
func (e Exclusion) label() string {
	return strings.Join(e.Patterns(), ", ")
}

// Renaming moves a path of the template to another place once the project has
// been rendered, e.g. to ship "github" and generate ".github", so that files
// meant for generated projects, such as CI workflows, stay inert in the
// template's own repository.
type Renaming struct {
	// From is the path in the template, as it is named there. Exclude and Raw
	// patterns also refer to the template's own names, before any renaming.
	From string `yaml:"from"`
	// To is the path in the generated project. It must not exist yet; missing
	// parent directories are created.
	To string `yaml:"to"`
}

// ProjectTemplate delineates the configuration manifest defining how a template generates.
type ProjectTemplate struct {
	Name        string           `yaml:"name"`
	Description string           `yaml:"description"`
	Renderer    string           `yaml:"renderer,omitempty"`
	Inputs      map[string]Input `yaml:"inputs"`
	Env         map[string]any   `yaml:"env"`
	// Raw lists glob patterns of files whose names and contents are copied without
	// being rendered, e.g. CI workflows or Helm charts that use {{ }} themselves.
	// A pattern may match a file's relative path, its name, or any parent directory.
	Raw []string `yaml:"raw,omitempty"`
	// Exclude lists paths that are dropped from the generated project, before
	// rendering, whenever their condition renders as true. Unlike Raw it also
	// applies to raw files.
	Exclude []Exclusion `yaml:"exclude,omitempty"`
	// Rename moves paths after everything has been rendered, in the order listed.
	Rename []Renaming `yaml:"rename,omitempty"`
}

// Validate reports every problem in the config at once, so a template author
// can fix them in one pass instead of discovering them one render at a time.
func (t ProjectTemplate) Validate() error {
	var errs []error

	names := make([]string, 0, len(t.Inputs))
	for name := range t.Inputs {
		names = append(names, name)
	}
	slices.Sort(names)

	for _, name := range names {
		if strings.TrimSpace(name) == "" {
			errs = append(errs, errors.New("input name must not be blank"))
			continue
		}
		if !isScalar(t.Inputs[name].Default) {
			errs = append(errs, fmt.Errorf("input %q: default must be a string, number or boolean, got %T", name, t.Inputs[name].Default))
		}
		if err := validateChoices(name, t.Inputs[name]); err != nil {
			errs = append(errs, err)
		}
	}

	for _, pattern := range t.Raw {
		if !doublestar.ValidatePattern(pattern) {
			errs = append(errs, fmt.Errorf("invalid raw pattern %q: %w", pattern, doublestar.ErrBadPattern))
		}
	}

	for _, exclusion := range t.Exclude {
		patterns := exclusion.Patterns()
		if len(patterns) == 0 {
			errs = append(errs, errors.New("exclude path must not be blank"))
		}
		for _, pattern := range patterns {
			if strings.TrimSpace(pattern) == "" {
				errs = append(errs, errors.New("exclude path must not be blank"))
			} else if !doublestar.ValidatePattern(pattern) {
				errs = append(errs, fmt.Errorf("invalid exclude pattern %q: %w", pattern, doublestar.ErrBadPattern))
			}
		}
		if strings.TrimSpace(exclusion.When) == "" {
			errs = append(errs, fmt.Errorf("exclude %q: when must not be blank", exclusion.label()))
		}
	}

	errs = append(errs, t.validateRenames()...)

	return errors.Join(errs...)
}

// validateRenames checks that every rename stays inside the project and that
// no two renames end up at the same place.
func (t ProjectTemplate) validateRenames() []error {
	var errs []error
	targets := make(map[string]bool, len(t.Rename))

	for _, rename := range t.Rename {
		from, to := cleanPath(rename.From), cleanPath(rename.To)
		switch {
		case strings.TrimSpace(rename.From) == "" || strings.TrimSpace(rename.To) == "":
			errs = append(errs, errors.New("rename needs both from and to"))
			continue
		case !filepath.IsLocal(from) || !filepath.IsLocal(to):
			errs = append(errs, fmt.Errorf("rename %q to %q: paths must be relative and stay inside the project", rename.From, rename.To))
			continue
		case from == to:
			errs = append(errs, fmt.Errorf("rename %q: from and to are the same", rename.From))
			continue
		case from == TemplateConfigName:
			errs = append(errs, fmt.Errorf("rename %q: the config file is never copied", rename.From))
			continue
		}

		if targets[to] {
			errs = append(errs, fmt.Errorf("rename %q: more than one rename ends up at %q", rename.From, rename.To))
		}
		targets[to] = true
	}
	return errs
}

// cleanPath normalises a slash separated path from the config.
func cleanPath(p string) string {
	return path.Clean(strings.TrimSpace(p))
}

// validateChoices rejects a default that the user could not have typed in.
func validateChoices(name string, input Input) error {
	if len(input.Choices) == 0 || input.Default == nil || input.Default == "" {
		return nil
	}
	if !slices.Contains(input.Choices, fmt.Sprint(input.Default)) {
		return fmt.Errorf("input %q: default %v is not one of its choices (%s)", name, input.Default, strings.Join(input.Choices, ", "))
	}
	return nil
}

// isScalar reports whether v can be typed in at a prompt; defaults stand in for
// user answers, so lists and maps can never be valid ones.
func isScalar(v any) bool {
	switch v.(type) {
	case nil, string, bool, int, int64, uint64, float64:
		return true
	default:
		return false
	}
}
