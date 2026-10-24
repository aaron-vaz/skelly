package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// completionSetup is a directory for the completions to run in: a skelly on the PATH, a
// config with two registered aliases, and some files to be offered.
type completionSetup struct {
	env     []string
	workDir string
}

func newCompletionSetup(t *testing.T) completionSetup {
	t.Helper()

	binary, err := filepath.Abs(binName)
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	if err := os.Symlink(binary, filepath.Join(binDir, "skelly")); err != nil {
		t.Fatal(err)
	}

	workDir := t.TempDir()
	for _, dir := range []string{"outdir", "with space"} {
		if err := os.Mkdir(filepath.Join(workDir, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(workDir, "tmpl.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	env := []string{
		"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"SKELLY_CONFIG=" + filepath.Join(t.TempDir(), "config.yml"),
		"HOME=" + t.TempDir(),
	}
	for alias, source := range map[string]string{"go": "github.com/aaron-vaz/skelly-template-go", "kotlin": "github.com/aaron-vaz/skelly-template-kotlin"} {
		if result := runSkelly(t, "", env, "sources", "add", alias, source); result.err != nil {
			t.Fatalf("could not register %s: %v %s", alias, result.err, result.stderr)
		}
	}
	return completionSetup{env: env, workDir: workDir}
}

// runShell runs a script in a shell, in the work directory, and returns what it printed.
func (s completionSetup) runShell(t *testing.T, shell string, script string, args ...string) string {
	t.Helper()
	stdout, _ := s.runShellStreams(t, shell, script, args...)
	return stdout
}

// runShellStreams is runShell that also returns what the shell wrote to stderr.
func (s completionSetup) runShellStreams(t *testing.T, shell string, script string, args ...string) (string, string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var cmdArgs []string
	switch shell {
	case "fish":
		cmdArgs = []string{"--no-config", "-c", script}
	default:
		cmdArgs = []string{"-c", script, "_"}
	}
	if shell == "zsh" {
		cmdArgs = append([]string{"-f"}, cmdArgs...)
	}
	if shell == "bash" {
		cmdArgs = append([]string{"--norc", "--noprofile"}, cmdArgs...)
	}

	cmd := exec.CommandContext(ctx, shell, append(cmdArgs, args...)...)
	cmd.Dir = s.workDir
	cmd.Env = append(os.Environ(), s.env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s failed: %v\nstderr: %s", shell, err, stderr.String())
	}
	return stdout.String(), stderr.String()
}

func needShell(t *testing.T, shell string) {
	t.Helper()
	if _, err := exec.LookPath(shell); err != nil {
		t.Skipf("%s is not installed", shell)
	}
}

func lines(output string) []string {
	var out []string
	for _, line := range strings.Split(output, "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	slices.Sort(out)
	return out
}

func TestIntegration_Autocomplete_PrintsAScriptForEachShell(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		t.Run(shell, func(t *testing.T) {
			// When asking for the script of a shell
			result := runSkelly(t, "", nil, "autocomplete", shell)

			// Then it is printed alone on stdout, ready to be evaluated
			if result.err != nil || result.stderr != "" {
				t.Fatalf("err = %v, stderr = %q", result.err, result.stderr)
			}
			if !strings.Contains(result.stdout, "skelly autocomplete complete --") {
				t.Errorf("not a completion script: %q", result.stdout)
			}

			// And the shell can parse it
			if _, err := exec.LookPath(shell); err != nil {
				return
			}
			setup := newCompletionSetup(t)
			flag := "-n"
			if shell == "fish" {
				flag = "--no-execute"
			}
			dir := t.TempDir()
			path := filepath.Join(dir, "script")
			if err := os.WriteFile(path, []byte(result.stdout), 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(shell, flag, path)
			cmd.Env = append(os.Environ(), setup.env...)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("%s does not accept the script: %v\n%s", shell, err, out)
			}
		})
	}
}

func TestIntegration_Autocomplete_UnknownShell(t *testing.T) {
	// When asking for a shell there is no script for
	result := runSkelly(t, "", nil, "autocomplete", "powershell")

	// Then it fails, and prints nothing to be evaluated
	if result.err == nil || result.stdout != "" {
		t.Errorf("err = %v, stdout = %q", result.err, result.stdout)
	}
}

func TestIntegration_Autocomplete_Complete(t *testing.T) {
	// Given registered aliases
	setup := newCompletionSetup(t)

	// When asking what can follow init
	result := runSkelly(t, "", setup.env, "autocomplete", "complete", "--", "init", "")

	// Then the aliases come with a description, and a last line says to add files
	want := "go\tregistered template\nkotlin\tregistered template\n:files\n"
	if result.err != nil || result.stdout != want {
		t.Errorf("err = %v, stdout = %q, want %q", result.err, result.stdout, want)
	}
}

func TestIntegration_Autocomplete_Bash(t *testing.T) {
	needShell(t, "bash")
	setup := newCompletionSetup(t)
	script := `source <(skelly autocomplete bash); COMP_WORDS=("$@"); COMP_CWORD=$(($#-1)); _skelly_complete 2>/dev/null; printf '%s\n' "${COMPREPLY[@]}"`

	tests := []struct {
		name  string
		words []string
		want  []string
	}{
		{"commands", []string{"skelly", "in"}, []string{"init", "inspect"}},
		{"subcommands", []string{"skelly", "sources", "r"}, []string{"remove"}},
		{"aliases and files for a template", []string{"skelly", "init", ""}, []string{"go", "kotlin", "outdir", "tmpl.txt", "with space"}},
		{"an alias by prefix", []string{"skelly", "init", "k"}, []string{"kotlin"}},
		{"a file by prefix", []string{"skelly", "init", "t"}, []string{"tmpl.txt"}},
		{"flags", []string{"skelly", "init", "-"}, []string{"-dst", "-json", "-no-input", "-set", "-src", "-yes"}},
		{"only directories for -dst", []string{"skelly", "init", "-dst", ""}, []string{"outdir", "with space"}},
		{"only aliases to remove", []string{"skelly", "sources", "remove", ""}, []string{"go", "kotlin"}},
		{"nothing for a switch's value", []string{"skelly", "init", "-set", ""}, nil},
		// bash splits -dst=dir into -dst, = and dir
		{"directories after a split =", []string{"skelly", "init", "-dst", "=", ""}, []string{"outdir", "with space"}},
		{"directories while typing the =", []string{"skelly", "init", "-dst", "="}, []string{"outdir", "with space"}},
		{"a directory by prefix after a split =", []string{"skelly", "init", "-dst", "=", "ou"}, []string{"outdir"}},
		{"no file after a split = of a directory flag", []string{"skelly", "init", "-dst", "=", "t"}, nil},
		{"the subcommands of a group for help", []string{"skelly", "help", "sources", ""}, []string{"add", "list", "remove"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given the words typed so far, the last being completed
			// When bash asks for the completions
			got := lines(setup.runShell(t, "bash", script, tt.words...))

			// Then they are the ones that can follow
			if !slices.Equal(got, tt.want) {
				t.Errorf("completing %q gave %q, want %q", tt.words, got, tt.want)
			}
		})
	}
}

func TestIntegration_Autocomplete_Fish(t *testing.T) {
	needShell(t, "fish")
	setup := newCompletionSetup(t)
	script := `skelly autocomplete fish | source; complete -C "$argv[1]"`

	tests := []struct {
		name string
		line string
		want []string
	}{
		{"commands", "skelly in", []string{"init\tInitialize a new project from a template", "inspect\tShow a template's inputs and settings as JSON"}},
		{"aliases and files for a template", "skelly init ", []string{"go\tregistered template", "kotlin\tregistered template", "outdir/", "tmpl.txt", "with space/"}},
		{"only directories for -dst", "skelly init -dst ", []string{"outdir/\tDirectory", "with space/\tDirectory"}},
		{"only aliases to remove", "skelly sources remove ", []string{"go\tregistered template", "kotlin\tregistered template"}},
		{"a file by prefix", "skelly init t", []string{"tmpl.txt"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When fish completes the line
			got := lines(setup.runShell(t, "fish", script, tt.line))

			// Then they are the ones that can follow
			want := slices.Clone(tt.want)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("completing %q gave %q, want %q", tt.line, got, want)
			}
		})
	}
}

func TestIntegration_Autocomplete_Zsh(t *testing.T) {
	needShell(t, "zsh")
	setup := newCompletionSetup(t)
	// _describe and _files belong to zsh's completion system, which needs a terminal to run,
	// so they print what the script gives them
	script := `
compdef() { :; }
source <(skelly autocomplete zsh)
_describe() { local name=${@[-1]}; print -rl -- "describe:${(@P)name}"; }
_files() { print -r -- "files:$*"; }
words=("$@"); CURRENT=$#
_skelly`

	tests := []struct {
		name  string
		words []string
		want  []string
	}{
		{"commands", []string{"skelly", "in"}, []string{"describe:init:Initialize a new project from a template", "inspect:Show a template's inputs and settings as JSON"}},
		{"aliases and files for a template", []string{"skelly", "init", ""}, []string{"describe:go:registered template", "kotlin:registered template", "files:"}},
		{"only directories for -dst", []string{"skelly", "init", "-dst", ""}, []string{"files:-/"}},
		{"only aliases to remove", []string{"skelly", "sources", "remove", ""}, []string{"describe:go:registered template", "kotlin:registered template"}},
		{"nothing for a free value", []string{"skelly", "init", "-set", ""}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When zsh asks for the completions of the words
			got := lines(setup.runShell(t, "zsh", script, tt.words...))

			// Then the script hands zsh the ones that can follow
			want := slices.Clone(tt.want)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("completing %q gave %q, want %q", tt.words, got, want)
			}
		})
	}
}

