package commands

import (
	"context"
	"flag"
	"strings"
	"testing"
)

type mockCommand struct {
	name        string
	description string
	initFunc    func(flags *flag.FlagSet)
	runFunc     func(args []string) error
}

func (m *mockCommand) Name() string        { return m.name }
func (m *mockCommand) Description() string { return m.description }
func (m *mockCommand) Init(flags *flag.FlagSet) {
	if m.initFunc != nil {
		m.initFunc(flags)
	}
}
func (m *mockCommand) Run(ctx context.Context, args []string) error {
	if m.runFunc != nil {
		return m.runFunc(args)
	}
	return nil
}

func TestHelpCommand_Run(t *testing.T) {
	mockCmds := map[string]Command{
		"dummy": &mockCommand{
			name:        "dummy",
			description: "A dummy command",
			initFunc: func(flags *flag.FlagSet) {
				flags.String("foo", "", "dummy flag")
			},
		},
	}

	tests := []struct {
		name     string
		args     []string
		wantErr  bool
		wantText string
	}{
		{
			name:     "no args shows help",
			args:     []string{},
			wantErr:  false,
			wantText: "USAGE:",
		},
		{
			name:     "known command shows help",
			args:     []string{"dummy"},
			wantErr:  false,
			wantText: "dummy flag",
		},
		{
			name:     "unknown command shows error",
			args:     []string{"unknown"},
			wantErr:  true,
			wantText: "USAGE:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			ui := &mockUI{}
			cmd := NewHelpCommand(mockCmds, ui)

			// When
			err := cmd.Run(context.Background(), tt.args)

			// Then
			if tt.wantErr {
				if err == nil {
					t.Errorf("Run() error = nil, wantErr %v", tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				}
			}

			// And
			if len(ui.infoMsgs) == 0 || !strings.Contains(ui.infoMsgs[0], tt.wantText) {
				t.Errorf("Expected output containing %q, got %v", tt.wantText, ui.infoMsgs)
			}
		})
	}
}

func TestHelpCommand_Run_SortedCommands(t *testing.T) {
	// Given
	cmds := map[string]Command{}
	for _, name := range []string{"zeta", "alpha", "mid", "beta", "omega"} {
		cmds[name] = &mockCommand{name: name}
	}
	ui := &mockUI{}
	cmd := NewHelpCommand(cmds, ui)

	// When
	err := cmd.Run(context.Background(), nil)

	// Then
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// And
	out := ui.infoMsgs[0]
	last := -1
	for _, name := range []string{"alpha", "beta", "mid", "omega", "zeta"} {
		idx := strings.Index(out, "    "+name)
		if idx < last {
			t.Fatalf("Expected commands in alphabetical order, got:\n%s", out)
		}
		last = idx
	}
}
