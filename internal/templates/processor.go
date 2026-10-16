package templates

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

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
	configBytes, err := os.ReadFile(filepath.Join(dir, TemplateConfigName))
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

	for _, pattern := range config.Raw {
		if _, err := path.Match(pattern, ""); err != nil {
			return nil, fmt.Errorf("invalid raw pattern %q in %s: %w", pattern, TemplateConfigName, err)
		}
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

	for _, skip := range dirsToSkip {
		if err := root.RemoveAll(skip); err != nil {
			return fmt.Errorf("error removing path %s: %w", skip, err)
		}
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
		if rel != "." && rel != TemplateConfigName {
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

	// Finally, delete the template config file
	if err := root.Remove(TemplateConfigName); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
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

// matchesAny reports whether rel, its base name, or any of its parent
// directories matches one of the glob patterns.
func matchesAny(patterns []string, rel string) bool {
	for _, pattern := range patterns {
		if ok, _ := path.Match(pattern, path.Base(rel)); ok {
			return true
		}
		for candidate := rel; candidate != "."; candidate = path.Dir(candidate) {
			if ok, _ := path.Match(pattern, candidate); ok {
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
