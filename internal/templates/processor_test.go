package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessor_CreateTemplate(t *testing.T) {
	proc := NewTemplateProcessor(NewRendererService())

	tests := []struct {
		name      string
		setupDest func(t *testing.T, dest string)
		wantNil   bool
		wantErr   bool
		wantName  string
	}{
		{
			name: "no config file",
			setupDest: func(t *testing.T, dest string) {
				// empty dir
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "valid config file",
			setupDest: func(t *testing.T, dest string) {
				config := `name: test-project`
				_ = os.WriteFile(filepath.Join(dest, TemplateConfigName), []byte(config), 0644)
			},
			wantNil:  false,
			wantErr:  false,
			wantName: "test-project",
		},
		{
			name: "invalid config file",
			setupDest: func(t *testing.T, dest string) {
				config := `name: [invalid yaml`
				_ = os.WriteFile(filepath.Join(dest, TemplateConfigName), []byte(config), 0644)
			},
			wantNil: true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			dest := t.TempDir()
			tt.setupDest(t, dest)

			// When
			cfg, err := proc.CreateTemplate(dest)

			// Then
			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateTemplate() error = nil, wantErr %v", tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("CreateTemplate() error = %v, wantErr %v", err, tt.wantErr)
				}

				// And
				if tt.wantNil && cfg != nil {
					t.Errorf("Expected nil config but got %v", cfg)
				}
				if !tt.wantNil {
					if cfg == nil {
						t.Errorf("Expected non-nil config")
					} else if cfg.Name != tt.wantName {
						t.Errorf("Expected Name = %v, got %v", tt.wantName, cfg.Name)
					}
				}
			}
		})
	}
}

func TestProcessor_ApplyTemplate(t *testing.T) {
	proc := NewTemplateProcessor(NewRendererService())

	t.Run("successful apply", func(t *testing.T) {
		// Given
		dest := t.TempDir()

		// Setup .skelly.yaml
		_ = os.WriteFile(filepath.Join(dest, TemplateConfigName), []byte(""), 0644)

		// Setup .git dir to be skipped
		gitDir := filepath.Join(dest, ".git")
		_ = os.MkdirAll(gitDir, 0755)
		_ = os.WriteFile(filepath.Join(gitDir, "ignore.txt"), []byte("ignored"), 0644)

		// Setup a template file
		_ = os.WriteFile(filepath.Join(dest, "{{.Name}}.txt"), []byte("Hello {{.Name}}!"), 0644)

		// Create a config
		cfg := ProjectTemplate{
			Name: "World",
		}

		// When
		err := proc.ApplyTemplate(cfg, dest)

		// Then
		if err != nil {
			t.Fatalf("ApplyTemplate failed: %v", err)
		}

		// And
		if _, err := os.Stat(gitDir); !os.IsNotExist(err) {
			t.Errorf("Expected .git to be removed")
		}

		// And
		if _, err := os.Stat(filepath.Join(dest, TemplateConfigName)); !os.IsNotExist(err) {
			t.Errorf("Expected %s to be removed", TemplateConfigName)
		}

		// And
		renamedFile := filepath.Join(dest, "World.txt")
		content, err := os.ReadFile(renamedFile)
		if err != nil {
			t.Errorf("Expected renamed file World.txt to exist: %v", err)
		} else if string(content) != "Hello World!" {
			t.Errorf("Expected file content 'Hello World!', got '%s'", string(content))
		}
	})
}

func TestProcessor_CreateTemplate_InvalidRawPattern(t *testing.T) {
	// Given
	dest := t.TempDir()
	_ = os.WriteFile(filepath.Join(dest, TemplateConfigName), []byte("raw: ['[unclosed']"), 0644)

	// When
	_, err := NewTemplateProcessor(NewRendererService()).CreateTemplate(dest)

	// Then
	if err == nil || !strings.Contains(err.Error(), "invalid raw pattern") {
		t.Errorf("CreateTemplate() error = %v, want invalid raw pattern", err)
	}
}

