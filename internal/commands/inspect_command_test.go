package commands

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/aaron-vaz/skelly/internal/config"
	"github.com/aaron-vaz/skelly/internal/templates"
)

func TestInspectCommand_Run(t *testing.T) {
	const config = `
name: kinds
description: Makes things
inputs:
  name: {description: Project name}
  kind: {description: What to make, default: cli, choices: [cli, service]}
raw: [.github]
exclude:
  - path: a
    when: "true"
  - paths: [b, c]
    when: '{{eq .Inputs.kind.Value "cli"}}'
`

	tests := []struct {
		name     string
		files    map[string]string
		args     []string
		wantErr  string
		validate func(t *testing.T, got inspectResult, raw string)
	}{
		{
			name:  "describes inputs, raw and exclude",
			files: map[string]string{templates.TemplateConfigName: config},
			args:  []string{"http://user:secret@example.com/t"},
			validate: func(t *testing.T, got inspectResult, raw string) {
				if !got.HasConfig || got.Name != "kinds" || got.Description != "Makes things" {
					t.Errorf("unexpected header: %+v", got)
				}
				if strings.Contains(got.Source, "secret") {
					t.Errorf("source leaks credentials: %q", got.Source)
				}
				if len(got.Inputs) != 2 || got.Inputs[0].Name != "kind" || got.Inputs[1].Name != "name" {
					t.Fatalf("inputs should be sorted by name: %+v", got.Inputs)
				}
				if got.Inputs[0].Required || got.Inputs[0].Default != "cli" || len(got.Inputs[0].Choices) != 2 {
					t.Errorf("kind = %+v", got.Inputs[0])
				}
				if !got.Inputs[1].Required {
					t.Errorf("an input without a default must be required: %+v", got.Inputs[1])
				}
				if len(got.Raw) != 1 || got.Raw[0] != ".github" {
					t.Errorf("raw = %v", got.Raw)
				}
				if len(got.Exclude) != 2 || len(got.Exclude[1].Paths) != 2 || got.Exclude[0].When != "true" {
					t.Errorf("exclude = %+v", got.Exclude)
				}
			},
		},
		{
			name:  "template without a config has no inputs, as empty lists",
			files: map[string]string{"a.txt": "x"},
			args:  []string{"-src", "http://example.com/t"},
			validate: func(t *testing.T, got inspectResult, raw string) {
				if got.HasConfig {
					t.Errorf("HasConfig = true, want false")
				}
				if !strings.Contains(raw, `"inputs": []`) {
					t.Errorf("lists should be empty, not null: %s", raw)
				}
			},
		},
		{
			name:    "source is required",
			files:   map[string]string{},
			wantErr: "source is required",
		},
		{
			name:    "extra arguments are rejected",
			files:   map[string]string{},
			args:    []string{"src", "extra"},
			wantErr: "unexpected arguments: extra",
		},
		{
			name:    "invalid config is reported",
			files:   map[string]string{templates.TemplateConfigName: "inputs:\n  a: {default: [x]}"},
			args:    []string{"http://example.com/t"},
			wantErr: "failed to process template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			ui := &mockUIInit{}
			cmd := NewInspectCommand(templates.NewTemplateProcessor(templates.NewRendererService()), &fileDownloader{files: tt.files}, &mockStore{}, ui)
			flags := flag.NewFlagSet("inspect", flag.ContinueOnError)
			cmd.Init(flags)
			args := tt.args
			if len(args) > 0 && args[0] == "-src" {
				_ = flags.Parse(args[:2])
				args = args[2:]
			}

			// When
			err := cmd.Run(context.Background(), args)

			// Then
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Run() error = %v, want error containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}

			// And
			if len(ui.infoMsgs) != 1 {
				t.Fatalf("Expected a single JSON message, got %q", ui.infoMsgs)
			}
			var got inspectResult
			if err := json.Unmarshal([]byte(ui.infoMsgs[0]), &got); err != nil {
				t.Fatalf("output is not JSON: %v\n%s", err, ui.infoMsgs[0])
			}
			tt.validate(t, got, ui.infoMsgs[0])
		})
	}
}

func TestInspectCommand_Run_ResolvesAlias(t *testing.T) {
	// Given
	store := &mockStore{cfg: config.Config{Sources: map[string]config.Source{"go": {URL: "github.com/user/go-template"}}}}
	downloader := &recordingDownloader{}
	cmd := NewInspectCommand(templates.NewTemplateProcessor(templates.NewRendererService()), downloader, store, &mockUIInit{})

	// When
	err := cmd.Run(context.Background(), []string{"go"})

	// Then
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if downloader.source != "github.com/user/go-template" {
		t.Errorf("downloaded %q, want the alias's source", downloader.source)
	}
}

func TestInspectCommand_Run_DownloadError(t *testing.T) {
	// Given
	cmd := NewInspectCommand(templates.NewTemplateProcessor(templates.NewRendererService()), &mockDownloader{err: errors.New("boom")}, &mockStore{}, &mockUIInit{})

	// When
	err := cmd.Run(context.Background(), []string{"http://example.com/t"})

	// Then
	if err == nil || !strings.Contains(err.Error(), "failed to download template: boom") {
		t.Errorf("Run() error = %v", err)
	}
}

// recordingDownloader remembers the source and creates an empty template.
type recordingDownloader struct {
	source string
}

func (r *recordingDownloader) Get(ctx context.Context, source string, destination string) error {
	r.source = source
	return (&fileDownloader{files: map[string]string{"a.txt": "x"}}).Get(ctx, source, destination)
}
