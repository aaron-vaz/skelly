package templates

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

// binarySniffLen mirrors git's heuristic: a NUL byte in the first 8000 bytes means binary.
const binarySniffLen = 8000

var dirsToSkip = []string{".git"}

// Processor oversees the orchestration of template parsing, extracting configs,
// and invoking renderers against template directories.
type Processor struct {
	renderer *RendererService
}

// CreateTemplate reads the local configuration file located in the fetched template
// directory, unmarshaling it into a ProjectTemplate struct. It returns nil if the
// template has no configuration file.
func (p *Processor) CreateTemplate(dir string) (*ProjectTemplate, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()

	configBytes, err := root.ReadFile(TemplateConfigName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var config ProjectTemplate
	if err := yaml.Unmarshal(configBytes, &config); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", TemplateConfigName, err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", TemplateConfigName, err)
	}

	return &config, nil
}

// ApplyTemplate renders the template in dir in place, in two phases: first the
// contents of every file, then the names of files and directories. All file
// access goes through an os.Root, so neither a symlink nor a rendered name can
// make it read or write outside dir.
func (p *Processor) ApplyTemplate(config ProjectTemplate, dir string) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()

	if err := removeSkippedDirs(root); err != nil {
		return err
	}

	// Derived values come first, as everything else may use them
	config, err = p.renderEnv(config)
	if err != nil {
		return err
	}

	// Patterns may be templates too, and are needed in their final form from here on
	config, err = p.renderPatterns(config)
	if err != nil {
		return err
	}

	// Excluded paths go next, so their contents are never rendered and cannot fail the render
	if err := p.removeExcluded(root, config); err != nil {
		return err
	}

	// Phase 1: Render file contents in place
	err = fs.WalkDir(root.FS(), ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Directories are handled in phase 2, and symlinks are never written through
		if !d.Type().IsRegular() || rel == TemplateConfigName || matchesAny(config.Raw, rel) {
			return nil
		}

		return p.renderContents(root, config, rel)
	})
	if err != nil {
		return fmt.Errorf("error rendering file contents: %w", err)
	}

	// Phase 2: Rename files and directories from the bottom up
	var pathsToRename []string
	err = fs.WalkDir(root.FS(), ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if rel != "." && rel != TemplateConfigName && !matchesAny(config.Raw, rel) {
			pathsToRename = append(pathsToRename, rel)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Deepest first, so a child is renamed while its parent still has its original name
	slices.SortStableFunc(pathsToRename, func(a, b string) int {
		return strings.Count(b, "/") - strings.Count(a, "/")
	})

	for _, rel := range pathsToRename {
		if err := p.renameEntry(root, config, rel); err != nil {
			return fmt.Errorf("error renaming path %s: %w", rel, err)
		}
	}

	// Renames come last, so the template's own names are what Raw and Exclude see
	if err := p.applyRenames(root, config); err != nil {
		return err
	}

	// Finally, delete the template config file
	if err := root.Remove(TemplateConfigName); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// applyRenames moves every path listed under Rename whose condition holds,
// creating the parent directories of the new location. A missing source or an
// existing target is an error, so a typo or a clash never silently leaves the
// project half renamed.
func (p *Processor) applyRenames(root *os.Root, config ProjectTemplate) error {
	for _, rename := range config.Rename {
		if strings.TrimSpace(rename.When) != "" {
			apply, err := p.evaluate(config, "rename "+rename.From, rename.When)
			if err != nil {
				return fmt.Errorf("rename %q: %w", rename.From, err)
			}
			if !apply {
				continue
			}
		}

		from, err := p.renderPath(config, "rename from", rename.From)
		if err != nil {
			return fmt.Errorf("rename %q: from: %w", rename.From, err)
		}
		to, err := p.renderPath(config, "rename to", rename.To)
		if err != nil {
			return fmt.Errorf("rename %q: to: %w", rename.From, err)
		}
		if from == to {
			return fmt.Errorf("rename %q: from and to are the same", from)
		}

		if _, err := root.Lstat(from); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("rename %q: no such file or directory in the template", from)
			}
			return err
		}
		if _, err := root.Lstat(to); err == nil {
			return fmt.Errorf("rename %q to %q: %s already exists", from, to, to)
		}

		if dir := path.Dir(to); dir != "." {
			if err := root.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("rename %q: %w", from, err)
			}
		}
		if err := root.Rename(from, to); err != nil {
			return fmt.Errorf("rename %q to %q: %w", from, to, err)
		}
	}
	return nil
}

