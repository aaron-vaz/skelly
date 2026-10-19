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
	// calls holds the names asked for in each call, one call per layer of inputs
	calls [][]string
	// seen holds the inputs as they were shown, with their rendered text
	seen map[string]templates.Input
}

// RenderInputs answers from the fixed answers, and accepts the default for the rest.
func (a *askingUI) RenderInputs(ctx context.Context, inputs map[string]templates.Input) error {
	if a.seen == nil {
		a.seen = map[string]templates.Input{}
	}

	var call []string
	for name, input := range inputs {
		call = append(call, name)
		a.asked = append(a.asked, name)
		a.seen[name] = input

		if answer, ok := a.answers[name]; ok {
			input.Value = answer
		} else {
			input.Value = input.Default
		}
		inputs[name] = input
	}
	slices.Sort(call)
	a.calls = append(a.calls, call)
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

const derivedTemplate = `
name: derived
inputs:
  name: {description: Project name}
  owner: {description: Who owns it, default: me}
  module:
    description: 'Module of {{.Inputs.name.Value}}'
    default: 'github.com/{{.Inputs.owner.Value}}/{{.Inputs.name.Value}}'
  image: {default: 'ghcr.io/{{.Inputs.module.Value}}'}
env:
  tag: 'v-{{.Inputs.name.Value}}'
`

func derivedFiles() map[string]string {
	return map[string]string{
		templates.TemplateConfigName: derivedTemplate,
		"out.txt":                    "{{.Inputs.module.Value}} {{.Inputs.image.Value}} {{.Env.tag}}",
	}
}

func TestInitCommand_Run_DerivedInputs(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string]string
		args      []string
		prompted  map[string]string
		wantErr   []string
		wantFile  string
		wantCalls [][]string
		validate  func(t *testing.T, ui *askingUI)
	}{
		{
			name:      "dependent inputs are asked after the ones they use, with their rendered defaults",
			args:      []string{},
			prompted:  map[string]string{"name": "app", "owner": "you"},
			wantFile:  "github.com/you/app ghcr.io/github.com/you/app v-app",
			wantCalls: [][]string{{"name", "owner"}, {"module"}, {"image"}},
			validate: func(t *testing.T, ui *askingUI) {
				if got := ui.seen["module"]; got.Default != "github.com/you/app" || got.Description != "Module of app" {
					t.Errorf("module was shown as %+v", got)
				}
			},
		},
		{
			name:     "-no-input renders defaults from the defaults of the inputs they use",
			args:     []string{"-no-input", "-set", "name=app"},
			wantFile: "github.com/me/app ghcr.io/github.com/me/app v-app",
		},
		{
			name:      "-set answers feed the defaults that use them, and are not asked for",
			args:      []string{"-set", "name=app", "-set", "owner=you"},
			wantFile:  "github.com/you/app ghcr.io/github.com/you/app v-app",
			wantCalls: [][]string{{"module"}, {"image"}},
		},
		{
			name:     "an answer given for a dependent input wins over its default",
			args:     []string{"-no-input", "-set", "name=app", "-set", "module=example.com/app"},
			wantFile: "example.com/app ghcr.io/example.com/app v-app",
		},
		{
			name:    "a missing required input is reported once, without errors for what depends on it",
			args:    []string{"-no-input"},
			wantErr: []string{"missing required inputs: name (pass them with -set name=value)"},
			validate: func(t *testing.T, ui *askingUI) {
				if len(ui.asked) != 0 {
					t.Errorf("asked for %v", ui.asked)
				}
			},
		},
		{
			name: "inputs that use each other are rejected when the template is loaded",
			files: map[string]string{
				templates.TemplateConfigName: "inputs:\n  a: {default: '{{.Inputs.b.Value}}'}\n  b: {default: '{{.Inputs.a.Value}}'}",
			},
			args:    []string{"-no-input"},
			wantErr: []string{"inputs use each other in a cycle, involving: a, b"},
		},
		{
			name: "templated choices are rendered and checked against answers",
			files: map[string]string{
				templates.TemplateConfigName: "inputs:\n  base: {default: cli}\n  kind: {choices: ['{{.Inputs.base.Value}}', other]}",
			},
			args:    []string{"-no-input", "-set", "kind=service"},
			wantErr: []string{`input "kind" must be one of: cli, other`},
		},
		{
			name: "an input whose template cannot be rendered is reported with its name",
			files: map[string]string{
				templates.TemplateConfigName: "inputs:\n  a: {default: x}\n  b: {default: '{{.Env.nope}}'}",
			},
			args:    []string{"-no-input"},
			wantErr: []string{`input "b": default:`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			files := tt.files
			if files == nil {
				files = derivedFiles()
			}
			dest := filepath.Join(t.TempDir(), "dest")
			ui := &askingUI{answers: tt.prompted}
			cmd := NewInitCommand(templates.NewTemplateProcessor(templates.NewRendererService()), &fileDownloader{files: files}, &mockStore{}, ui)
			flags := flag.NewFlagSet("init", flag.ContinueOnError)
			cmd.Init(flags)
			if err := flags.Parse(append([]string{"-dst", dest}, tt.args...)); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			// When
			err := cmd.Run(context.Background(), []string{"http://example.com/t"})

			// Then
			if len(tt.wantErr) > 0 {
				if err == nil {
					t.Fatalf("Run() error = nil, want %v", tt.wantErr)
				}
				for _, want := range tt.wantErr {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("Run() error = %q, want it to contain %q", err, want)
					}
				}
			} else if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if tt.wantFile != "" {
				assertFile(t, filepath.Join(dest, "out.txt"), tt.wantFile)
			}
			if tt.wantCalls != nil && !slices.EqualFunc(ui.calls, tt.wantCalls, slices.Equal[[]string]) {
				t.Errorf("asked in %v, want %v", ui.calls, tt.wantCalls)
			}

			// And
			if tt.validate != nil {
				tt.validate(t, ui)
			}
		})
	}
}
