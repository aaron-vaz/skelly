package commands

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aaron-vaz/skelly/internal/config"
	"github.com/aaron-vaz/skelly/internal/templates"
)

type mockDownloader struct {
	err    error
	source string
}

func (m *mockDownloader) Get(ctx context.Context, source string, destination string) error {
	m.source = source
	if m.err != nil {
		return m.err
	}
	// Create a dummy template in destination
	_ = os.MkdirAll(destination, 0755)
	configYaml := `
name: "test-template"
description: "A test template"
`
	_ = os.WriteFile(filepath.Join(destination, templates.TemplateConfigName), []byte(configYaml), 0644)
	_ = os.WriteFile(filepath.Join(destination, "{{.Name}}.txt"), []byte("hello {{.Name}}"), 0644)
	return nil
}

type mockUIReturns struct {
	questionAnswer string
	questionErr    error
}

type mockUIInit struct {
	returns   mockUIReturns
	infoMsgs  []string
	questions []string
}

func (m *mockUIInit) RenderInputs(ctx context.Context, inputs map[string]templates.Input) error {
	return nil
}
func (m *mockUIInit) RenderQuestion(ctx context.Context, question string, options []string) (string, error) {
	m.questions = append(m.questions, question)
	return m.returns.questionAnswer, m.returns.questionErr
}
func (m *mockUIInit) RenderInfo(message string) error {
	m.infoMsgs = append(m.infoMsgs, message)
	return nil
}
func (m *mockUIInit) RenderError(message string) error { return nil }

func TestInitCommand_Run(t *testing.T) {
	proc := templates.NewTemplateProcessor(templates.NewRendererService())

	tests := []struct {
		name       string
		downloader *mockDownloader
		answer     string
		flags      func(dest string) []string
		args       []string
		existing   bool // put a file in dest before running
		wantErr    string
		validate   func(t *testing.T, dest string, ui *mockUIInit)
	}{
		{
			name:       "missing src option",
			downloader: &mockDownloader{},
			flags:      func(dest string) []string { return []string{"-dst", dest} },
			wantErr:    "invalid options: source is required",
		},
		{
			name:       "unexpected extra arguments",
			downloader: &mockDownloader{},
			flags:      func(dest string) []string { return []string{"-dst", dest} },
			args:       []string{"src", "extra"},
			wantErr:    "invalid options: unexpected arguments: extra",
		},
		{
			name:       "downloader error leaves existing destination untouched",
			downloader: &mockDownloader{err: errors.New("download failed")},
			answer:     "y",
			flags:      func(dest string) []string { return []string{"-src", "http://example.com/t", "-dst", dest} },
			existing:   true,
			wantErr:    "failed to download template: download failed",
			validate: func(t *testing.T, dest string, ui *mockUIInit) {
				if _, err := os.Stat(filepath.Join(dest, "existing.txt")); err != nil {
					t.Errorf("Expected existing file to survive a failed download: %v", err)
				}
			},
		},
		{
			name:       "non-empty destination, user aborts",
			downloader: &mockDownloader{},
			answer:     "n",
			flags:      func(dest string) []string { return []string{"-src", "http://example.com/t", "-dst", dest} },
			existing:   true,
			validate: func(t *testing.T, dest string, ui *mockUIInit) {
				if len(ui.infoMsgs) == 0 || !strings.Contains(ui.infoMsgs[0], "Exiting....") {
					t.Errorf("Expected exiting message, got %v", ui.infoMsgs)
				}
				if _, err := os.Stat(filepath.Join(dest, "existing.txt")); err != nil {
					t.Errorf("Expected existing file to be kept: %v", err)
				}
			},
		},
		{
			name:       "non-empty destination, user overwrites",
			downloader: &mockDownloader{},
			answer:     "Y",
			flags:      func(dest string) []string { return []string{"-src", "http://example.com/t", "-dst", dest} },
			existing:   true,
			validate: func(t *testing.T, dest string, ui *mockUIInit) {
				if _, err := os.Stat(filepath.Join(dest, "existing.txt")); !os.IsNotExist(err) {
					t.Errorf("Expected existing file to be removed")
				}
				assertFile(t, filepath.Join(dest, "test-template.txt"), "hello test-template")
			},
		},
		{
			name:       "successful init into new directory without prompting",
			downloader: &mockDownloader{},
			flags:      func(dest string) []string { return []string{"-src", "http://example.com/t", "-dst", dest} },
			validate: func(t *testing.T, dest string, ui *mockUIInit) {
				if len(ui.questions) != 0 {
					t.Errorf("Expected no overwrite prompt, got %v", ui.questions)
				}
				assertFile(t, filepath.Join(dest, "test-template.txt"), "hello test-template")
				if _, err := os.Stat(filepath.Join(dest, templates.TemplateConfigName)); !os.IsNotExist(err) {
					t.Errorf("Expected %s to be removed", templates.TemplateConfigName)
				}
			},
		},
		{
			name:       "source given as positional argument",
			downloader: &mockDownloader{},
			flags:      func(dest string) []string { return []string{"-dst", dest} },
			args:       []string{"http://example.com/t"},
			validate: func(t *testing.T, dest string, ui *mockUIInit) {
				assertFile(t, filepath.Join(dest, "test-template.txt"), "hello test-template")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			dest := filepath.Join(t.TempDir(), "dest")
			if tt.existing {
				_ = os.MkdirAll(dest, 0755)
				_ = os.WriteFile(filepath.Join(dest, "existing.txt"), []byte("keep me"), 0644)
			}

			ui := &mockUIInit{returns: mockUIReturns{questionAnswer: tt.answer}}
			cmd := NewInitCommand(proc, tt.downloader, &mockStore{}, ui)
			flags := flag.NewFlagSet("init", flag.ContinueOnError)
			cmd.Init(flags)
			if err := flags.Parse(tt.flags(dest)); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			// When
			err := cmd.Run(context.Background(), tt.args)

			// Then
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Run() error = %v, want error containing %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("Run() error = %v, want nil", err)
			}

			// And
			if tt.validate != nil {
				tt.validate(t, dest, ui)
			}
		})
	}
}

