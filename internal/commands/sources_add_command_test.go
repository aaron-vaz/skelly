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
)

func TestSourcesAddCommand_Run(t *testing.T) {
	existing := map[string]config.Source{"go": {URL: "github.com/user/go-template"}}

	tests := []struct {
		name     string
		flags    []string
		args     []string
		store    *mockStore
		wantErr  string
		wantURLs map[string]string
	}{
		{
			name:     "registers a new alias",
			args:     []string{"java", "github.com/user/java-template"},
			store:    &mockStore{},
			wantURLs: map[string]string{"java": "github.com/user/java-template"},
		},
		{
			name:    "missing source",
			args:    []string{"java"},
			store:   &mockStore{},
			wantErr: "expected an alias and a source",
		},
		{
			name:    "blank source",
			args:    []string{"java", "  "},
			store:   &mockStore{},
			wantErr: "source is required",
		},
		{
			name:    "alias that looks like a path",
			args:    []string{"./java", "github.com/user/java-template"},
			store:   &mockStore{},
			wantErr: "invalid alias",
		},
		{
			name:     "existing alias is not replaced without force",
			args:     []string{"go", "github.com/other/go-template"},
			store:    &mockStore{cfg: config.Config{Sources: existing}},
			wantErr:  "already registered",
			wantURLs: map[string]string{"go": "github.com/user/go-template"},
		},
		{
			name:     "existing alias is replaced with force",
			flags:    []string{"-force"},
			args:     []string{"go", "github.com/other/go-template"},
			store:    &mockStore{cfg: config.Config{Sources: existing}},
			wantURLs: map[string]string{"go": "github.com/other/go-template"},
		},
		{
			name:     "re-registering the same source is fine",
			args:     []string{"go", "github.com/user/go-template"},
			store:    &mockStore{cfg: config.Config{Sources: existing}},
			wantURLs: map[string]string{"go": "github.com/user/go-template"},
		},
		{
			name:    "config load error",
			args:    []string{"go", "github.com/user/go-template"},
			store:   &mockStore{loadErr: errors.New("broken config")},
			wantErr: "broken config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			ui := &mockUI{}
			cmd := NewSourcesAddCommand(tt.store, ui)
			flags := flag.NewFlagSet("add", flag.ContinueOnError)
			cmd.Init(flags)
			if err := flags.Parse(tt.flags); err != nil {
				t.Fatal(err)
			}

			// When
			err := cmd.Run(context.Background(), tt.args)

			// Then
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Run() error = %v, want %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("Run() unexpected error: %v", err)
			}

			// And
			if len(tt.store.cfg.Sources) != len(tt.wantURLs) {
				t.Errorf("stored sources = %v, want %v", tt.store.cfg.Sources, tt.wantURLs)
			}
			for alias, url := range tt.wantURLs {
				if got := tt.store.cfg.Sources[alias].URL; got != url {
					t.Errorf("source for %s = %q, want %q", alias, got, url)
				}
			}
		})
	}
}

func TestSourcesAddCommand_Run_RelativeLocalPath(t *testing.T) {
	// Given
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "tmpl"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	store := &mockStore{}
	cmd := NewSourcesAddCommand(store, &mockUI{})
	cmd.Init(flag.NewFlagSet("add", flag.ContinueOnError))

	// When
	err := cmd.Run(context.Background(), []string{"local", "tmpl"})

	// Then
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	// And
	got := store.cfg.Sources["local"].URL
	if !filepath.IsAbs(got) || filepath.Base(got) != "tmpl" {
		t.Errorf("Expected an absolute path to tmpl, got %q", got)
	}
}