func TestProcessor_ApplyTemplate_Scenarios(t *testing.T) {
	cfg := ProjectTemplate{
		Name: "World",
		Inputs: map[string]Input{
			"name": {Value: "foo"},
			"pkg":  {Value: "com/example"},
			"up":   {Value: "../outside"},
		},
		Raw: []string{".github", "*.tpl"},
	}

	tests := []struct {
		name    string
		files   map[string]string // relative path -> content
		setup   func(t *testing.T, dest string)
		wantErr string
		want    map[string]string // relative path -> expected content
		absent  []string
	}{
		{
			name:   "file inside a templated directory is renamed too",
			files:  map[string]string{"{{.Inputs.name.Value}}/{{.Inputs.name.Value}}.txt": "hi {{.Inputs.name.Value}}"},
			want:   map[string]string{"foo/foo.txt": "hi foo"},
			absent: []string{"{{.Inputs.name.Value}}"},
		},
		{
			name:  "deeply nested templated directories",
			files: map[string]string{"a/{{.Name}}/b/{{.Inputs.name.Value}}/{{.Name}}.go": "package x"},
			want:  map[string]string{"a/World/b/foo/World.go": "package x"},
		},
		{
			name:  "rendered name may create subdirectories",
			files: map[string]string{"src/{{.Inputs.pkg.Value}}/Main.java": "class Main {}"},
			want:  map[string]string{"src/com/example/Main.java": "class Main {}"},
		},
		{
			name:  "raw directories and patterns are copied unrendered",
			files: map[string]string{".github/workflows/ci.yml": "sha: ${{ github.sha }}", "x.tpl": "{{.Missing}}", "y.txt": "{{.Name}}"},
			want:  map[string]string{".github/workflows/ci.yml": "sha: ${{ github.sha }}", "x.tpl": "{{.Missing}}", "y.txt": "World"},
		},
		{
			name:  "raw paths keep their templated-looking names",
			files: map[string]string{".github/{{cookiecutter.project}}/x.txt": "{{ y }}"},
			want:  map[string]string{".github/{{cookiecutter.project}}/x.txt": "{{ y }}"},
		},
		{
			name:    "unrendered file with foreign {{ }} syntax suggests raw",
			files:   map[string]string{"ci.yml": "sha: ${{ github.sha }}"},
			wantErr: "list the file under 'raw'",
		},
		{
			name:  "binary files are left untouched",
			files: map[string]string{"logo.bin": "\x00\x01{{.Name}}"},
			want:  map[string]string{"logo.bin": "\x00\x01{{.Name}}"},
		},
		{
			name:    "missing input key is an error",
			files:   map[string]string{"a.txt": "{{.Inputs.nope.Value}}"},
			wantErr: "map has no entry for key",
		},
		{
			name:    "rendered name cannot escape the destination",
			files:   map[string]string{"{{.Inputs.up.Value}}": "x"},
			wantErr: "points outside its directory",
		},
		{
			name:    "rendered name cannot clobber an existing file",
			files:   map[string]string{"{{.Name}}.txt": "new", "World.txt": "old"},
			wantErr: "already exists",
		},
		{
			name: "symlinks are never written through",
			setup: func(t *testing.T, dest string) {
				outside := filepath.Join(t.TempDir(), "victim.txt")
				_ = os.WriteFile(outside, []byte("{{.Name}}"), 0644)
				if err := os.Symlink(outside, filepath.Join(dest, "link.txt")); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					content, _ := os.ReadFile(outside)
					if string(content) != "{{.Name}}" {
						t.Errorf("File outside the template was modified: %q", content)
					}
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			dest := t.TempDir()
			_ = os.WriteFile(filepath.Join(dest, TemplateConfigName), []byte(""), 0644)
			for rel, content := range tt.files {
				full := filepath.Join(dest, filepath.FromSlash(rel))
				_ = os.MkdirAll(filepath.Dir(full), 0755)
				_ = os.WriteFile(full, []byte(content), 0644)
			}
			if tt.setup != nil {
				tt.setup(t, dest)
			}

			// When
			err := NewTemplateProcessor(NewRendererService()).ApplyTemplate(cfg, dest)

			// Then
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("ApplyTemplate() error = %v, want error containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ApplyTemplate() error = %v", err)
			}

			// And
			for rel, want := range tt.want {
				content, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(rel)))
				if err != nil {
					t.Errorf("Expected %s to exist: %v", rel, err)
				} else if string(content) != want {
					t.Errorf("%s = %q, want %q", rel, content, want)
				}
			}

			// And
			for _, rel := range tt.absent {
				if _, err := os.Lstat(filepath.Join(dest, rel)); !os.IsNotExist(err) {
					t.Errorf("Expected %s to be gone", rel)
				}
			}
		})
	}
}

