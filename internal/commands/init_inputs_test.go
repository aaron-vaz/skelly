package commands

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aaron-vaz/skelly/internal/templates"
)

// fileDownloader writes a template made of the given files, so tests control the config.
type fileDownloader struct {
	files map[string]string
}

func (f *fileDownloader) Get(ctx context.Context, source string, destination string) error {
	for rel, content := range f.files {
		full := filepath.Join(destination, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// askingUI answers the inputs it is asked for from a fixed set of answers.
type askingUI struct {
	mockUIInit
	answers map[string]string
	asked   []string
}

func (a *askingUI) RenderInputs(ctx context.Context, inputs map[string]templates.Input) error {
	for name, input := range inputs {
		a.asked = append(a.asked, name)
		input.Value = a.answers[name]
		inputs[name] = input
	}
	slices.Sort(a.asked)
	return nil
}

const kindTemplate = `
name: kinds
inputs:
  name: {description: Project name}
  kind: {description: What to make, default: cli, choices: [cli, service]}
  owner: {description: Who owns it, default: me}
`

func kindFiles() map[string]string {
	return map[string]string{
		templates.TemplateConfigName: kindTemplate,
		"out.txt":                    "{{.Inputs.name.Value}} {{.Inputs.kind.Value}} {{.Inputs.owner.Value}}",
	}
}

func TestInitCommand_Run_Inputs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		existing  bool
		prompted  map[string]string
		wantErr   string
		wantFile  string
		wantAsked []string
		validate  func(t *testing.T, dest string, ui *askingUI)
	}{
		{
			name:      "answers given with -set are not asked for again",
			args:      []string{"-set", "name=app", "-set", "kind=service"},
			prompted:  map[string]string{"owner": "you"},
			wantFile:  "app service you",
			wantAsked: []string{"owner"},
		},
		{
			name:     "-no-input takes defaults for everything unanswered",
			args:     []string{"-no-input", "-set", "name=app"},
			wantFile: "app cli me",
		},
		{
			name:    "-no-input fails naming every required input without a default",
			args:    []string{"-no-input"},
			wantErr: "missing required inputs: name (pass them with -set name=value)",
		},
		{
			name:    "unknown input is rejected with the known ones listed",
			args:    []string{"-no-input", "-set", "nmae=x"},
			wantErr: `unknown input "nmae" (the template has: kind, name, owner)`,
		},
		{
			name:    "value outside the choices is rejected",
			args:    []string{"-no-input", "-set", "name=app", "-set", "kind=worker"},
			wantErr: `input "kind" must be one of: cli, service`,
		},
		{
			name:    "empty value is rejected",
			args:    []string{"-no-input", "-set", "name="},
			wantErr: `input "name" must not be empty`,
		},
		{
			name:     "-no-input refuses a destination that is not empty",
			args:     []string{"-no-input", "-set", "name=app"},
			existing: true,
			wantErr:  "is not empty, pass -yes to replace its contents",
			validate: func(t *testing.T, dest string, ui *askingUI) {
				assertFile(t, filepath.Join(dest, "existing.txt"), "keep me")
			},
		},
		{
			name:     "-yes replaces a destination that is not empty without asking",
			args:     []string{"-no-input", "-yes", "-set", "name=app"},
			existing: true,
			wantFile: "app cli me",
			validate: func(t *testing.T, dest string, ui *askingUI) {
				if len(ui.questions) != 0 {
					t.Errorf("Expected no question, got %v", ui.questions)
				}
				if _, err := os.Stat(filepath.Join(dest, "existing.txt")); !os.IsNotExist(err) {
					t.Errorf("Expected the existing file to be replaced")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			dest := filepath.Join(t.TempDir(), "dest")
			if tt.existing {
				_ = os.MkdirAll(dest, 0o755)
				_ = os.WriteFile(filepath.Join(dest, "existing.txt"), []byte("keep me"), 0o644)
			}
			ui := &askingUI{answers: tt.prompted}
			cmd := NewInitCommand(templates.NewTemplateProcessor(templates.NewRendererService()), &fileDownloader{files: kindFiles()}, &mockStore{}, ui)
			flags := flag.NewFlagSet("init", flag.ContinueOnError)
			cmd.Init(flags)
			if err := flags.Parse(append([]string{"-dst", dest}, tt.args...)); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			// When
			err := cmd.Run(context.Background(), []string{"http://example.com/t"})

			// Then
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Run() error = %v, want error containing %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if tt.wantFile != "" {
				assertFile(t, filepath.Join(dest, "out.txt"), tt.wantFile)
			}
			if tt.wantAsked != nil && !slices.Equal(ui.asked, tt.wantAsked) {
				t.Errorf("asked for %v, want %v", ui.asked, tt.wantAsked)
			}

			// And
			if tt.validate != nil {
				tt.validate(t, dest, ui)
			}
		})
	}
}

func TestInitCommand_Run_JSON(t *testing.T) {
	// Given
	dest := filepath.Join(t.TempDir(), "dest")
	ui := &askingUI{}
	cmd := NewInitCommand(templates.NewTemplateProcessor(templates.NewRendererService()), &fileDownloader{files: kindFiles()}, &mockStore{}, ui)
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	cmd.Init(flags)
	if err := flags.Parse([]string{"-json", "-dst", dest, "-set", "name=app"}); err != nil {
		t.Fatal(err)
	}

	// When
	err := cmd.Run(context.Background(), []string{"http://user:secret@example.com/t"})

	// Then
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(ui.asked) != 0 || len(ui.questions) != 0 {
		t.Errorf("Expected -json to imply -no-input, asked %v and %v", ui.asked, ui.questions)
	}

	// And
	if len(ui.infoMsgs) != 1 {
		t.Fatalf("Expected a single JSON message, got %q", ui.infoMsgs)
	}
	var got initResult
	if err := json.Unmarshal([]byte(ui.infoMsgs[0]), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, ui.infoMsgs[0])
	}
	if got.Destination != dest {
		t.Errorf("destination = %q, want %q", got.Destination, dest)
	}
	if strings.Contains(got.Source, "secret") {
		t.Errorf("source leaks credentials: %q", got.Source)
	}
	if got.Inputs["name"] != "app" || got.Inputs["kind"] != "cli" || got.Inputs["owner"] != "me" {
		t.Errorf("inputs = %v", got.Inputs)
	}
}

func TestInitCommand_Run_SetWithoutConfig(t *testing.T) {
	// Given
	ui := &askingUI{}
	cmd := NewInitCommand(templates.NewTemplateProcessor(templates.NewRendererService()), &fileDownloader{files: map[string]string{"a.txt": "x"}}, &mockStore{}, ui)
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	cmd.Init(flags)
	_ = flags.Parse([]string{"-dst", filepath.Join(t.TempDir(), "dest"), "-set", "name=app"})

	// When
	err := cmd.Run(context.Background(), []string{"http://example.com/t"})

	// Then
	if err == nil || !strings.Contains(err.Error(), "takes no inputs to -set") {
		t.Errorf("Run() error = %v, want a no-inputs error", err)
	}
}

func TestAnswerFlag_Set(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "name and value", raw: "a=b", want: "a=b"},
		{name: "value may contain equals signs", raw: "url=http://x/?a=b", want: "url=http://x/?a=b"},
		{name: "no equals sign", raw: "a", wantErr: true},
		{name: "blank name", raw: "=b", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			var answers answerFlag

			// When
			err := answers.Set(tt.raw)

			// Then
			if (err != nil) != tt.wantErr {
				t.Fatalf("Set() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got := answers.String(); got != tt.want && !tt.wantErr {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
