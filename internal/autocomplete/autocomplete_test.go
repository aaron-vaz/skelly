package autocomplete

import (
	"slices"
	"strings"
	"testing"
)

func testSpec() Spec {
	return Spec{Commands: []Cmd{
		{
			Name:        "init",
			Description: "Initialize a new project from a template",
			Flags: []Flag{
				{Name: "dst", Usage: "Destination dir (default is the current one)", TakesValue: true, Value: Dir},
				{Name: "json", Usage: "Print JSON"},
				{Name: "set", Usage: "Answer an input as name=value", TakesValue: true},
				{Name: "src", Usage: "URL, path or alias", TakesValue: true, Value: Source},
				{Name: "yes", Usage: "Replace without asking"},
			},
			Args: []Kind{Source},
		},
		{Name: "inspect", Description: "Show a template's inputs", Args: []Kind{Source}},
		{
			Name:        "sources",
			Description: "Manage template sources",
			Subcommands: []Cmd{
				{Name: "add", Description: "Register", Flags: []Flag{{Name: "force"}}, Args: []Kind{None, File}},
				{Name: "list", Description: "List"},
				{Name: "remove", Description: "Remove", Args: []Kind{Alias}},
			},
		},
		{Name: "help", Description: "Show help", Args: []Kind{Command}},
		{Name: "version", Description: "Show version"},
	}}
}

func testAliases() []string {
	return []string{"go", "kotlin"}
}

func values(r Result) []string {
	var out []string
	for _, c := range r.Candidates {
		out = append(out, c.Value)
	}
	return out
}

func TestComplete(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		want      []string
		directive Directive
	}{
		{"commands", []string{""}, []string{"init", "inspect", "sources", "help", "version"}, NoFiles},
		{"no words at all", nil, []string{"init", "inspect", "sources", "help", "version"}, NoFiles},
		{"command by prefix", []string{"in"}, []string{"init", "inspect"}, NoFiles},
		{"unknown command", []string{"nope", ""}, nil, NoFiles},
		{"subcommands", []string{"sources", ""}, []string{"add", "list", "remove"}, NoFiles},
		{"subcommand by prefix", []string{"sources", "r"}, []string{"remove"}, NoFiles},
		{"first argument is a source", []string{"init", ""}, []string{"go", "kotlin"}, Files},
		{"alias by prefix", []string{"init", "k"}, []string{"kotlin"}, Files},
		{"nothing after the one argument", []string{"init", "go", ""}, nil, NoFiles},
		{"flags", []string{"init", "-"}, []string{"-dst", "-json", "-set", "-src", "-yes"}, NoFiles},
		{"flag by prefix", []string{"init", "-d"}, []string{"-dst"}, NoFiles},
		{"flags with two dashes", []string{"init", "--s"}, []string{"--set", "--src"}, NoFiles},
		{"flags after the argument", []string{"init", "go", "-"}, []string{"-dst", "-json", "-set", "-src", "-yes"}, NoFiles},
		{"flags of a subcommand", []string{"sources", "add", "-"}, []string{"-force"}, NoFiles},
		{"value of a directory flag", []string{"init", "-dst", ""}, nil, Dirs},
		{"value of a source flag", []string{"init", "-src", ""}, []string{"go", "kotlin"}, Files},
		{"value of a free flag", []string{"init", "-set", ""}, nil, NoFiles},
		{"a value that looks like a flag", []string{"init", "-set", "-"}, nil, NoFiles},
		{"a switch takes no value", []string{"init", "-yes", ""}, []string{"go", "kotlin"}, Files},
		{"flag with its value attached", []string{"init", "-dst=x", ""}, []string{"go", "kotlin"}, Files},
		{"a flag value does not count as the argument", []string{"init", "-dst", "out", ""}, []string{"go", "kotlin"}, Files},
		{"flags before the argument", []string{"init", "-yes", "-dst", "out", "k"}, []string{"kotlin"}, Files},
		{"a word after -- is an argument, not a flag", []string{"init", "--", "-"}, nil, Files},
		{"new alias is free text", []string{"sources", "add", ""}, nil, NoFiles},
		{"source of a new alias is a file", []string{"sources", "add", "x", ""}, nil, Files},
		{"remove takes aliases only", []string{"sources", "remove", ""}, []string{"go", "kotlin"}, NoFiles},
		{"help takes commands", []string{"help", ""}, []string{"init", "inspect", "sources", "help", "version"}, NoFiles},
		{"nothing for a command without arguments", []string{"version", ""}, nil, NoFiles},
		{"flags before a command", []string{"-"}, nil, NoFiles},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given the commands of the test spec, and two registered aliases

			// When completing the last word
			got := Complete(testSpec(), tt.args, testAliases)

			// Then the candidates are the ones that can follow
			if !slices.Equal(values(got), tt.want) {
				t.Errorf("Complete(%q) = %q, want %q", tt.args, values(got), tt.want)
			}

			// And the shell is told whether to add files
			if got.Directive != tt.directive {
				t.Errorf("directive = %q, want %q", got.Directive, tt.directive)
			}
		})
	}
}