func TestInitCommand_Run_RefusesToOverwriteHome(t *testing.T) {
	tests := []struct {
		name string
		dest func(t *testing.T, home string) string
	}{
		{
			name: "home directory",
			dest: func(t *testing.T, home string) string { return home },
		},
		{
			name: "symlink to home directory",
			dest: func(t *testing.T, home string) string {
				link := filepath.Join(t.TempDir(), "home-link")
				if err := os.Symlink(home, link); err != nil {
					t.Fatal(err)
				}
				return link
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			home := t.TempDir()
			t.Setenv("HOME", home)
			_ = os.WriteFile(filepath.Join(home, "precious.txt"), []byte("data"), 0644)

			ui := &mockUIInit{returns: mockUIReturns{questionAnswer: "y"}}
			cmd := NewInitCommand(templates.NewTemplateProcessor(templates.NewRendererService()), &mockDownloader{}, &mockStore{}, ui)
			flags := flag.NewFlagSet("init", flag.ContinueOnError)
			cmd.Init(flags)
			_ = flags.Parse([]string{"-src", "http://example.com/t", "-dst", tt.dest(t, home)})

			// When
			err := cmd.Run(context.Background(), nil)

			// Then
			if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
				t.Errorf("Run() error = %v, want refusal", err)
			}

			// And
			if _, err := os.Stat(filepath.Join(home, "precious.txt")); err != nil {
				t.Errorf("Expected home directory contents to be untouched: %v", err)
			}
		})
	}
}

func TestInitCommand_Run_ResolvesSource(t *testing.T) {
	store := &mockStore{cfg: config.Config{Sources: map[string]config.Source{
		"go": {URL: "github.com/user/go-template"},
	}}}

	tests := []struct {
		name       string
		source     string
		store      *mockStore
		wantSource string
		wantErr    string
	}{
		{name: "registered alias is replaced by its source", source: "go", store: store, wantSource: "github.com/user/go-template"},
		{name: "unregistered word is used as-is", source: "java", store: store, wantSource: "java"},
		{name: "url skips the config entirely", source: "github.com/user/t", store: &mockStore{loadErr: errors.New("broken")}, wantSource: "github.com/user/t"},
		{name: "config error is reported for an alias", source: "go", store: &mockStore{loadErr: errors.New("broken")}, wantErr: "broken"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			downloader := &mockDownloader{}
			ui := &mockUIInit{}
			cmd := NewInitCommand(templates.NewTemplateProcessor(templates.NewRendererService()), downloader, tt.store, ui)
			flags := flag.NewFlagSet("init", flag.ContinueOnError)
			cmd.Init(flags)
			_ = flags.Parse([]string{"-dst", filepath.Join(t.TempDir(), "dest")})

			// When
			err := cmd.Run(context.Background(), []string{tt.source})

			// Then
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Run() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Run() unexpected error: %v", err)
			}

			// And
			if downloader.source != tt.wantSource {
				t.Errorf("downloaded %q, want %q", downloader.source, tt.wantSource)
			}
		})
	}
}

func assertFile(t *testing.T, path string, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("Expected file %s to exist: %v", path, err)
		return
	}
	if string(content) != want {
		t.Errorf("File %s = %q, want %q", path, string(content), want)
	}
}
