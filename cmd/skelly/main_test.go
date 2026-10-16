package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var binName = "skelly_test_bin"

func TestMain(m *testing.M) {
	// Build the CLI binary
	build := exec.Command("go", "build", "-o", binName)
	if err := build.Run(); err != nil {
		_, _ = os.Stderr.WriteString("Could not build binary: " + err.Error() + "\n")
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Clean up
	_ = os.Remove(binName)

	os.Exit(code)
}

func TestIntegration_Help(t *testing.T) {
	// Given
	cmd := exec.Command("./"+binName, "help")

	// When
	out, err := cmd.CombinedOutput()

	// Then
	if err != nil {
		t.Fatalf("Expected no error, got %v: %s", err, string(out))
	}

	// And
	if !strings.Contains(string(out), "USAGE:") {
		t.Errorf("Expected help output, got %s", string(out))
	}
}

func TestIntegration_Version(t *testing.T) {
	// Given
	cmd := exec.Command("./"+binName, "version")

	// When
	out, err := cmd.CombinedOutput()

	// Then
	if err != nil {
		t.Fatalf("Expected no error, got %v: %s", err, string(out))
	}

	// And
	if !strings.Contains(string(out), "skelly version dev") {
		t.Errorf("Expected version output, got %s", string(out))
	}
}

func TestIntegration_Init_Success(t *testing.T) {
	// Given
	srcDir, _ := filepath.EvalSymlinks(t.TempDir())
	tmpDestBase, _ := filepath.EvalSymlinks(t.TempDir())
	destDir := filepath.Join(tmpDestBase, "dest")

	// Setup template
	configYaml := `
name: "integration-test"
description: "A test template"
inputs:
  project_name:
    description: "The name of the project"
    default: "my-app"
  author:
    description: "Author's name"
`
	err := os.WriteFile(filepath.Join(srcDir, ".skelly.yml"), []byte(configYaml), 0644)
	if err != nil {
		t.Fatalf("Failed to create config yaml: %v", err)
	}

	// Setup template files
	err = os.WriteFile(filepath.Join(srcDir, "{{.Inputs.project_name.Value}}.txt"), []byte("Author: {{.Inputs.author.Value}}"), 0644)
	if err != nil {
		t.Fatalf("Failed to write template file: %v", err)
	}

	// Prepare execution
	cmd := exec.Command("./"+binName, "init", "-src", srcDir, "-dst", destDir)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}

	// Provide interactive stdin input asynchronously
	go func() {
		defer func() { _ = stdin.Close() }()
		// Author input (required as no default)
		_, _ = io.WriteString(stdin, "John Doe\n")
		// Project_name input (has default "my-app", we hit enter to accept)
		_, _ = io.WriteString(stdin, "\n")
	}()

	// When
	out, err := cmd.CombinedOutput()

	// Then
	if err != nil {
		t.Fatalf("Expected no error from init run, got %v: %s", err, string(out))
	}

	// And
	expectedFile := filepath.Join(destDir, "my-app.txt")
	content, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Errorf("Expected generated file %s to exist, but it doesn't: %v. Output: %s", expectedFile, err, string(out))
	}

	// And
	if string(content) != "Author: John Doe" {
		t.Errorf("Expected generated file content 'Author: John Doe', got %s", string(content))
	}
}

func TestIntegration_Init_MissingSrc(t *testing.T) {
	// Given
	cmd := exec.Command("./"+binName, "init") // no -src provided

	// When
	out, err := cmd.CombinedOutput()

	// Then
	if err == nil {
		t.Fatalf("Expected an error for missing src, got nil")
	}

	// And
	if !strings.Contains(string(out), "invalid options: source is required") {
		t.Errorf("Expected message 'source is required', got %s", string(out))
	}
}