func TestComplete_WithoutAliases(t *testing.T) {
	// Given no way to list aliases
	// When completing a source
	got := Complete(testSpec(), []string{"init", ""}, nil)

	// Then there are no candidates, but files are still offered
	if len(got.Candidates) != 0 || got.Directive != Files {
		t.Errorf("got %+v, want only the files directive", got)
	}
}

func TestComplete_Descriptions(t *testing.T) {
	// When completing commands and flags
	commands := Complete(testSpec(), []string{"in"}, testAliases)
	flags := Complete(testSpec(), []string{"init", "-d"}, testAliases)
	aliases := Complete(testSpec(), []string{"init", "g"}, testAliases)

	// Then each candidate says what it is, in a short form
	if commands.Candidates[0].Description != "Initialize a new project from a template" {
		t.Errorf("command description = %q", commands.Candidates[0].Description)
	}
	if flags.Candidates[0].Description != "Destination dir" {
		t.Errorf("flag description = %q, want the part before the parenthesis", flags.Candidates[0].Description)
	}
	if aliases.Candidates[0].Description != "registered template" {
		t.Errorf("alias description = %q", aliases.Candidates[0].Description)
	}
}

func TestResult_String(t *testing.T) {
	tests := []struct {
		name   string
		result Result
		want   string
	}{
		{
			"candidates, then the directive",
			Result{Candidates: []Candidate{{"init", "Start"}, {"go", ""}}, Directive: Files},
			"init\tStart\ngo\n:files",
		},
		{"only a directive", Result{Directive: Dirs}, ":dirs"},
		{"no directive means no files", Result{}, ":none"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSummary(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Short", "Short"},
		{"Ends with a dot.", "Ends with a dot"},
		{"First sentence. Second sentence.", "First sentence"},
		{"Before the parenthesis (and after)", "Before the parenthesis"},
		{"Before a semicolon; after", "Before a semicolon"},
		{"Name: description", "Name"},
		{"  spread \n over\tlines  ", "spread over lines"},
		{strings.Repeat("a", 100), strings.Repeat("a", 69) + "…"},
	}
	for _, tt := range tests {
		if got := Summary(tt.in); got != tt.want {
			t.Errorf("Summary(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestScript(t *testing.T) {
	for _, shell := range Shells() {
		t.Run(shell, func(t *testing.T) {
			// When asking for the script of a supported shell
			script, err := Script(shell)

			// Then it asks skelly for the candidates, and registers itself
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(script, "skelly autocomplete complete --") {
				t.Errorf("the %s script does not call skelly for the candidates", shell)
			}
			if !strings.Contains(script, "skelly autocomplete "+shell) {
				t.Errorf("the %s script does not say how to load it", shell)
			}
		})
	}

	// And an unknown shell is refused, naming the ones there are
	if _, err := Script("tcsh"); err == nil || !strings.Contains(err.Error(), "bash, fish, zsh") {
		t.Errorf("err = %v, want one listing the supported shells", err)
	}
}
