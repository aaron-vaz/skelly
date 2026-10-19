package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// variantConfig is a template that produces a CLI or a service from the same files.
const variantConfig = `
name: variants
description: One template, two variants
inputs:
  name:
    description: Project name
  kind:
    description: What to make
    default: cli
    choices: [cli, service]
raw: [".github", "**/*.tpl"]
exclude:
  - when: '{{ne .Inputs.kind.Value "cli"}}'
    paths: [internal/cli, "docs/**/*.cli.md"]
  - when: '{{ne .Inputs.kind.Value "service"}}'
    paths: [internal/server, Dockerfile, .github/workflows/docker.yml]
  - path: "**/*_test.go"
    when: "{{false}}"
  - path: TEMPLATE.md
    when: "true"
`

func variantFiles() map[string]string {
	return map[string]string{
		".skelly.yml":                    variantConfig,
		"main.go":                        "package {{.Inputs.name.Value}} // {{.Inputs.kind.Value}}",
		"internal/cli/cli.go":            "package cli",
		"internal/server/server.go":      "package server",
		"internal/server/server_test.go": "package server",
		"Dockerfile":                     "FROM scratch",
		"docs/a/b/usage.cli.md":          "cli docs",
		"docs/a/b/usage.md":              "docs",
		".github/workflows/ci.yml":       "sha: ${{ github.sha }}",
		".github/workflows/docker.yml":   "token: ${{ secrets.T }}",
		"deep/er/file.tpl":               "{{ not rendered }}",
		"TEMPLATE.md":                    "about the template, not the project",
	}
}

// skellyResult is what a run of the binary printed and returned.
type skellyResult struct {
	stdout string
	stderr string
	err    error
}

// runSkelly runs the binary with an isolated config file and the given stdin.
func runSkelly(t *testing.T, stdin string, env []string, args ...string) skellyResult {
	t.Helper()

	cmd := exec.Command("./"+binName, args...)
	cmd.Env = append(os.Environ(), "SKELLY_CONFIG="+filepath.Join(t.TempDir(), "config.yml"))
	cmd.Env = append(cmd.Env, env...)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start skelly: %v", err)
	}
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		return skellyResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
	case <-time.After(30 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("skelly %v hung, stdout: %s stderr: %s", args, stdout.String(), stderr.String())
		return skellyResult{}
	}
}

func assertExists(t *testing.T, dir string, rels ...string) {
	t.Helper()
	for _, rel := range rels {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("Expected %s to exist: %v", rel, err)
		}
	}
}

func assertAbsent(t *testing.T, dir string, rels ...string) {
	t.Helper()
	for _, rel := range rels {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("Expected %s to be absent, stat err = %v", rel, err)
		}
	}
}

func assertContent(t *testing.T, path string, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Errorf("%s = %q (%v), want %q", path, got, err, want)
	}
}

func TestIntegration_Exclude_Variants(t *testing.T) {
	tests := []struct {
		name   string
		kind   string
		want   []string
		absent []string
	}{
		{
			name:   "cli drops the service files, template files and unrendered globs stay intact",
			kind:   "cli",
			want:   []string{"main.go", "internal/cli/cli.go", "docs/a/b/usage.md", ".github/workflows/ci.yml", "deep/er/file.tpl"},
			absent: []string{"internal/server", "Dockerfile", ".github/workflows/docker.yml", "TEMPLATE.md", ".skelly.yml"},
		},
		{
			name:   "service drops the cli files, including recursive glob matches",
			kind:   "service",
			want:   []string{"main.go", "internal/server/server.go", "internal/server/server_test.go", "Dockerfile", ".github/workflows/docker.yml", "docs/a/b/usage.md"},
			absent: []string{"internal/cli", "docs/a/b/usage.cli.md", "TEMPLATE.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			srcDir := writeTemplate(t, variantFiles())
			destDir := filepath.Join(t.TempDir(), "dest")

			// When
			res := runSkelly(t, "", nil, "init", srcDir, "-dst", destDir, "-no-input", "-set", "name=demo", "-set", "kind="+tt.kind)

			// Then
			if res.err != nil {
				t.Fatalf("init failed: %v\n%s%s", res.err, res.stdout, res.stderr)
			}
			assertExists(t, destDir, tt.want...)
			assertAbsent(t, destDir, tt.absent...)

			// And contents are rendered, except for raw files
			assertContent(t, filepath.Join(destDir, "main.go"), "package demo // "+tt.kind)
			assertContent(t, filepath.Join(destDir, ".github", "workflows", "ci.yml"), "sha: ${{ github.sha }}")
			assertContent(t, filepath.Join(destDir, "deep", "er", "file.tpl"), "{{ not rendered }}")
		})
	}
}

