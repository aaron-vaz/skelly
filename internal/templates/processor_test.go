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
