// Package autocomplete works out the completions of a skelly command line, and holds the
// scripts that ask for them from bash, zsh and fish. The scripts only forward the words
// typed so far, so every shell completes the same way and the logic is tested once.
package autocomplete

import (
	"fmt"
	"slices"
	"strings"
)

// Kind says what a flag value or a positional argument is completed with.
type Kind int

const (
	// None suggests nothing, for free text such as the name of a new alias.
	None Kind = iota
	// Dir suggests directories.
	Dir
	// File suggests files and directories.
	File
	// Source suggests registered aliases, files and directories.
	Source
	// Alias suggests registered aliases only.
	Alias
	// Command suggests the names of the commands.
	Command
)

// Directive tells a shell script what to offer besides the candidates.
type Directive string

const (
	// NoFiles offers nothing else.
	NoFiles Directive = "none"
	// Files adds the files and directories of the current path.
	Files Directive = "files"
	// Dirs adds the directories of the current path.
	Dirs Directive = "dirs"
)

// maxSummary is the length a description is cut to, so a menu stays on one line.
const maxSummary = 70

// Flag is an option of a command.
type Flag struct {
	// Name is the flag without its dash.
	Name  string
	Usage string
	// TakesValue is false for a switch such as -yes.
	TakesValue bool
	// Value is what the value of the flag is completed with.
	Value Kind
}

// Cmd is a command or subcommand and what can follow it.
type Cmd struct {
	Name        string
	Description string
	Flags       []Flag
	// Args holds what each positional argument, in order, is completed with.
	Args        []Kind
	Subcommands []Cmd
}

// Spec describes every command of the program.
type Spec struct {
	Commands []Cmd
}

// Candidate is one suggestion.
type Candidate struct {
	Value       string
	Description string
}

// Result is what to suggest for a command line.
type Result struct {
	Candidates []Candidate
	Directive  Directive
}

// String renders the result for a shell script: one candidate per line, as the value
// and, after a tab, its description, then a last line holding the directive.
func (r Result) String() string {
	var b strings.Builder
	for _, c := range r.Candidates {
		b.WriteString(c.Value)
		if c.Description != "" {
			b.WriteString("\t" + c.Description)
		}
		b.WriteString("\n")
	}
	directive := r.Directive
	if directive == "" {
		directive = NoFiles
	}
	b.WriteString(":" + string(directive))
	return b.String()
}

// Complete suggests what can come at the end of args, which are the words typed after the
// program name. The last word is the one being completed and is empty when the cursor
// follows a space. aliases lists the registered template aliases when they are needed.
func Complete(spec Spec, args []string, aliases func() []string) Result {
	if len(args) == 0 {
		args = []string{""}
	}
	cur, words := args[len(args)-1], args[:len(args)-1]

	var (
		cmd         *Cmd
		pending     *Flag
		positional  int
		arguments   []string
		onlyArgs    bool
		nothingMore = Result{Directive: NoFiles}
	)
	for _, word := range words {
		switch {
		case pending != nil && word == "=":
			// bash splits -dst=dir into -dst, = and dir, so the = only joins the flag to its value
		case pending != nil:
			// This word is the value of the flag before it
			pending = nil
		case onlyArgs || !isFlag(word):
			if cmd == nil {
				if cmd = spec.command(word); cmd == nil {
					return nothingMore
				}
				continue
			}
			if positional == 0 {
				if sub := cmd.sub(word); sub != nil {
					cmd = sub
					continue
				}
			}
			positional++
			arguments = append(arguments, word)
		case word == "--":
			onlyArgs = true
		default:
			pending = cmd.valueFlag(word)
		}
	}

	if pending != nil && cur == "=" {
		cur = ""
	}

	switch {
	case pending != nil:
		return kindResult(pending.Value, cur, nil, spec, aliases)
	case strings.HasPrefix(cur, "-") && !onlyArgs:
		if cmd == nil {
			return nothingMore
		}
		return flagResult(cmd, cur)
	case cmd == nil:
		return commandResult(spec.Commands, cur)
	case positional == 0 && len(cmd.Subcommands) > 0:
		return commandResult(cmd.Subcommands, cur)
	}

	kind := None
	switch {
	case positional < len(cmd.Args):
		kind = cmd.Args[positional]
	case len(cmd.Args) > 0 && cmd.Args[len(cmd.Args)-1] == Command:
		// A command name can be followed by the subcommands of that command, as in help sources add
		kind = Command
	}
	return kindResult(kind, cur, arguments, spec, aliases)
}