func TestIntegration_Init_DefaultDest(t *testing.T) {
	// Given
	srcDir, _ := filepath.EvalSymlinks(t.TempDir())
	destDir, _ := filepath.EvalSymlinks(t.TempDir())

	// Setup template
	configYaml := `
name: "integration-test-default-dest"
description: "A test template for default destination"
inputs:
  project_name:
    description: "The name of the project"
    default: "my-app-default"
  author:
    description: "Author's name"
`
	err := os.WriteFile(filepath.Join(srcDir, ".skelly.yml"), []byte(configYaml), 0644)
	if err != nil {
		t.Fatalf("Failed to create config yaml: %v", err)
	}

	// Setup template files
	err = os.WriteFile(filepath.Join(srcDir, "{{.Inputs.project_name.Value}}.txt"), []byte("Author: {{.Inputs.author.Value}}"), 0644)
	if err != nil {
		t.Fatalf("Failed to write template file: %v", err)
	}

	// A pre-existing file means the overwrite prompt is shown
	_ = os.WriteFile(filepath.Join(destDir, "old.txt"), []byte("old"), 0644)

	// Prepare execution
	absBin, _ := filepath.Abs(binName)
	cmd := exec.Command(absBin, "init", "-src", srcDir)
	cmd.Dir = destDir // Run from destDir to test default destination (.)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}

	// Provide interactive stdin input asynchronously
	go func() {
		defer func() { _ = stdin.Close() }()
		// Overwrite prompt (because destDir . is not empty)
		_, _ = io.WriteString(stdin, "y\n")
		// Author input (required as no default)
		_, _ = io.WriteString(stdin, "Jane Doe\n")
		// Project_name input (has default "my-app-default", we hit enter to accept)
		_, _ = io.WriteString(stdin, "\n")
	}()

	// When
	out, err := cmd.CombinedOutput()

	// Then
	if err != nil {
		t.Fatalf("Expected no error from init run, got %v: %s", err, string(out))
	}

	// And
	expectedFile := filepath.Join(destDir, "my-app-default.txt")
	content, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Errorf("Expected generated file %s to exist in the default destination, but it doesn't: %v. Output: %s", expectedFile, err, string(out))
	}

	// And
	if string(content) != "Author: Jane Doe" {
		t.Errorf("Expected generated file content 'Author: Jane Doe', got %s", string(content))
	}

	// And
	if _, err := os.Stat(filepath.Join(destDir, "old.txt")); !os.IsNotExist(err) {
		t.Errorf("Expected pre-existing file to be replaced")
	}
}

func TestIntegration_Init_ClosedStdin(t *testing.T) {
	// Given
	srcDir := writeTemplate(t, map[string]string{
		".skelly.yml": "inputs:\n  author:\n    description: \"Author's name\"\n",
		"a.txt":       "{{.Inputs.author.Value}}",
	})
	destDir := filepath.Join(t.TempDir(), "dest")
	cmd := exec.Command("./"+binName, "init", "-src", srcDir, "-dst", destDir)
	cmd.Stdin = strings.NewReader("")

	// When
	done := make(chan struct{})
	var out []byte
	var err error
	go func() {
		out, err = cmd.CombinedOutput()
		close(done)
	}()

	// Then
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("init hung waiting for input on a closed stdin")
	}
	if err == nil || !strings.Contains(string(out), "input closed") {
		t.Errorf("Expected a closed input error, got %v: %s", err, string(out))
	}

	// And
	if _, err := os.Stat(destDir); !os.IsNotExist(err) {
		t.Errorf("Expected destination not to be created on failure")
	}
}

