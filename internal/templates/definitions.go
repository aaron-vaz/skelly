package templates

import (
	"errors"
	"fmt"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"text/template"
	"text/template/parse"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

// Input defines interactive user fields typically consumed via CLI dialog.
//
// Description, Default and Choices may be templates that use the values of other
// inputs, e.g. a default of "github.com/{{.Inputs.Owner.Value}}/{{.Inputs.Name.Value}}".
// Such an input is asked after the ones it uses.
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
	// When is rendered as a template and must produce "true" or "false". Without
	// it the paths are always excluded, as for the template's own files.
	When string `yaml:"when,omitempty"`
}

// UnmarshalYAML reads an exclusion and rejects keys it does not know. Without a
// when an exclusion always applies, so a misspelt when ("wen") must not be
// ignored and quietly turn a conditional exclusion into an unconditional one.
func (e *Exclusion) UnmarshalYAML(node *yaml.Node) error {
	if err := checkKeys(node, "exclude", "path", "paths", "when"); err != nil {
		return err
	}
	type plain Exclusion
	if err := node.Decode((*plain)(e)); err != nil {
		return err
	}
	return checkNotBlank(node, "exclude", map[string]bool{"path": e.Path == "", "paths": len(e.Paths) == 0})
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
	// From is the path as it is named once the template has been rendered, so
	// a name that contains {{ }} is written out as the user's inputs render it.
	// It is itself rendered as a template, which lets it say the same thing as
	// the file's own name does. Exclude and Raw patterns refer to the
	// template's own names, before any rendering.
	From string `yaml:"from"`
	// To is the path in the generated project, also rendered as a template. It
	// must not exist yet; missing parent directories are created.
	To string `yaml:"to"`
	// When is rendered as a template and must produce "true" or "false". The
	// rename is skipped when it is false; empty means always.
	When string `yaml:"when,omitempty"`
}

// UnmarshalYAML reads a rename and rejects keys it does not know, for the same
// reason as for an exclusion.
func (r *Renaming) UnmarshalYAML(node *yaml.Node) error {
	if err := checkKeys(node, "rename", "from", "to", "when"); err != nil {
		return err
	}
	type plain Renaming
	return node.Decode((*plain)(r))
}

// checkKeys rejects a mapping that has a key other than the allowed ones.
func checkKeys(node *yaml.Node, what string, allowed ...string) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: each %s entry must be a mapping with the keys %s", node.Line, what, strings.Join(allowed, ", "))
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		if !slices.Contains(allowed, key.Value) {
			return fmt.Errorf("line %d: unknown key %q in %s (allowed: %s)", key.Line, key.Value, what, strings.Join(allowed, ", "))
		}
	}
	return nil
}

// checkNotBlank rejects a key that is written but empty, such as path: "" or
// paths: [], which would otherwise read the same as leaving it out.
func checkNotBlank(node *yaml.Node, what string, blank map[string]bool) error {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if key := node.Content[i].Value; blank[key] {
			return fmt.Errorf("line %d: %s %s must not be blank", node.Content[i].Line, what, key)
		}
	}
	return nil
}

// ProjectTemplate delineates the configuration manifest defining how a template generates.
type ProjectTemplate struct {
	Name        string           `yaml:"name"`
	Description string           `yaml:"description"`
	Renderer    string           `yaml:"renderer,omitempty"`
	Inputs      map[string]Input `yaml:"inputs"`
	// Env holds values for templates to use as .Env.Name. A string value is
	// itself rendered as a template that can use the inputs, so a value derived
	// from them, like a module path, is defined once. It cannot refer to other
	// Env values.
	Env map[string]any `yaml:"env"`
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
		errs = append(errs, t.validateInputTemplates(name)...)
		if !inputIsTemplated(t.Inputs[name]) {
			if err := validateChoices(name, t.Inputs[name]); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if _, err := t.InputLayers(); err != nil {
		errs = append(errs, err)
	}

	for _, pattern := range t.Raw {
		if err := checkPattern(pattern); err != nil {
			errs = append(errs, fmt.Errorf("invalid raw pattern %q: %w", pattern, err))
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
			} else if err := checkPattern(pattern); err != nil {
				errs = append(errs, fmt.Errorf("invalid exclude pattern %q: %w", pattern, err))
			}
		}
		if err := checkTemplate(exclusion.When); err != nil {
			errs = append(errs, fmt.Errorf("exclude %q: when: %w", exclusion.label(), err))
		}
	}

	errs = append(errs, t.validateEnv()...)
	errs = append(errs, t.validateRenames()...)

	return errors.Join(errs...)
}