func TestIntegration_Choices(t *testing.T) {
	t.Run("interactive prompt rejects a value outside the choices and asks again", func(t *testing.T) {
		// Given
		srcDir := writeTemplate(t, variantFiles())
		destDir := filepath.Join(t.TempDir(), "dest")

		// When
		// Inputs are asked in alphabetical order, so kind comes before name
		res := runSkelly(t, "worker\nservice\ndemo\n", nil, "init", srcDir, "-dst", destDir)

		// Then
		if res.err != nil {
			t.Fatalf("init failed: %v\n%s%s", res.err, res.stdout, res.stderr)
		}
		if !strings.Contains(res.stderr, "must be one of: cli, service") {
			t.Errorf("Expected the rejection on stderr, got %q", res.stderr)
		}
		if strings.Contains(res.stderr, "is required") {
			t.Errorf("A rejected choice is not a missing value, got %q", res.stderr)
		}
		assertExists(t, destDir, "internal/server/server.go")
		assertAbsent(t, destDir, "internal/cli")
	})

	t.Run("-set outside the choices fails before anything is written", func(t *testing.T) {
		// Given
		srcDir := writeTemplate(t, variantFiles())
		destDir := filepath.Join(t.TempDir(), "dest")

		// When
		res := runSkelly(t, "", nil, "init", srcDir, "-dst", destDir, "-no-input", "-set", "name=demo", "-set", "kind=worker")

		// Then
		if res.err == nil || !strings.Contains(res.stderr, `input "kind" must be one of: cli, service`) {
			t.Errorf("Expected a choices error, got %v: %s", res.err, res.stderr)
		}
		assertAbsent(t, destDir)
	})

	t.Run("a default outside the choices is rejected when the template is loaded", func(t *testing.T) {
		// Given
		srcDir := writeTemplate(t, map[string]string{
			".skelly.yml": "inputs:\n  kind: {default: web, choices: [cli, service]}\n",
		})
		destDir := filepath.Join(t.TempDir(), "dest")

		// When
		res := runSkelly(t, "", nil, "init", srcDir, "-dst", destDir, "-no-input")

		// Then
		if res.err == nil || !strings.Contains(res.stderr, "default web is not one of its choices") {
			t.Errorf("Expected a validation error, got %v: %s", res.err, res.stderr)
		}
		assertAbsent(t, destDir)
	})
}

func TestIntegration_InvalidConfigs(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{name: "malformed exclude glob", config: "exclude:\n  - path: '[unclosed'\n    when: 'true'", wantErr: `invalid exclude pattern "[unclosed"`},
		{name: "malformed glob in paths", config: "exclude:\n  - paths: [ok, '[unclosed']\n    when: 'true'", wantErr: `invalid exclude pattern "[unclosed"`},
		{name: "malformed raw glob", config: "raw: ['**/[x']", wantErr: `invalid raw pattern "**/[x"`},
		{name: "exclude without a path", config: "exclude:\n  - when: 'true'", wantErr: "exclude path must not be blank"},
		{name: "exclude without a condition", config: "exclude:\n  - path: a", wantErr: `exclude "a": when must not be blank`},
		{name: "condition that is not a boolean", config: "inputs:\n  n: {default: x}\nexclude:\n  - path: a\n    when: '{{.Inputs.n.Value}}'", wantErr: "when must render as true or false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			srcDir := writeTemplate(t, map[string]string{".skelly.yml": tt.config, "a": "x"})
			destDir := filepath.Join(t.TempDir(), "dest")

			// When
			res := runSkelly(t, "", nil, "init", srcDir, "-dst", destDir, "-no-input")

			// Then
			if res.err == nil || !strings.Contains(res.stderr, tt.wantErr) {
				t.Errorf("Expected an error containing %q, got %v: %s", tt.wantErr, res.err, res.stderr)
			}
			assertAbsent(t, destDir)
		})
	}
}