func TestProcessor_ApplyTemplate_Exclude(t *testing.T) {
	tests := []struct {
		name    string
		kind    string
		exclude []Exclusion
		files   map[string]string
		want    []string
		absent  []string
		wantErr string
	}{
		{
			name:    "directory is removed when the condition is true",
			kind:    "cli",
			exclude: []Exclusion{{Path: "internal/server", When: `{{eq .Inputs.kind.Value "cli"}}`}},
			files:   map[string]string{"internal/server/server.go": "package server", "internal/app/app.go": "package app"},
			want:    []string{"internal/app/app.go"},
			absent:  []string{"internal/server"},
		},
		{
			name:    "path is kept when the condition is false",
			kind:    "service",
			exclude: []Exclusion{{Path: "internal/server", When: `{{eq .Inputs.kind.Value "cli"}}`}},
			files:   map[string]string{"internal/server/server.go": "package server"},
			want:    []string{"internal/server/server.go"},
		},
		{
			name:    "raw files can be excluded and are never rendered",
			kind:    "cli",
			exclude: []Exclusion{{Path: ".github/workflows/docker.yml", When: "true"}},
			files:   map[string]string{".github/workflows/docker.yml": "${{ secrets.TOKEN }}", ".github/workflows/ci.yml": "${{ github.sha }}"},
			want:    []string{".github/workflows/ci.yml"},
			absent:  []string{".github/workflows/docker.yml"},
		},
		{
			name:    "excluded files with unrenderable content do not fail the render",
			kind:    "cli",
			exclude: []Exclusion{{Path: "*.tmpl", When: "true"}},
			files:   map[string]string{"a.tmpl": "{{.Missing.Key}}", "b.txt": "ok"},
			want:    []string{"b.txt"},
			absent:  []string{"a.tmpl"},
		},
		{
			name:    "paths share a single condition",
			kind:    "cli",
			exclude: []Exclusion{{Paths: []string{"internal/server", "Dockerfile"}, When: "true"}},
			files:   map[string]string{"internal/server/server.go": "package server", "Dockerfile": "FROM scratch", "main.go": "package main"},
			want:    []string{"main.go"},
			absent:  []string{"internal/server", "Dockerfile"},
		},
		{
			name:    "path and paths can be combined",
			kind:    "cli",
			exclude: []Exclusion{{Path: "a.txt", Paths: []string{"b.txt"}, When: "true"}},
			files:   map[string]string{"a.txt": "x", "b.txt": "x", "c.txt": "x"},
			want:    []string{"c.txt"},
			absent:  []string{"a.txt", "b.txt"},
		},
		{
			name:    "double star matches files at any depth",
			kind:    "cli",
			exclude: []Exclusion{{Path: "**/*_test.go", When: "true"}},
			files:   map[string]string{"a_test.go": "x", "pkg/b_test.go": "x", "pkg/deep/c_test.go": "x", "pkg/deep/c.go": "x"},
			want:    []string{"pkg/deep/c.go"},
			absent:  []string{"a_test.go", "pkg/b_test.go", "pkg/deep/c_test.go"},
		},
		{
			name:    "double star in the middle of a pattern spans directories",
			kind:    "cli",
			exclude: []Exclusion{{Path: "docs/**/*.md", When: "true"}},
			files:   map[string]string{"docs/a.md": "x", "docs/x/y/b.md": "x", "docs/x/keep.txt": "x", "README.md": "x"},
			want:    []string{"docs/x/keep.txt", "README.md"},
			absent:  []string{"docs/a.md", "docs/x/y/b.md"},
		},
		{
			name:    "single star does not cross directories",
			kind:    "cli",
			exclude: []Exclusion{{Path: "docs/*/*.md", When: "true"}},
			files:   map[string]string{"docs/x/a.md": "x", "docs/x/y/b.md": "x"},
			want:    []string{"docs/x/y/b.md"},
			absent:  []string{"docs/x/a.md"},
		},
		{
			name:    "braces match alternatives",
			kind:    "cli",
			exclude: []Exclusion{{Path: "Dockerfile{,.dev}", When: "true"}},
			files:   map[string]string{"Dockerfile": "x", "Dockerfile.dev": "x", "Dockerfile.prod": "x"},
			want:    []string{"Dockerfile.prod"},
			absent:  []string{"Dockerfile", "Dockerfile.dev"},
		},
		{
			name:    "condition may have surrounding whitespace",
			kind:    "cli",
			exclude: []Exclusion{{Path: "gone.txt", When: "\n  true\n"}},
			files:   map[string]string{"gone.txt": "x"},
			absent:  []string{"gone.txt"},
		},
		{
			name:    "condition must render as a boolean",
			kind:    "cli",
			exclude: []Exclusion{{Path: "a.txt", When: "{{.Inputs.kind.Value}}"}},
			files:   map[string]string{"a.txt": "x"},
			wantErr: `exclude "a.txt": when must render as true or false, got "cli"`,
		},
		{
			name:    "condition referencing an unknown input is an error",
			kind:    "cli",
			exclude: []Exclusion{{Path: "a.txt", When: "{{.Inputs.nope.Value}}"}},
			files:   map[string]string{"a.txt": "x"},
			wantErr: `exclude "a.txt"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			cfg := ProjectTemplate{
				Inputs:  map[string]Input{"kind": {Value: tt.kind}},
				Exclude: tt.exclude,
			}
			dest := t.TempDir()
			_ = os.WriteFile(filepath.Join(dest, TemplateConfigName), []byte(""), 0644)
			for rel, content := range tt.files {
				full := filepath.Join(dest, filepath.FromSlash(rel))
				_ = os.MkdirAll(filepath.Dir(full), 0755)
				_ = os.WriteFile(full, []byte(content), 0644)
			}
			cfg.Raw = []string{".github"}

			// When
			err := NewTemplateProcessor(NewRendererService()).ApplyTemplate(cfg, dest)

			// Then
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("ApplyTemplate() error = %v, want error containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ApplyTemplate() error = %v", err)
			}

			// And
			for _, rel := range tt.want {
				if _, err := os.Lstat(filepath.Join(dest, filepath.FromSlash(rel))); err != nil {
					t.Errorf("Expected %s to exist: %v", rel, err)
				}
			}
			for _, rel := range tt.absent {
				if _, err := os.Lstat(filepath.Join(dest, filepath.FromSlash(rel))); !os.IsNotExist(err) {
					t.Errorf("Expected %s to be gone", rel)
				}
			}
		})
	}
}

func TestMatchesAny(t *testing.T) {
	tests := []struct {
		pattern string
		rel     string
		want    bool
	}{
		{"*.png", "assets/img/logo.png", true},
		{".github", ".github/workflows/ci.yml", true},
		{"charts/*", "charts/app/templates/deploy.yaml", true},
		{"docs/*.md", "docs/readme.md", true},
		{"docs/*.md", "other/readme.md", false},
		{"*.png", "logo.jpg", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.rel, func(t *testing.T) {
			// When
			got := matchesAny([]string{tt.pattern}, tt.rel)

			// Then
			if got != tt.want {
				t.Errorf("matchesAny(%q, %q) = %v, want %v", tt.pattern, tt.rel, got, tt.want)
			}
		})
	}
}

func TestProcessor_CreateTemplate_SymlinkedConfigOutsideTemplate(t *testing.T) {
	// Given a config that is a symlink to a file outside the template
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret")
	_ = os.WriteFile(outside, []byte("name: secret"), 0644)
	if err := os.Symlink(outside, filepath.Join(dir, TemplateConfigName)); err != nil {
		t.Fatal(err)
	}

	// When
	_, err := NewTemplateProcessor(NewRendererService()).CreateTemplate(dir)

	// Then the link is not followed
	if err == nil {
		t.Error("CreateTemplate() error = nil, want refusal to read outside the template")
	}
}

func TestProcessor_RemoveTemplateMetadata(t *testing.T) {
	// Given
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)

	// When
	err := NewTemplateProcessor(NewRendererService()).RemoveTemplateMetadata(dir)

	// Then
	if err != nil {
		t.Fatalf("RemoveTemplateMetadata() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); !os.IsNotExist(err) {
		t.Errorf("Expected .git to be removed, stat err = %v", err)
	}

	// And
	if _, err := os.Stat(filepath.Join(dir, "main.go")); err != nil {
		t.Errorf("Expected other files to be kept: %v", err)
	}
}

func TestProcessor_ApplyTemplate_Rename(t *testing.T) {
	tests := []struct {
		name    string
		rename  []Renaming
		raw     []string
		exclude []Exclusion
		files   map[string]string
		want    map[string]string
		absent  []string
		wantErr string
	}{
		{
			name:   "directory is moved with its contents",
			rename: []Renaming{{From: "github", To: ".github"}},
			files:  map[string]string{"github/workflows/ci.yml": "a", "github/dependabot.yml": "b", "main.go": "c"},
			want:   map[string]string{".github/workflows/ci.yml": "a", ".github/dependabot.yml": "b", "main.go": "c"},
			absent: []string{"github"},
		},
		{
			name:   "file is moved and missing parent directories are created",
			rename: []Renaming{{From: "gitignore", To: "config/.gitignore"}},
			files:  map[string]string{"gitignore": "build/"},
			want:   map[string]string{"config/.gitignore": "build/"},
			absent: []string{"gitignore"},
		},
		{
			name:    "raw and exclude refer to the template's own names",
			rename:  []Renaming{{From: "github", To: ".github"}},
			raw:     []string{"github"},
			exclude: []Exclusion{{Path: "github/workflows/docker.yml", When: "true"}},
			files: map[string]string{
				"github/workflows/ci.yml":     "sha: ${{ github.sha }}",
				"github/workflows/docker.yml": "gone",
			},
			want:   map[string]string{".github/workflows/ci.yml": "sha: ${{ github.sha }}"},
			absent: []string{".github/workflows/docker.yml"},
		},
		{
			name:   "contents and names are rendered before the move",
			rename: []Renaming{{From: "src", To: "app"}},
			files:  map[string]string{"src/{{.Inputs.kind.Value}}.txt": "kind is {{.Inputs.kind.Value}}"},
			want:   map[string]string{"app/cli.txt": "kind is cli"},
		},
		{
			name:   "renames apply in order, so a later one can use an earlier result",
			rename: []Renaming{{From: "a", To: "b"}, {From: "b/x.txt", To: "c.txt"}},
			files:  map[string]string{"a/x.txt": "x"},
			want:   map[string]string{"c.txt": "x"},
		},
		{
			name:    "missing source is an error",
			rename:  []Renaming{{From: "nope", To: "somewhere"}},
			files:   map[string]string{"a.txt": "x"},
			wantErr: `rename "nope": no such file or directory in the template`,
		},
		{
			name:    "existing target is an error",
			rename:  []Renaming{{From: "a.txt", To: "b.txt"}},
			files:   map[string]string{"a.txt": "a", "b.txt": "b"},
			wantErr: `rename "a.txt" to "b.txt": b.txt already exists`,
		},
		{
			name:    "an excluded source can no longer be renamed",
			rename:  []Renaming{{From: "a.txt", To: "b.txt"}},
			exclude: []Exclusion{{Path: "a.txt", When: "true"}},
			files:   map[string]string{"a.txt": "a"},
			wantErr: `rename "a.txt": no such file or directory in the template`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			cfg := ProjectTemplate{
				Inputs:  map[string]Input{"kind": {Value: "cli"}},
				Rename:  tt.rename,
				Raw:     tt.raw,
				Exclude: tt.exclude,
			}
			dest := t.TempDir()
			_ = os.WriteFile(filepath.Join(dest, TemplateConfigName), []byte(""), 0644)
			for rel, content := range tt.files {
				full := filepath.Join(dest, filepath.FromSlash(rel))
				_ = os.MkdirAll(filepath.Dir(full), 0755)
				_ = os.WriteFile(full, []byte(content), 0644)
			}

			// When
			err := NewTemplateProcessor(NewRendererService()).ApplyTemplate(cfg, dest)

			// Then
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("ApplyTemplate() error = %v, want error containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ApplyTemplate() error = %v", err)
			}

			// And
			for rel, content := range tt.want {
				got, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(rel)))
				if err != nil || string(got) != content {
					t.Errorf("%s = %q (%v), want %q", rel, got, err, content)
				}
			}
			for _, rel := range tt.absent {
				if _, err := os.Stat(filepath.Join(dest, filepath.FromSlash(rel))); !os.IsNotExist(err) {
					t.Errorf("Expected %s to be absent, stat err = %v", rel, err)
				}
			}
		})
	}
}

func TestProcessor_RenderInput(t *testing.T) {
	config := ProjectTemplate{
		Inputs: map[string]Input{
			"owner":  {Value: "me"},
			"name":   {Value: "app"},
			"module": {Description: "Module of {{.Inputs.name.Value}}", Default: "github.com/{{.Inputs.owner.Value}}/{{.Inputs.name.Value}}", Choices: []string{"github.com/me/app", "other"}},
			"count":  {Default: 3},
			"env":    {Default: "{{.Env.Module}}"},
			"bad":    {Default: "{{.Inputs.owner.Value}}", Choices: []string{"x"}},
		},
		Env: map[string]any{"Module": "unrendered {{.Inputs.name.Value}}"},
	}
	processor := NewTemplateProcessor(NewRendererService())

	tests := []struct {
		name    string
		input   string
		check   func(t *testing.T, got Input)
		wantErr string
	}{
		{
			name:  "description, default and choices are rendered from other inputs",
			input: "module",
			check: func(t *testing.T, got Input) {
				if got.Description != "Module of app" || got.Default != "github.com/me/app" || got.Choices[0] != "github.com/me/app" {
					t.Errorf("got %+v", got)
				}
			},
		},
		{
			name:  "an input without templates is returned as it is",
			input: "count",
			check: func(t *testing.T, got Input) {
				if got.Default != 3 {
					t.Errorf("got %+v", got)
				}
			},
		},
		{
			name:    "env is not available to inputs",
			input:   "env",
			wantErr: `input "env": default:`,
		},
		{
			name:    "a rendered default outside the rendered choices is an error",
			input:   "bad",
			wantErr: `input "bad": default me is not one of its choices (x)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got, err := processor.RenderInput(config, tt.input)

			// Then
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("RenderInput() error = %v, want error containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("RenderInput() error = %v", err)
			}
			tt.check(t, got)
		})
	}

	// And the config it was given is not changed
	if config.Inputs["module"].Default != "github.com/{{.Inputs.owner.Value}}/{{.Inputs.name.Value}}" {
		t.Errorf("RenderInput changed the config: %+v", config.Inputs["module"])
	}
}