// validateInputTemplates reports templates in an input that are malformed or
// that use an input which does not exist.
func (t ProjectTemplate) validateInputTemplates(name string) []error {
	var errs []error
	for _, field := range inputTemplates(t.Inputs[name]) {
		refs, err := inputRefs(field.text)
		if err != nil {
			errs = append(errs, fmt.Errorf("input %q: %s: %w", name, field.name, err))
			continue
		}
		for _, ref := range refs {
			if _, ok := t.Inputs[ref]; !ok {
				errs = append(errs, fmt.Errorf("input %q: %s uses unknown input %q", name, field.name, ref))
			}
		}
	}
	return errs
}

// InputDependencies returns, for every input that has any, the sorted names of
// the other inputs its description, default and choices are built from.
func (t ProjectTemplate) InputDependencies() map[string][]string {
	deps := make(map[string][]string)
	for name, input := range t.Inputs {
		seen := map[string]bool{}
		for _, field := range inputTemplates(input) {
			refs, _ := inputRefs(field.text)
			for _, ref := range refs {
				if _, ok := t.Inputs[ref]; ok {
					seen[ref] = true
				}
			}
		}
		if len(seen) > 0 {
			deps[name] = slices.Sorted(maps.Keys(seen))
		}
	}
	return deps
}

// InputLayers groups the inputs in the order they can be answered: every input
// comes after the ones it uses, and the inputs of a layer are independent of
// each other. Inputs that use each other in a cycle are an error.
func (t ProjectTemplate) InputLayers() ([][]string, error) {
	deps := t.InputDependencies()
	remaining := make(map[string]bool, len(t.Inputs))
	for name := range t.Inputs {
		remaining[name] = true
	}

	var layers [][]string
	for len(remaining) > 0 {
		var layer []string
		for name := range remaining {
			ready := true
			for _, dep := range deps[name] {
				if remaining[dep] {
					ready = false
					break
				}
			}
			if ready {
				layer = append(layer, name)
			}
		}

		if len(layer) == 0 {
			return nil, fmt.Errorf("inputs use each other in a cycle, involving: %s", strings.Join(slices.Sorted(maps.Keys(remaining)), ", "))
		}

		slices.Sort(layer)
		for _, name := range layer {
			delete(remaining, name)
		}
		layers = append(layers, layer)
	}
	return layers, nil
}

type inputField struct {
	name string
	text string
}

// inputTemplates lists the parts of an input that may be templates.
func inputTemplates(input Input) []inputField {
	fields := []inputField{{"description", input.Description}}
	if text, ok := input.Default.(string); ok {
		fields = append(fields, inputField{"default", text})
	}
	for _, choice := range input.Choices {
		fields = append(fields, inputField{"choices", choice})
	}
	return fields
}

func inputIsTemplated(input Input) bool {
	for _, field := range inputTemplates(input) {
		if isTemplated(field.text) {
			return true
		}
	}
	return false
}

// inputRefs returns the names of the inputs a template reads, found by walking
// the parsed template for .Inputs.<name>. A template that reaches the inputs any
// other way, such as with index or with, cannot be ordered and is an error.
func inputRefs(text string) ([]string, error) {
	if !isTemplated(text) {
		return nil, nil
	}

	parsed, err := template.New("").Option("missingkey=error").Parse(text)
	if err != nil {
		return nil, err
	}

	var refs []string
	opaque := false
	walkTemplate(parsed.Root, func(ident []string) {
		// A variable starts with the variable's name, so $.Inputs.A is $, Inputs, A
		if len(ident) > 0 && ident[0] == "$" {
			ident = ident[1:]
		}
		switch {
		case len(ident) == 0 || ident[0] != "Inputs":
		case len(ident) == 1:
			opaque = true
		default:
			refs = append(refs, ident[1])
		}
	})
	if opaque {
		return nil, errors.New("uses .Inputs without naming an input, so the order the inputs are asked in cannot be worked out; write .Inputs.<name>.Value")
	}
	return refs, nil
}

