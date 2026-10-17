package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/aaron-vaz/skelly/internal/config"
)

func TestSourcesRemoveCommand_Run(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantErr   string
		wantAlias bool
	}{
		{name: "removes a registered alias", args: []string{"go"}},
		{name: "unknown alias", args: []string{"java"}, wantErr: "not registered", wantAlias: true},
		{name: "no alias", args: nil, wantErr: "expected a single alias", wantAlias: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			store := &mockStore{cfg: config.Config{Sources: map[string]config.Source{"go": {URL: "github.com/user/go-template"}}}}
			cmd := NewSourcesRemoveCommand(store, &mockUI{})

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
			if _, ok := store.cfg.Sources["go"]; ok != tt.wantAlias {
				t.Errorf("alias go registered = %v, want %v", ok, tt.wantAlias)
			}
		})
	}
}