func TestIntegration_Inspect(t *testing.T) {
	type input struct {
		Name     string   `json:"name"`
		Default  any      `json:"default"`
		Choices  []string `json:"choices"`
		Required bool     `json:"required"`
	}
	type result struct {
		Source    string `json:"source"`
		HasConfig bool   `json:"hasConfig"`
		Name      string `json:"name"`
		Inputs    []input
		Raw       []string
		Exclude   []struct {
			Paths []string `json:"paths"`
			When  string   `json:"when"`
		}
	}

	t.Run("prints the template's inputs and settings as JSON on stdout only", func(t *testing.T) {
		// Given
		srcDir := writeTemplate(t, variantFiles())
		workDir := t.TempDir()

		// When
		res := runSkelly(t, "", []string{"PWD=" + workDir}, "inspect", srcDir)

		// Then
		if res.err != nil {
			t.Fatalf("inspect failed: %v\n%s%s", res.err, res.stdout, res.stderr)
		}
		if res.stderr != "" {
			t.Errorf("Expected nothing on stderr, got %q", res.stderr)
		}
		var got result
		if err := json.Unmarshal([]byte(res.stdout), &got); err != nil {
			t.Fatalf("stdout is not JSON: %v\n%s", err, res.stdout)
		}

		// And
		if !got.HasConfig || got.Name != "variants" || got.Source != srcDir {
			t.Errorf("unexpected header: %+v", got)
		}
		if len(got.Inputs) != 2 || got.Inputs[0].Name != "kind" || got.Inputs[1].Name != "name" {
			t.Fatalf("inputs = %+v", got.Inputs)
		}
		if got.Inputs[0].Required || got.Inputs[0].Default != "cli" || strings.Join(got.Inputs[0].Choices, ",") != "cli,service" {
			t.Errorf("kind = %+v", got.Inputs[0])
		}
		if !got.Inputs[1].Required {
			t.Errorf("name has no default so it must be required: %+v", got.Inputs[1])
		}
		if len(got.Raw) != 2 || len(got.Exclude) != 4 || len(got.Exclude[0].Paths) != 2 {
			t.Errorf("raw = %v, exclude = %+v", got.Raw, got.Exclude)
		}
	})

	t.Run("a template without a config has empty lists", func(t *testing.T) {
		// Given
		srcDir := writeTemplate(t, map[string]string{"a.txt": "x"})

		// When
		res := runSkelly(t, "", nil, "inspect", "-src", srcDir)

		// Then
		if res.err != nil {
			t.Fatalf("inspect failed: %v\n%s", res.err, res.stderr)
		}
		for _, want := range []string{`"hasConfig": false`, `"inputs": []`, `"raw": []`, `"exclude": []`} {
			if !strings.Contains(res.stdout, want) {
				t.Errorf("Expected %s in %s", want, res.stdout)
			}
		}
	})

	t.Run("works through a registered alias and creates nothing", func(t *testing.T) {
		// Given
		srcDir := writeTemplate(t, variantFiles())
		configPath := filepath.Join(t.TempDir(), "skelly", "config.yml")
		env := []string{"SKELLY_CONFIG=" + configPath}
		if res := runSkelly(t, "", env, "sources", "add", "mine", srcDir); res.err != nil {
			t.Fatalf("sources add failed: %v\n%s", res.err, res.stderr)
		}
		cwd := t.TempDir()
		cmd := exec.Command(mustAbs(t, "./"+binName), "inspect", "mine")
		cmd.Env = append(os.Environ(), env...)
		cmd.Dir = cwd

		// When
		out, err := cmd.Output()

		// Then
		if err != nil || !json.Valid(out) {
			t.Fatalf("inspect via alias failed: %v\n%s", err, out)
		}
		if entries, _ := os.ReadDir(cwd); len(entries) != 0 {
			t.Errorf("Expected inspect to leave the working directory alone, found %v", entries)
		}
	})

	t.Run("an invalid config fails with a message on stderr", func(t *testing.T) {
		// Given
		srcDir := writeTemplate(t, map[string]string{".skelly.yml": "inputs:\n  a: {default: [x]}"})

		// When
		res := runSkelly(t, "", nil, "inspect", srcDir)

		// Then
		if res.err == nil || res.stdout != "" || !strings.Contains(res.stderr, "default must be a string, number or boolean") {
			t.Errorf("Expected a clean failure, got %v, stdout %q, stderr %q", res.err, res.stdout, res.stderr)
		}
	})

	t.Run("requires a source", func(t *testing.T) {
		// When
		res := runSkelly(t, "", nil, "inspect")

		// Then
		if res.err == nil || !strings.Contains(res.stderr, "source is required") {
			t.Errorf("Expected a source error, got %v: %s", res.err, res.stderr)
		}
	})
}