// renderPath renders a path of the config and checks that it stays inside the project.
func (p *Processor) renderPath(config ProjectTemplate, name string, text string) (string, error) {
	rendered, err := p.renderer.Render(config, name, text)
	if err != nil {
		return "", err
	}

	cleaned := cleanPath(rendered)
	if !filepath.IsLocal(cleaned) || cleaned == TemplateConfigName {
		return "", fmt.Errorf("%q is not a path inside the project", rendered)
	}
	return cleaned, nil
}

// removeExcluded deletes every path matched by an Exclude entry whose
// condition renders as true.
func (p *Processor) removeExcluded(root *os.Root, config ProjectTemplate) error {
	var patterns []string
	for _, exclusion := range config.Exclude {
		excluded, err := p.isExcluded(config, exclusion)
		if err != nil {
			return err
		}
		if excluded {
			patterns = append(patterns, exclusion.Patterns()...)
		}
	}
	if len(patterns) == 0 {
		return nil
	}

	var matched []string
	err := fs.WalkDir(root.FS(), ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if rel == "." || rel == TemplateConfigName || !matchesAny(patterns, rel) {
			return nil
		}
		matched = append(matched, rel)
		if d.IsDir() {
			return fs.SkipDir
		}
		return nil
	})
	if err != nil {
		return err
	}

	for _, rel := range matched {
		if err := root.RemoveAll(rel); err != nil {
			return fmt.Errorf("error removing excluded path %s: %w", rel, err)
		}
	}
	return nil
}

// RenderInput renders the description, default and choices of the named input,
// which may use the values of the inputs answered so far. Env is not available
// to inputs, as it is derived from them, so it is empty here.
func (p *Processor) RenderInput(config ProjectTemplate, name string) (Input, error) {
	input := config.Inputs[name]
	if !inputIsTemplated(input) {
		return input, nil
	}

	scope := config
	scope.Env = map[string]any{}

	render := func(field string, text string) (string, error) {
		rendered, err := p.renderer.Render(scope, "input "+name+" "+field, text)
		if err != nil {
			return "", fmt.Errorf("input %q: %s: %w", name, field, err)
		}
		return rendered, nil
	}

	var err error
	if input.Description, err = render("description", input.Description); err != nil {
		return input, err
	}
	if text, ok := input.Default.(string); ok {
		if input.Default, err = render("default", text); err != nil {
			return input, err
		}
	}

	input.Choices = slices.Clone(input.Choices)
	for i, choice := range input.Choices {
		if input.Choices[i], err = render("choices", choice); err != nil {
			return input, err
		}
	}

	if err := validateChoices(name, input); err != nil {
		return input, err
	}
	return input, nil
}

// renderPatterns renders the globs of Raw and Exclude, which may be templates,
// and checks the result. The config it was given is left as it is.
func (p *Processor) renderPatterns(config ProjectTemplate) (ProjectTemplate, error) {
	render := func(what string, pattern string) (string, error) {
		rendered, err := p.renderer.Render(config, what+" "+pattern, pattern)
		if err != nil {
			return "", fmt.Errorf("%s pattern %q: %w", what, pattern, err)
		}
		rendered = strings.TrimSpace(rendered)
		if rendered == "" || !doublestar.ValidatePattern(rendered) {
			return "", fmt.Errorf("%s pattern %q renders as %q, which is not a valid glob", what, pattern, rendered)
		}
		return rendered, nil
	}

	raw := make([]string, 0, len(config.Raw))
	for _, pattern := range config.Raw {
		rendered, err := render("raw", pattern)
		if err != nil {
			return config, err
		}
		raw = append(raw, rendered)
	}

	exclude := make([]Exclusion, 0, len(config.Exclude))
	for _, exclusion := range config.Exclude {
		patterns := make([]string, 0, len(exclusion.Patterns()))
		for _, pattern := range exclusion.Patterns() {
			rendered, err := render("exclude", pattern)
			if err != nil {
				return config, err
			}
			patterns = append(patterns, rendered)
		}
		exclude = append(exclude, Exclusion{Paths: patterns, When: exclusion.When})
	}

	config.Raw = raw
	config.Exclude = exclude
	return config, nil
}

func (p *Processor) isExcluded(config ProjectTemplate, exclusion Exclusion) (bool, error) {
	excluded, err := p.evaluate(config, "exclude "+exclusion.label(), exclusion.When)
	if err != nil {
		return false, fmt.Errorf("exclude %q: %w", exclusion.label(), err)
	}
	return excluded, nil
}