// walkTemplate calls visit with the path of every field or variable reference
// in the parse tree.
func walkTemplate(node parse.Node, visit func(ident []string)) {
	switch n := node.(type) {
	case *parse.ListNode:
		if n == nil {
			return
		}
		for _, child := range n.Nodes {
			walkTemplate(child, visit)
		}
	case *parse.ActionNode:
		walkTemplate(n.Pipe, visit)
	case *parse.PipeNode:
		if n == nil {
			return
		}
		for _, cmd := range n.Cmds {
			walkTemplate(cmd, visit)
		}
	case *parse.CommandNode:
		for _, arg := range n.Args {
			walkTemplate(arg, visit)
		}
	case *parse.FieldNode:
		visit(n.Ident)
	case *parse.VariableNode:
		visit(n.Ident)
	case *parse.ChainNode:
		walkTemplate(n.Node, visit)
	case *parse.TemplateNode:
		walkTemplate(n.Pipe, visit)
	case *parse.IfNode:
		walkBranch(n.BranchNode, visit)
	case *parse.RangeNode:
		walkBranch(n.BranchNode, visit)
	case *parse.WithNode:
		walkBranch(n.BranchNode, visit)
	}
}

func walkBranch(branch parse.BranchNode, visit func(ident []string)) {
	walkTemplate(branch.Pipe, visit)
	walkTemplate(branch.List, visit)
	walkTemplate(branch.ElseList, visit)
}

// checkPattern reports a glob that cannot work. A pattern that is a template
// can only be checked once it has been rendered, so only its syntax is checked.
func checkPattern(pattern string) error {
	if isTemplated(pattern) {
		return checkTemplate(pattern)
	}
	if !doublestar.ValidatePattern(pattern) {
		return doublestar.ErrBadPattern
	}
	return nil
}

// validateEnv reports Env values that are not valid templates.
func (t ProjectTemplate) validateEnv() []error {
	var errs []error
	for _, key := range slices.Sorted(maps.Keys(t.Env)) {
		if text, ok := t.Env[key].(string); ok {
			if err := checkTemplate(text); err != nil {
				errs = append(errs, fmt.Errorf("env %q: %w", key, err))
			}
		}
	}
	return errs
}

// validateRenames checks that every rename is well formed. A path that is a
// template can only be checked once it is rendered, so the checks on where it
// leads are limited to plain paths and are repeated when the rename is applied.
func (t ProjectTemplate) validateRenames() []error {
	var errs []error
	targets := make(map[string]bool, len(t.Rename))

	for _, rename := range t.Rename {
		if strings.TrimSpace(rename.From) == "" || strings.TrimSpace(rename.To) == "" {
			errs = append(errs, errors.New("rename needs both from and to"))
			continue
		}

		for _, field := range []struct{ name, text string }{{"from", rename.From}, {"to", rename.To}, {"when", rename.When}} {
			if err := checkTemplate(field.text); err != nil {
				errs = append(errs, fmt.Errorf("rename %q: %s: %w", rename.From, field.name, err))
			}
		}

		if isTemplated(rename.From) || isTemplated(rename.To) {
			continue
		}

		from, to := cleanPath(rename.From), cleanPath(rename.To)
		switch {
		case !filepath.IsLocal(from) || !filepath.IsLocal(to) || from == "." || to == ".":
			errs = append(errs, fmt.Errorf("rename %q to %q: paths must be relative and stay inside the project", rename.From, rename.To))
			continue
		case touchesSkipped(from) || touchesSkipped(to):
			errs = append(errs, fmt.Errorf("rename %q to %q: a directory that is never copied from a template (%s) cannot be used", rename.From, rename.To, strings.Join(dirsToSkip, ", ")))
			continue
		case from == to:
			errs = append(errs, fmt.Errorf("rename %q: from and to are the same", rename.From))
			continue
		case from == TemplateConfigName:
			errs = append(errs, fmt.Errorf("rename %q: the config file is never copied", rename.From))
			continue
		}

		// Two renames may share a target when their conditions keep one of them out
		if strings.TrimSpace(rename.When) == "" {
			if targets[to] {
				errs = append(errs, fmt.Errorf("rename %q: more than one rename ends up at %q", rename.From, rename.To))
			}
			targets[to] = true
		}
	}
	return errs
}

// isTemplated reports whether text has anything to render.
func isTemplated(text string) bool {
	return strings.Contains(text, "{{")
}

// checkTemplate reports a syntax error in text, so it is found when the
// template is loaded and not halfway through rendering. What a template renders
// to depends on the answers, so that can only be checked when it is rendered.
func checkTemplate(text string) error {
	_, err := template.New("").Option("missingkey=error").Parse(text)
	return err
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
