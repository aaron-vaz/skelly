package commands

import (
	"context"
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	ui := &mockUI{}
	add := NewSourcesAddCommand(&mockStore{}, ui)
	group := NewGroup("sources", "Manage sources", ui, add)

	tests := []struct {
		name     string
		cmd      Command
		args     []string
		wantCmd  Command
		wantPath string
		wantArgs []string
	}{
		{name: "subcommand", cmd: group, args: []string{"add", "go", "url"}, wantCmd: add, wantPath: "sources add", wantArgs: []string{"go", "url"}},
		{name: "group without subcommand", cmd: group, args: nil, wantCmd: group, wantPath: "sources"},
		{name: "unknown subcommand stays on group", cmd: group, args: []string{"nope"}, wantCmd: group, wantPath: "sources", wantArgs: []string{"nope"}},
		{name: "plain command ignores args", cmd: add, args: []string{"add"}, wantCmd: add, wantPath: "add", wantArgs: []string{"add"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			cmd, path, args := Resolve(tt.cmd, tt.args)

			// Then
			if cmd != tt.wantCmd || path != tt.wantPath || strings.Join(args, " ") != strings.Join(tt.wantArgs, " ") {
				t.Errorf("Resolve() = (%s, %q, %v), want (%s, %q, %v)", cmd.Name(), path, args, tt.wantCmd.Name(), tt.wantPath, tt.wantArgs)
			}
		})
	}
}

func TestGroup_Run(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "no subcommand shows usage"},
		{name: "unknown subcommand", args: []string{"nope"}, wantErr: "unknown command: sources nope"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			ui := &mockUI{}
			group := NewGroup("sources", "Manage sources", ui, NewSourcesListCommand(&mockStore{}, ui))

			// When
			err := group.Run(context.Background(), tt.args)

			// Then
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Errorf("Run() error = %v, want %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("Run() unexpected error: %v", err)
			}

			// And
			if len(ui.infoMsgs) != 1 || !strings.Contains(ui.infoMsgs[0], "Usage: skelly sources COMMAND") || !strings.Contains(ui.infoMsgs[0], "list") {
				t.Errorf("Expected group usage, got %q", ui.infoMsgs)
			}
		})
	}
}