// evaluate renders a condition and reads the result as a boolean.
func (p *Processor) evaluate(config ProjectTemplate, name string, when string) (bool, error) {
	rendered, err := p.renderer.Render(config, name, when)
	if err != nil {
		return false, err
	}

	result, err := strconv.ParseBool(strings.TrimSpace(rendered))
	if err != nil {
		return false, fmt.Errorf("when must render as true or false, got %q", strings.TrimSpace(rendered))
	}
	return result, nil
}

// renderEnv renders the string values of Env as templates that can use the
// inputs. They are rendered against an empty Env, so one value referring to
// another is an error and not an unrendered template.
func (p *Processor) renderEnv(config ProjectTemplate) (ProjectTemplate, error) {
	if len(config.Env) == 0 {
		return config, nil
	}

	scope := config
	scope.Env = map[string]any{}

	rendered := make(map[string]any, len(config.Env))
	for _, key := range slices.Sorted(maps.Keys(config.Env)) {
		text, ok := config.Env[key].(string)
		if !ok {
			rendered[key] = config.Env[key]
			continue
		}

		value, err := p.renderer.Render(scope, "env "+key, text)
		if err != nil {
			return config, fmt.Errorf("env %q: %w", key, err)
		}
		rendered[key] = value
	}

	config.Env = rendered
	return config, nil
}

func (p *Processor) renderContents(root *os.Root, config ProjectTemplate, rel string) error {
	content, err := root.ReadFile(rel)
	if err != nil {
		return err
	}

	if bytes.IndexByte(content[:min(len(content), binarySniffLen)], 0) >= 0 {
		return nil
	}

	rendered, err := p.renderer.Render(config, rel, string(content))
	if err != nil {
		return fmt.Errorf("%w (list the file under 'raw' in %s to copy it without rendering)", err, TemplateConfigName)
	}

	info, err := root.Lstat(rel)
	if err != nil {
		return err
	}

	return root.WriteFile(rel, []byte(rendered), info.Mode().Perm())
}

// renameEntry renders only the final element of rel, so the parent directories,
// which are renamed later, are still found under their original names.
func (p *Processor) renameEntry(root *os.Root, config ProjectTemplate, rel string) error {
	parent, name := path.Split(rel)

	rendered, err := p.renderer.Render(config, rel, name)
	if err != nil {
		return err
	}
	if rendered == name {
		return nil
	}

	// A rendered name may add subdirectories (e.g. a Java package "com/example"),
	// but must stay inside its parent directory
	if !filepath.IsLocal(rendered) {
		return fmt.Errorf("rendered name %q is empty or points outside its directory", rendered)
	}

	target := path.Join(parent, rendered)
	if _, err := root.Lstat(target); err == nil {
		return fmt.Errorf("rendered path %s already exists", target)
	}

	if targetDir := path.Dir(target); targetDir != "." {
		if err := root.MkdirAll(targetDir, 0o755); err != nil {
			return err
		}
	}

	return root.Rename(rel, target)
}

// RemoveTemplateMetadata deletes the parts of a template that must never end
// up in a generated project, such as its .git directory. ApplyTemplate already
// does this, so it is only needed for templates that are not rendered.
func (p *Processor) RemoveTemplateMetadata(dir string) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()

	return removeSkippedDirs(root)
}

func removeSkippedDirs(root *os.Root) error {
	for _, skip := range dirsToSkip {
		if err := root.RemoveAll(skip); err != nil {
			return fmt.Errorf("error removing path %s: %w", skip, err)
		}
	}
	return nil
}

// matchesAny reports whether rel, its base name, or any of its parent
// directories matches one of the glob patterns. Patterns use doublestar
// syntax, so "**" matches across directory separators.
func matchesAny(patterns []string, rel string) bool {
	for _, pattern := range patterns {
		if ok, _ := doublestar.Match(pattern, path.Base(rel)); ok {
			return true
		}
		for candidate := rel; candidate != "."; candidate = path.Dir(candidate) {
			if ok, _ := doublestar.Match(pattern, candidate); ok {
				return true
			}
		}
	}
	return false
}

// NewTemplateProcessor instantiates a Processor equipped with a RendererService.
func NewTemplateProcessor(renderer *RendererService) *Processor {
	return &Processor{
		renderer: renderer,
	}
}