func TestIntegration_Init_NonInteractive(t *testing.T) {
	t.Run("-set answers some inputs and the prompt covers the rest", func(t *testing.T) {
		// Given
		srcDir := writeTemplate(t, variantFiles())
		destDir := filepath.Join(t.TempDir(), "dest")

		// When only name is missing from -set, so it is the only question
		res := runSkelly(t, "demo\n", nil, "init", srcDir, "-dst", destDir, "-set", "kind=service")

		// Then
		if res.err != nil {
			t.Fatalf("init failed: %v\n%s%s", res.err, res.stdout, res.stderr)
		}
		if strings.Contains(res.stdout, "kind:") {
			t.Errorf("Expected kind not to be asked for, got %s", res.stdout)
		}
		assertContent(t, filepath.Join(destDir, "main.go"), "package demo // service")
	})

	t.Run("-no-input never waits on stdin, even when it is open", func(t *testing.T) {
		// Given a stdin that never closes
		srcDir := writeTemplate(t, variantFiles())
		destDir := filepath.Join(t.TempDir(), "dest")
		cmd := exec.Command("./"+binName, "init", srcDir, "-dst", destDir, "-no-input", "-set", "name=demo")
		cmd.Env = append(os.Environ(), "SKELLY_CONFIG="+filepath.Join(t.TempDir(), "config.yml"))
		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = stdin.Close() }()

		// When
		done := make(chan error, 1)
		go func() { _, err := cmd.CombinedOutput(); done <- err }()

		// Then
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("init failed: %v", err)
			}
		case <-time.After(30 * time.Second):
			_ = cmd.Process.Kill()
			t.Fatal("init waited for input despite -no-input")
		}
		assertExists(t, destDir, "main.go")
	})

	t.Run("a missing required input is named and nothing is created", func(t *testing.T) {
		// Given
		srcDir := writeTemplate(t, variantFiles())
		destDir := filepath.Join(t.TempDir(), "dest")

		// When
		res := runSkelly(t, "", nil, "init", srcDir, "-dst", destDir, "-no-input")

		// Then
		if res.err == nil || !strings.Contains(res.stderr, "missing required inputs: name (pass them with -set name=value)") {
			t.Errorf("Expected the missing input to be named, got %v: %s", res.err, res.stderr)
		}
		assertAbsent(t, destDir)
	})

	t.Run("unknown and invalid answers are all reported at once", func(t *testing.T) {
		// Given
		srcDir := writeTemplate(t, variantFiles())
		destDir := filepath.Join(t.TempDir(), "dest")

		// When
		res := runSkelly(t, "", nil, "init", srcDir, "-dst", destDir, "-no-input", "-set", "nmae=x", "-set", "kind=worker", "-set", "name=")

		// Then
		for _, want := range []string{`unknown input "nmae" (the template has: kind, name)`, `input "kind" must be one of`, `input "name" must not be empty`} {
			if !strings.Contains(res.stderr, want) {
				t.Errorf("Expected %q in %q", want, res.stderr)
			}
		}
		assertAbsent(t, destDir)
	})

	t.Run("-set without a name=value form is a flag error", func(t *testing.T) {
		// When
		res := runSkelly(t, "", nil, "init", "src", "-set", "novalue")

		// Then
		if res.err == nil || !strings.Contains(res.stderr, "name=value") {
			t.Errorf("Expected a flag error, got %v: %s", res.err, res.stderr)
		}
	})

	t.Run("-set on a template without a config is rejected", func(t *testing.T) {
		// Given
		srcDir := writeTemplate(t, map[string]string{"a.txt": "x"})
		destDir := filepath.Join(t.TempDir(), "dest")

		// When
		res := runSkelly(t, "", nil, "init", srcDir, "-dst", destDir, "-set", "a=b")

		// Then
		if res.err == nil || !strings.Contains(res.stderr, "takes no inputs to -set") {
			t.Errorf("Expected a no-inputs error, got %v: %s", res.err, res.stderr)
		}
		assertAbsent(t, destDir)
	})
}

