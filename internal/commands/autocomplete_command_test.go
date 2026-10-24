package commands

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/aaron-vaz/skelly/internal/autocomplete"
	"github.com/aaron-vaz/skelly/internal/config"
)

// completionCommands builds the commands the way the invoker does, without what they need to run.
func completionCommands(store config.Store, ui *mockUI) map[string]Command {
	cmds := map[string]Command{}
	for _, cmd := range []Command{
		NewInitCommand(nil, nil, store, ui),
		NewInspectCommand(nil, nil, store, ui),
		NewSourcesCommand(store, ui),
		NewVersionCommand(ui),
	} {
		cmds[cmd.Name()] = cmd
	}
	cmds["help"] = NewHelpCommand(cmds, ui)
	cmds["autocomplete"] = NewAutocompleteCommand(cmds, store, ui)
	return cmds
}

func findCmd(t *testing.T, cmds []autocomplete.Cmd, name string) autocomplete.Cmd {
	t.Helper()
	for _, c := range cmds {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no command %q in %v", name, cmds)
	return autocomplete.Cmd{}
}

func findFlag(t *testing.T, flags []autocomplete.Flag, name string) autocomplete.Flag {
	t.Helper()
	for _, f := range flags {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("no flag %q in %v", name, flags)
	return autocomplete.Flag{}
}

func TestBuildCompletionSpec_ReadsTheCommands(t *testing.T) {
	// Given the commands of the program
	spec := BuildCompletionSpec(completionCommands(&mockStore{}, &mockUI{}))

	// When reading init
	init := findCmd(t, spec.Commands, "init")

	// Then its flags are the ones it registers, with whether they take a value
	for name, takesValue := range map[string]bool{"src": true, "dst": true, "set": true, "yes": false, "no-input": false, "json": false} {
		if got := findFlag(t, init.Flags, name).TakesValue; got != takesValue {
			t.Errorf("init -%s takes a value = %v, want %v", name, got, takesValue)
		}
	}

	// And what their values are completed with comes from the hints
	if got := findFlag(t, init.Flags, "dst").Value; got != autocomplete.Dir {
		t.Errorf("init -dst is completed as %v, want directories", got)
	}
	if got := findFlag(t, init.Flags, "src").Value; got != autocomplete.Source {
		t.Errorf("init -src is completed as %v, want sources", got)
	}

	// And a group lists its subcommands
	var subs []string
	for _, sub := range findCmd(t, spec.Commands, "sources").Subcommands {
		subs = append(subs, sub.Name)
	}
	if !slices.Equal(subs, []string{"add", "list", "remove"}) {
		t.Errorf("sources has subcommands %v", subs)
	}

	// And so does autocomplete, which is itself completed
	subs = nil
	for _, sub := range findCmd(t, spec.Commands, "autocomplete").Subcommands {
		subs = append(subs, sub.Name)
	}
	if !slices.Equal(subs, []string{"bash", "complete", "fish", "zsh"}) {
		t.Errorf("autocomplete has subcommands %v", subs)
	}
}

func TestCompletionHints_NameWhatExists(t *testing.T) {
	// Given the hints and the commands they describe
	spec := BuildCompletionSpec(completionCommands(&mockStore{}, &mockUI{}))

	for path, hint := range completionHints {
		t.Run(path, func(t *testing.T) {
			// When looking the command up by its full name
			cmd := findCmd(t, spec.Commands, strings.Fields(path)[0])
			for _, name := range strings.Fields(path)[1:] {
				cmd = findCmd(t, cmd.Subcommands, name)
			}

			// Then every flag a hint names exists, so renaming a flag fails here
			for name := range hint.flags {
				findFlag(t, cmd.Flags, name)
			}
			if len(cmd.Args) != len(hint.args) {
				t.Errorf("%s has %d hinted arguments", path, len(cmd.Args))
			}
		})
	}
}

func TestAutocompleteShellCommand(t *testing.T) {
	for _, shell := range autocomplete.Shells() {
		t.Run(shell, func(t *testing.T) {
			// Given the command of a shell
			ui := &mockUI{}
			cmd := &AutocompleteShellCommand{shell: shell, ui: ui}

			// When it runs
			err := cmd.Run(context.Background(), nil)

			// Then it prints the script of that shell, once, without a blank last line
			want, _ := autocomplete.Script(shell)
			if err != nil || len(ui.infoMsgs) != 1 || ui.infoMsgs[0] != strings.TrimSuffix(want, "\n") {
				t.Errorf("err = %v, printed %q", err, ui.infoMsgs)
			}
		})
	}

	t.Run("arguments are refused", func(t *testing.T) {
		cmd := &AutocompleteShellCommand{shell: "bash", ui: &mockUI{}}
		if err := cmd.Run(context.Background(), []string{"zsh"}); err == nil {
			t.Error("expected an error for an extra argument")
		}
	})
}

func TestAutocompleteCompleteCommand(t *testing.T) {
	registered := &mockStore{cfg: config.Config{Sources: map[string]config.Source{
		"kotlin": {URL: "github.com/aaron-vaz/skelly-template-kotlin"},
		"go":     {URL: "github.com/aaron-vaz/skelly-template-go"},
	}}}

	tests := []struct {
		name  string
		store *mockStore
		args  []string
		want  string
	}{
		{"registered aliases, sorted, and files", registered, []string{"init", ""}, "go\tregistered template\nkotlin\tregistered template\n:files"},
		{"flags", registered, []string{"inspect", "-s"}, "-src\tURL, path or registered alias of the template to inspect\n:none"},
		{"an unreadable config offers no aliases", &mockStore{loadErr: errors.New("broken")}, []string{"init", ""}, ":files"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given the commands and a config
			ui := &mockUI{}
			cmd := completionCommands(tt.store, ui)["autocomplete"].(*Group)
			complete, _ := cmd.Subcommand("complete")

			// When completing the words
			err := complete.Run(context.Background(), tt.args)

			// Then the candidates are printed for the shell script
			if err != nil || len(ui.infoMsgs) != 1 || ui.infoMsgs[0] != tt.want {
				t.Errorf("err = %v, printed %q, want %q", err, ui.infoMsgs, tt.want)
			}
		})
	}
}