func TestProcessor_ApplyTemplate_TemplatedValues(t *testing.T) {
	tests := []struct {
		name    string
		config  ProjectTemplate
		files   map[string]string
		want    map[string]string
		absent  []string
		wantErr string
	}{
		{
			name: "env values are rendered from the inputs and used by files",
			config: ProjectTemplate{
				Inputs: map[string]Input{"owner": {Value: "me"}, "name": {Value: "app"}},
				Env:    map[string]any{"Module": "github.com/{{.Inputs.owner.Value}}/{{.Inputs.name.Value}}", "Year": 2026},
			},
			files: map[string]string{"go.mod": "module {{.Env.Module}} // {{.Env.Year}}"},
			want:  map[string]string{"go.mod": "module github.com/me/app // 2026"},
		},
		{
			name: "env values cannot refer to other env values",
			config: ProjectTemplate{
				Inputs: map[string]Input{"name": {Value: "app"}},
				Env:    map[string]any{"A": "{{.Inputs.name.Value}}", "B": "{{.Env.A}}"},
			},
			files:   map[string]string{"a.txt": "x"},
			wantErr: `env "B"`,
		},
		{
			name: "exclude and raw patterns are rendered",
			config: ProjectTemplate{
				Inputs:  map[string]Input{"kind": {Value: "cli"}},
				Raw:     []string{"{{.Inputs.kind.Value}}-raw"},
				Exclude: []Exclusion{{Paths: []string{"{{.Inputs.kind.Value}}-only", "plain.txt"}, When: "true"}},
			},
			files:  map[string]string{"cli-only/a.txt": "x", "plain.txt": "x", "cli-raw/b.txt": "{{ keep }}", "c.txt": "{{.Inputs.kind.Value}}"},
			want:   map[string]string{"cli-raw/b.txt": "{{ keep }}", "c.txt": "cli"},
			absent: []string{"cli-only", "plain.txt"},
		},
		{
			name: "a pattern that renders to an invalid glob is an error",
			config: ProjectTemplate{
				Inputs:  map[string]Input{"p": {Value: "[x"}},
				Exclude: []Exclusion{{Path: "{{.Inputs.p.Value}}", When: "true"}},
			},
			files:   map[string]string{"a.txt": "x"},
			wantErr: `exclude pattern "{{.Inputs.p.Value}}" renders as "[x", which is not a valid glob`,
		},
		{
			name: "rename paths are rendered and may use env",
			config: ProjectTemplate{
				Inputs: map[string]Input{"name": {Value: "app"}},
				Env:    map[string]any{"Dir": "svc-{{.Inputs.name.Value}}"},
				Rename: []Renaming{{From: "src", To: "{{.Env.Dir}}/code"}},
			},
			files:  map[string]string{"src/main.go": "package main"},
			want:   map[string]string{"svc-app/code/main.go": "package main"},
			absent: []string{"src"},
		},
		{
			name: "from names the path as it was rendered",
			config: ProjectTemplate{
				Inputs: map[string]Input{"name": {Value: "app"}},
				Rename: []Renaming{{From: "cmd/{{.Inputs.name.Value}}", To: "cmd/main"}},
			},
			files: map[string]string{"cmd/{{.Inputs.name.Value}}/main.go": "package main"},
			want:  map[string]string{"cmd/main/main.go": "package main"},
		},
		{
			name: "rename is skipped when its condition is false and applied when true",
			config: ProjectTemplate{
				Inputs: map[string]Input{"kind": {Value: "service"}},
				Rename: []Renaming{
					{From: "Makefile.cli", To: "Makefile", When: `{{eq .Inputs.kind.Value "cli"}}`},
					{From: "Makefile.service", To: "Makefile", When: `{{eq .Inputs.kind.Value "service"}}`},
				},
			},
			files:  map[string]string{"Makefile.cli": "cli", "Makefile.service": "service"},
			want:   map[string]string{"Makefile": "service", "Makefile.cli": "cli"},
			absent: []string{"Makefile.service"},
		},
		{
			name: "rename condition must be a boolean",
			config: ProjectTemplate{
				Inputs: map[string]Input{"kind": {Value: "cli"}},
				Rename: []Renaming{{From: "a", To: "b", When: "{{.Inputs.kind.Value}}"}},
			},
			files:   map[string]string{"a": "x"},
			wantErr: `rename "a": when must render as true or false, got "cli"`,
		},
		{
			name: "a rendered rename path cannot leave the project",
			config: ProjectTemplate{
				Inputs: map[string]Input{"to": {Value: "../escaped"}},
				Rename: []Renaming{{From: "a", To: "{{.Inputs.to.Value}}"}},
			},
			files:   map[string]string{"a": "x"},
			wantErr: `rename "a": to: "../escaped" is not a path inside the project`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			dest := t.TempDir()
			_ = os.WriteFile(filepath.Join(dest, TemplateConfigName), []byte(""), 0644)
			for rel, content := range tt.files {
				full := filepath.Join(dest, filepath.FromSlash(rel))
				_ = os.MkdirAll(filepath.Dir(full), 0755)
				_ = os.WriteFile(full, []byte(content), 0644)
			}

			// When
			err := NewTemplateProcessor(NewRendererService()).ApplyTemplate(tt.config, dest)

			// Then
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("ApplyTemplate() error = %v, want error containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ApplyTemplate() error = %v", err)
			}

			// And
			for rel, content := range tt.want {
				got, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(rel)))
				if err != nil || string(got) != content {
					t.Errorf("%s = %q (%v), want %q", rel, got, err, content)
				}
			}
			for _, rel := range tt.absent {
				if _, err := os.Stat(filepath.Join(dest, filepath.FromSlash(rel))); !os.IsNotExist(err) {
					t.Errorf("Expected %s to be absent, stat err = %v", rel, err)
				}
			}
		})
	}
}

