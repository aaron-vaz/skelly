package commands

import (
	"context"
	"testing"

	"github.com/aaron-vaz/skelly/internal/config"
)

func TestSourcesListCommand_Run(t *testing.T) {
	tests := []struct {
		name    string
		sources map[string]config.Source
		want    string
	}{
		{
			name: "no sources",
			want: "No templates registered, add one with: skelly sources add <alias> <src>",
		},
		{
			name: "sources are sorted and aligned",
			sources: map[string]config.Source{
				"python": {URL: "github.com/user/py"},
				"go":     {URL: "github.com/user/go"},
			},
			want: "go      github.com/user/go\npython  github.com/user/py",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			ui := &mockUI{}
			cmd := NewSourcesListCommand(&mockStore{cfg: config.Config{Sources: tt.sources}}, ui)

			// When
			err := cmd.Run(context.Background(), nil)

			// Then
			if err != nil {
				t.Fatalf("Run() unexpected error: %v", err)
			}

			// And
			if len(ui.infoMsgs) != 1 || ui.infoMsgs[0] != tt.want {
				t.Errorf("output = %q, want %q", ui.infoMsgs, tt.want)
			}
		})
	}
}