func TestIntegration_Init_NestedAndRaw(t *testing.T) {
	// Given
	srcDir := writeTemplate(t, map[string]string{
		".skelly.yml": "raw: [\".github\"]\ninputs:\n  name:\n    description: \"Project name\"\n",
		"{{.Inputs.name.Value}}/{{.Inputs.name.Value}}.go": "package {{.Inputs.name.Value}}",
		".github/workflows/ci.yml":                         "sha: ${{ github.sha }}",
	})
	destDir := filepath.Join(t.TempDir(), "dest")

	// When
	cmd := exec.Command("./"+binName, "init", "-dst", destDir, srcDir)
	cmd.Stdin = strings.NewReader("demo\n")
	out, err := cmd.CombinedOutput()

	// Then
	if err != nil {
		t.Fatalf("Expected no error from init run, got %v: %s", err, string(out))
	}

	// And
	content, err := os.ReadFile(filepath.Join(destDir, "demo", "demo.go"))
	if err != nil || string(content) != "package demo" {
		t.Errorf("Expected demo/demo.go with 'package demo', got %q (%v)", content, err)
	}

	// And
	content, err = os.ReadFile(filepath.Join(destDir, ".github", "workflows", "ci.yml"))
	if err != nil || string(content) != "sha: ${{ github.sha }}" {
		t.Errorf("Expected raw workflow to be copied unrendered, got %q (%v)", content, err)
	}
}

func TestIntegration_InitHelpFlag(t *testing.T) {
	// Given
	cmd := exec.Command("./"+binName, "init", "-h")

	// When
	out, err := cmd.CombinedOutput()

	// Then
	if err != nil {
		t.Fatalf("Expected -h to succeed, got %v: %s", err, string(out))
	}

	// And
	if !strings.Contains(string(out), "Usage: skelly init") || strings.Contains(string(out), "Error") {
		t.Errorf("Expected init help output, got %s", string(out))
	}
}

func writeTemplate(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestIntegration_Init_Interrupted(t *testing.T) {
	// Given a template whose required input is never answered
	srcDir := writeTemplate(t, map[string]string{
		".skelly.yml": "inputs:\n  author:\n    description: \"Author's name\"\n",
		"a.txt":       "{{.Inputs.author.Value}}",
	})
	destDir := filepath.Join(t.TempDir(), "dest")
	tmpDir := t.TempDir()

	cmd := exec.Command("./"+binName, "init", "-src", srcDir, "-dst", destDir)
	cmd.Env = append(os.Environ(), "TMPDIR="+tmpDir)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	defer func() { _ = stdin.Close() }()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start init: %v", err)
	}

	// And init is waiting at the prompt
	prompted := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		var seen strings.Builder
		for {
			n, err := stdout.Read(buf)
			seen.Write(buf[:n])
			if strings.Contains(seen.String(), "author") {
				close(prompted)
				_, _ = io.Copy(io.Discard, stdout)
				return
			}
			if err != nil {
				return
			}
		}
	}()
	select {
	case <-prompted:
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("init never prompted for input")
	}

	// When Ctrl+C is pressed
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("Failed to send interrupt: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	// Then init exits with an error instead of hanging at the prompt
	select {
	case err := <-done:
		if err == nil {
			t.Error("Expected init to fail after interrupt")
		}
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("init did not exit after interrupt")
	}

	// And the destination was never created
	if _, err := os.Stat(destDir); !os.IsNotExist(err) {
		t.Errorf("Expected destination to be untouched, stat err = %v", err)
	}

	// And the working directory was cleaned up
	for _, dir := range []string{tmpDir, filepath.Dir(destDir)} {
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("Expected working directory to be removed from %s, found %v", dir, entries)
		}
	}
}

func TestIntegration_Init_NoConfigSkipsGitDir(t *testing.T) {
	// Given a plain template with no config but with a .git directory
	srcDir := writeTemplate(t, map[string]string{
		".git/HEAD": "ref: refs/heads/main",
		"main.go":   "package main",
	})
	destDir := filepath.Join(t.TempDir(), "dest")

	// When
	out, err := exec.Command("./"+binName, "init", "-src", srcDir, "-dst", destDir).CombinedOutput()

	// Then
	if err != nil {
		t.Fatalf("Expected init to succeed, got %v: %s", err, out)
	}
	if _, err := os.Stat(filepath.Join(destDir, "main.go")); err != nil {
		t.Errorf("Expected template files to be copied: %v", err)
	}

	// And
	if _, err := os.Stat(filepath.Join(destDir, ".git")); !os.IsNotExist(err) {
		t.Errorf("Expected .git not to be copied, stat err = %v", err)
	}

	// And no working directory is left next to the destination
	if entries, _ := os.ReadDir(filepath.Dir(destDir)); len(entries) != 1 {
		t.Errorf("Expected only the destination next to it, found %v", entries)
	}
}