func TestProcessor_ApplyTemplate_NeverRecreatesGitDir(t *testing.T) {
	tests := []struct {
		name    string
		config  ProjectTemplate
		files   map[string]string
		wantErr string
	}{
		{
			name:    "a rename cannot target .git",
			config:  ProjectTemplate{Rename: []Renaming{{From: "gh", To: "{{.Inputs.d.Value}}/hooks"}}, Inputs: map[string]Input{"d": {Value: ".git"}}},
			files:   map[string]string{"gh/pre-commit": "#!/bin/sh"},
			wantErr: `rename "gh": to: ".git/hooks" would use a directory that is never copied`,
		},
		{
			name:    "case does not matter, as some filesystems ignore it",
			config:  ProjectTemplate{Rename: []Renaming{{From: "gh", To: "{{.Inputs.d.Value}}"}}, Inputs: map[string]Input{"d": {Value: "sub/.GIT"}}},
			files:   map[string]string{"gh/pre-commit": "#!/bin/sh"},
			wantErr: "never copied",
		},
		{
			name:    "a rename cannot start from .git either",
			config:  ProjectTemplate{Rename: []Renaming{{From: "{{.Inputs.d.Value}}", To: "x"}}, Inputs: map[string]Input{"d": {Value: ".git"}}},
			files:   map[string]string{"a": "x"},
			wantErr: "never copied",
		},
		{
			name:    "a rename cannot move the whole project",
			config:  ProjectTemplate{Rename: []Renaming{{From: "a", To: "{{.Inputs.d.Value}}"}}, Inputs: map[string]Input{"d": {Value: "."}}},
			files:   map[string]string{"a": "x"},
			wantErr: `"." is not a path inside the project`,
		},
		{
			name:    "a rendered directory name cannot become .git",
			config:  ProjectTemplate{Env: map[string]any{"d": ".git"}},
			files:   map[string]string{"{{.Env.d}}/pre-commit": "#!/bin/sh"},
			wantErr: `rendered name ".git" would create a directory that is never copied`,
		},
		{
			name:    "a rendered file name cannot place a file inside .git",
			config:  ProjectTemplate{Env: map[string]any{"d": "x/.git/hooks"}},
			files:   map[string]string{"{{.Env.d}}": "#!/bin/sh"},
			wantErr: "never copied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			dest := t.TempDir()
			_ = os.WriteFile(filepath.Join(dest, TemplateConfigName), []byte(""), 0644)
			for rel, content := range tt.files {
				full := filepath.Join(dest, filepath.FromSlash(rel))
				_ = os.MkdirAll(filepath.Dir(full), 0755)
				_ = os.WriteFile(full, []byte(content), 0644)
			}

			// When
			err := NewTemplateProcessor(NewRendererService()).ApplyTemplate(tt.config, dest)

			// Then
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("ApplyTemplate() error = %v, want error containing %q", err, tt.wantErr)
			}
			found := false
			_ = filepath.WalkDir(dest, func(p string, d os.DirEntry, _ error) error {
				if strings.EqualFold(d.Name(), ".git") {
					found = true
				}
				return nil
			})
			if found {
				t.Errorf("a .git directory was created")
			}
		})
	}
}