func TestIntegration_Init_Destination(t *testing.T) {
	setup := func(t *testing.T) (srcDir string, destDir string) {
		srcDir = writeTemplate(t, variantFiles())
		destDir = filepath.Join(t.TempDir(), "dest")
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(destDir, "old.txt"), []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
		return srcDir, destDir
	}

	t.Run("-no-input refuses a destination that is not empty and keeps its files", func(t *testing.T) {
		// Given
		srcDir, destDir := setup(t)

		// When
		res := runSkelly(t, "", nil, "init", srcDir, "-dst", destDir, "-no-input", "-set", "name=demo")

		// Then
		if res.err == nil || !strings.Contains(res.stderr, "is not empty, pass -yes to replace its contents") {
			t.Errorf("Expected a refusal, got %v: %s", res.err, res.stderr)
		}
		assertContent(t, filepath.Join(destDir, "old.txt"), "old")
	})

	t.Run("-yes replaces the contents without asking", func(t *testing.T) {
		// Given
		srcDir, destDir := setup(t)

		// When
		res := runSkelly(t, "", nil, "init", srcDir, "-dst", destDir, "-no-input", "-yes", "-set", "name=demo")

		// Then
		if res.err != nil {
			t.Fatalf("init failed: %v\n%s%s", res.err, res.stdout, res.stderr)
		}
		assertAbsent(t, destDir, "old.txt")
		assertExists(t, destDir, "main.go")
	})

	t.Run("-yes still refuses to wipe the home directory", func(t *testing.T) {
		// Given
		srcDir := writeTemplate(t, variantFiles())
		home := t.TempDir()
		if err := os.WriteFile(filepath.Join(home, "precious.txt"), []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}

		// When
		res := runSkelly(t, "", []string{"HOME=" + home}, "init", srcDir, "-dst", home, "-no-input", "-yes", "-set", "name=demo")

		// Then
		if res.err == nil || !strings.Contains(res.stderr, "refusing to overwrite") {
			t.Errorf("Expected a refusal, got %v: %s", res.err, res.stderr)
		}
		assertContent(t, filepath.Join(home, "precious.txt"), "data")
	})
}

func TestIntegration_Init_JSON(t *testing.T) {
	type output struct {
		Source      string         `json:"source"`
		Destination string         `json:"destination"`
		Inputs      map[string]any `json:"inputs"`
	}

	t.Run("prints one JSON object with the inputs used and nothing else", func(t *testing.T) {
		// Given
		srcDir := writeTemplate(t, variantFiles())
		destDir, _ := filepath.EvalSymlinks(t.TempDir())
		destDir = filepath.Join(destDir, "dest")

		// When
		res := runSkelly(t, "", nil, "init", srcDir, "-dst", destDir, "-json", "-set", "name=demo")

		// Then
		if res.err != nil {
			t.Fatalf("init failed: %v\n%s%s", res.err, res.stdout, res.stderr)
		}
		var got output
		if err := json.Unmarshal([]byte(res.stdout), &got); err != nil {
			t.Fatalf("stdout is not a single JSON value: %v\n%s", err, res.stdout)
		}
		if got.Destination != destDir || got.Source != srcDir {
			t.Errorf("unexpected result: %+v", got)
		}
		if got.Inputs["name"] != "demo" || got.Inputs["kind"] != "cli" {
			t.Errorf("inputs = %v", got.Inputs)
		}
		assertExists(t, destDir, "main.go")
	})

	t.Run("a template without a config reports no inputs and no chatter", func(t *testing.T) {
		// Given
		srcDir := writeTemplate(t, map[string]string{"a.txt": "x"})
		destDir := filepath.Join(t.TempDir(), "dest")

		// When
		res := runSkelly(t, "", nil, "init", srcDir, "-dst", destDir, "-json")

		// Then
		var got output
		if res.err != nil || json.Unmarshal([]byte(res.stdout), &got) != nil {
			t.Fatalf("expected only JSON on stdout, got %v: %q", res.err, res.stdout)
		}
		if len(got.Inputs) != 0 {
			t.Errorf("inputs = %v", got.Inputs)
		}
	})

	t.Run("failures keep stdout empty and use a non-zero exit code", func(t *testing.T) {
		// Given
		srcDir := writeTemplate(t, variantFiles())
		destDir := filepath.Join(t.TempDir(), "dest")

		// When
		res := runSkelly(t, "", nil, "init", srcDir, "-dst", destDir, "-json")

		// Then
		if res.err == nil || res.stdout != "" || !strings.Contains(res.stderr, "missing required inputs") {
			t.Errorf("Expected a clean failure, got %v, stdout %q, stderr %q", res.err, res.stdout, res.stderr)
		}
	})
}

func TestIntegration_Help_ListsNewCommandsAndFlags(t *testing.T) {
	// When
	list := runSkelly(t, "", nil, "help")
	initHelp := runSkelly(t, "", nil, "help", "init")
	inspectHelp := runSkelly(t, "", nil, "inspect", "-h")

	// Then
	if !strings.Contains(list.stdout, "inspect") {
		t.Errorf("Expected inspect in the command list, got %s", list.stdout)
	}
	for _, flag := range []string{"-set", "-no-input", "-yes", "-json"} {
		if !strings.Contains(initHelp.stdout, flag) {
			t.Errorf("Expected %s in init help, got %s", flag, initHelp.stdout)
		}
	}
	if !strings.Contains(inspectHelp.stdout, "Usage: skelly inspect") {
		t.Errorf("Expected inspect help, got %s %s", inspectHelp.stdout, inspectHelp.stderr)
	}
}

func mustAbs(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}