func TestIntegration_Sources(t *testing.T) {
	// Given a template and an isolated config file
	srcDir := writeTemplate(t, map[string]string{
		".skelly.yml":                "inputs:\n  name:\n    description: \"Project name\"\n",
		"{{.Inputs.name.Value}}.txt": "hello {{.Inputs.name.Value}}",
	})
	configPath := filepath.Join(t.TempDir(), "skelly", "config.yml")
	skelly := func(stdin string, args ...string) (string, error) {
		cmd := exec.Command("./"+binName, args...)
		cmd.Env = append(os.Environ(), "SKELLY_CONFIG="+configPath)
		cmd.Stdin = strings.NewReader(stdin)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// When the template is registered
	out, err := skelly("", "sources", "add", "mine", srcDir)

	// Then
	if err != nil {
		t.Fatalf("Expected sources add to succeed, got %v: %s", err, out)
	}

	// And it is listed
	out, err = skelly("", "sources", "list")
	if err != nil || !strings.Contains(out, "mine") || !strings.Contains(out, srcDir) {
		t.Errorf("Expected list to show the alias, got %v: %s", err, out)
	}

	// And init accepts the alias in place of the source
	destDir := filepath.Join(t.TempDir(), "dest")
	out, err = skelly("demo\n", "init", "mine", "-dst", destDir)
	if err != nil {
		t.Fatalf("Expected init via alias to succeed, got %v: %s", err, out)
	}
	content, err := os.ReadFile(filepath.Join(destDir, "demo.txt"))
	if err != nil || string(content) != "hello demo" {
		t.Errorf("Expected demo.txt with 'hello demo', got %q (%v)", content, err)
	}

	// And registering the alias again with another source needs -force
	if out, err := skelly("", "sources", "add", "mine", "github.com/other/template"); err == nil || !strings.Contains(out, "-force") {
		t.Errorf("Expected sources add to refuse replacing the alias, got %v: %s", err, out)
	}

	// When the alias is removed
	out, err = skelly("", "sources", "remove", "mine")

	// Then
	if err != nil {
		t.Fatalf("Expected sources remove to succeed, got %v: %s", err, out)
	}

	// And it is no longer listed
	out, err = skelly("", "sources", "list")
	if err != nil || strings.Contains(out, srcDir) {
		t.Errorf("Expected the alias to be gone, got %v: %s", err, out)
	}
}

func TestIntegration_SourcesHelp(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr bool
	}{
		{name: "group lists subcommands", args: []string{"sources"}, want: "Usage: skelly sources COMMAND"},
		{name: "help for the group", args: []string{"help", "sources"}, want: "Usage: skelly sources COMMAND"},
		{name: "subcommand help flag", args: []string{"sources", "add", "-h"}, want: "Usage: skelly sources add"},
		{name: "unknown subcommand", args: []string{"sources", "nope"}, want: "unknown command: sources nope", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			cmd := exec.Command("./"+binName, tt.args...)
			cmd.Env = append(os.Environ(), "SKELLY_CONFIG="+filepath.Join(t.TempDir(), "config.yml"))

			// When
			out, err := cmd.CombinedOutput()

			// Then
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v: %s", err, tt.wantErr, out)
			}

			// And
			if !strings.Contains(string(out), tt.want) {
				t.Errorf("Expected output containing %q, got %s", tt.want, out)
			}
		})
	}
}