// Summary shortens a description to its first sentence or clause, on one line.
func Summary(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	cut := len(text)
	for _, sep := range []string{" (", ". ", "; ", ": "} {
		if i := strings.Index(text, sep); i >= 0 {
			cut = min(cut, i)
		}
	}
	text = strings.TrimRight(text[:cut], ".")

	if runes := []rune(text); len(runes) > maxSummary {
		text = string(runes[:maxSummary-1]) + "…"
	}
	return text
}

func isFlag(word string) bool {
	return strings.HasPrefix(word, "-") && word != "-"
}

func (s Spec) command(name string) *Cmd {
	return find(s.Commands, name)
}

func (c *Cmd) sub(name string) *Cmd {
	return find(c.Subcommands, name)
}

func find(cmds []Cmd, name string) *Cmd {
	if i := slices.IndexFunc(cmds, func(c Cmd) bool { return c.Name == name }); i >= 0 {
		return &cmds[i]
	}
	return nil
}

// valueFlag returns the flag a word names, if it is one that takes its value as the next
// word. A flag with its value attached (-dst=dir) is no name that matches, so it takes
// nothing from the next word.
func (c *Cmd) valueFlag(word string) *Flag {
	if c == nil {
		return nil
	}
	name := strings.TrimLeft(word, "-")
	if i := slices.IndexFunc(c.Flags, func(f Flag) bool { return f.Name == name }); i >= 0 && c.Flags[i].TakesValue {
		return &c.Flags[i]
	}
	return nil
}

func commandResult(cmds []Cmd, cur string) Result {
	result := Result{Directive: NoFiles}
	for _, c := range cmds {
		if strings.HasPrefix(c.Name, cur) {
			result.Candidates = append(result.Candidates, Candidate{c.Name, Summary(c.Description)})
		}
	}
	return result
}

// flagResult suggests flags in the dash style the user has started with.
func flagResult(cmd *Cmd, cur string) Result {
	dashes := "-"
	if strings.HasPrefix(cur, "--") {
		dashes = "--"
	}

	result := Result{Directive: NoFiles}
	for _, f := range cmd.Flags {
		if strings.HasPrefix(dashes+f.Name, cur) {
			result.Candidates = append(result.Candidates, Candidate{dashes + f.Name, Summary(f.Usage)})
		}
	}
	return result
}

func kindResult(kind Kind, cur string, previous []string, spec Spec, aliases func() []string) Result {
	result := Result{Directive: NoFiles}
	switch kind {
	case Dir:
		result.Directive = Dirs
	case File:
		result.Directive = Files
	case Source:
		result.Directive = Files
		result.Candidates = aliasCandidates(aliases, cur)
	case Alias:
		result.Candidates = aliasCandidates(aliases, cur)
	case Command:
		result.Candidates = commandResult(commandsAfter(spec.Commands, previous), cur).Candidates
	case None:
	}
	return result
}

// commandsAfter returns the commands that can follow the names of a command path, such as the
// subcommands of "sources" for [sources].
func commandsAfter(cmds []Cmd, path []string) []Cmd {
	for _, name := range path {
		c := find(cmds, name)
		if c == nil {
			return nil
		}
		cmds = c.Subcommands
	}
	return cmds
}

func aliasCandidates(aliases func() []string, cur string) []Candidate {
	if aliases == nil {
		return nil
	}
	var candidates []Candidate
	for _, alias := range aliases() {
		if strings.HasPrefix(alias, cur) {
			candidates = append(candidates, Candidate{Value: alias, Description: "registered template"})
		}
	}
	return candidates
}

// Shells lists the shells there is a script for.
func Shells() []string {
	return []string{"bash", "fish", "zsh"}
}

// Script returns the completion script of a shell. It is meant to be evaluated from the
// startup file of the shell, so it always matches the skelly that is installed.
func Script(shell string) (string, error) {
	switch shell {
	case "bash":
		return bashScript, nil
	case "zsh":
		return zshScript, nil
	case "fish":
		return fishScript, nil
	}
	return "", fmt.Errorf("unsupported shell %q (supported: %s)", shell, strings.Join(Shells(), ", "))
}