func TestIntegration_Autocomplete_FishIsQuietWithoutSkelly(t *testing.T) {
	needShell(t, "fish")
	setup := newCompletionSetup(t)

	// Given the script is loaded, and skelly then goes missing from the PATH
	script := `skelly autocomplete fish | source; set -gx PATH /usr/bin /bin; complete -C "skelly init "`

	// When fish completes
	stdout, stderr := setup.runShellStreams(t, "fish", script)

	// Then it says nothing, instead of an error and a stack trace on every Tab
	if stdout != "" || stderr != "" {
		t.Errorf("expected no output, got stdout %q and stderr %q", stdout, stderr)
	}
}

func TestIntegration_Autocomplete_ZshRegistersWhateverTheOrderOfCompinit(t *testing.T) {
	needShell(t, "zsh")
	setup := newCompletionSetup(t)
	// A shell that is not interactive never shows a prompt, so the hooks that run before one are run by hand
	prompt := `for f in $precmd_functions; do $f; done`

	tests := []struct {
		name   string
		script string
		want   string
	}{
		{"the eval before compinit, as in a startup file that ends with compinit", `eval "$(skelly autocomplete zsh)"; autoload -Uz compinit; compinit -u -d "$ZDOTDIR/dump"; ` + prompt, "_skelly"},
		{"the eval after compinit", `autoload -Uz compinit; compinit -u -d "$ZDOTDIR/dump"; eval "$(skelly autocomplete zsh)"; ` + prompt, "_skelly"},
		{"no compinit at all registers nothing and says nothing", `eval "$(skelly autocomplete zsh)"; ` + prompt, "<none>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a shell with its own completion cache directory
			zdotdir := t.TempDir()

			// When the startup file is run, and then the first prompt
			got := setup.runShell(t, "zsh", `export ZDOTDIR=`+zdotdir+`; `+tt.script+`; print -r -- ${_comps[skelly]:-<none>}`)

			// Then skelly is completed by the script, or nothing is, and the hook is gone
			if strings.TrimSpace(got) != tt.want {
				t.Errorf("_comps[skelly] = %q, want %q", strings.TrimSpace(got), tt.want)
			}
			if _, err := os.Stat(filepath.Join(zdotdir, "dump")); tt.want == "<none>" && err == nil {
				t.Error("the script ran compinit, which it must leave to the user")
			}
		})
	}
}
