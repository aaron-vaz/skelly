package view

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/aaron-vaz/skelly/internal/templates"
)

func TestStdUI_RenderInfo(t *testing.T) {
	// Given
	r, w, _ := os.Pipe()
	ui := NewStdUI(os.Stdin, w, os.Stderr)

	// When
	err := ui.RenderInfo("hello info")

	// Then
	if err != nil {
		t.Fatalf("RenderInfo failed: %v", err)
	}

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	// And
	if !strings.Contains(buf.String(), "hello info\n") {
		t.Errorf("Expected info message, got %s", buf.String())
	}
}

func TestStdUI_RenderError(t *testing.T) {
	// Given
	r, w, _ := os.Pipe()
	ui := NewStdUI(os.Stdin, os.Stdout, w)

	// When
	err := ui.RenderError("hello error")

	// Then
	if err != nil {
		t.Fatalf("RenderError failed: %v", err)
	}

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	// And
	if !strings.Contains(buf.String(), "hello error\n") {
		t.Errorf("Expected error message, got %s", buf.String())
	}
}

func TestStdUI_RenderQuestion(t *testing.T) {
	// Given
	inR, inW, _ := os.Pipe()
	outR, outW, _ := os.Pipe()

	ui := NewStdUI(inR, outW, os.Stderr)

	go func() {
		_, _ = inW.WriteString("y\n")
		_ = inW.Close()
	}()

	// When
	ans, err := ui.RenderQuestion(context.Background(), "Are you sure?", []string{"y", "n"})
	_ = outW.Close()

	// Then
	if err != nil {
		t.Fatalf("RenderQuestion failed: %v", err)
	}

	// And
	if ans != "y" {
		t.Errorf("Expected answer 'y', got '%s'", ans)
	}

	var outBuf bytes.Buffer
	_, _ = io.Copy(&outBuf, outR)

	// And
	if !strings.Contains(outBuf.String(), "Are you sure?") {
		t.Errorf("Expected question prompt, got %s", outBuf.String())
	}
}

func TestStdUI_RenderInputs(t *testing.T) {
	// Given
	inR, inW, _ := os.Pipe()
	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	_ = outR

	ui := NewStdUI(inR, outW, errW)

	go func() {
		// opt_input is asked first (sorted by key): Provide empty value
		_, _ = inW.WriteString("\n")
		// req_input is asked second: Provide empty value (triggers error and re-prompt)
		_, _ = inW.WriteString("\n")
		// req_input asked third: Provide valid value
		_, _ = inW.WriteString("valid-val\n")
		_ = inW.Close()
	}()

	inputs := map[string]templates.Input{
		"req_input": {
			Description: "A required input",
			Default:     nil,
		},
		"opt_input": {
			Description: "An optional input",
			Default:     "def-val",
		},
	}

	// When
	err := ui.RenderInputs(context.Background(), inputs)
	_ = outW.Close()
	_ = errW.Close()

	// Then
	if err != nil {
		t.Fatalf("RenderInputs failed: %v", err)
	}

	// And
	if inputs["req_input"].Value != "valid-val" {
		t.Errorf("Expected req_input to be 'valid-val', got %v", inputs["req_input"].Value)
	}

	// And
	if inputs["opt_input"].Value != "def-val" {
		t.Errorf("Expected opt_input to be 'def-val', got %v", inputs["opt_input"].Value)
	}

	var errBuf bytes.Buffer
	_, _ = io.Copy(&errBuf, errR)

	// And
	if !strings.Contains(errBuf.String(), "is required. Please provide a value") {
		t.Errorf("Expected required error missing, got '%s'", errBuf.String())
	}
}

func TestStdUI_RenderInputs_Choices(t *testing.T) {
	// Given
	var out, errOut bytes.Buffer
	ui := NewStdUI(strings.NewReader("web\nservice\n\n"), &out, &errOut)
	inputs := map[string]templates.Input{
		"kind": {Description: "Kind of project", Choices: []string{"cli", "service"}},
		"mode": {Description: "Mode", Default: "dev", Choices: []string{"dev", "prod"}},
	}

	// When
	err := ui.RenderInputs(context.Background(), inputs)

	// Then
	if err != nil {
		t.Fatalf("RenderInputs failed: %v", err)
	}

	// And an answer outside the choices is rejected and asked again
	if !strings.Contains(errOut.String(), "must be one of: cli, service") {
		t.Errorf("Expected choices error, got %q", errOut.String())
	}
	if inputs["kind"].Value != "service" {
		t.Errorf("Expected kind to be 'service', got %v", inputs["kind"].Value)
	}

	// And the default stands in for a blank answer
	if inputs["mode"].Value != "dev" {
		t.Errorf("Expected mode to be 'dev', got %v", inputs["mode"].Value)
	}

	// And the prompt lists the choices
	if !strings.Contains(out.String(), "(one of: cli, service)") {
		t.Errorf("Expected choices in prompt, got %q", out.String())
	}
}

func TestStdUI_RenderInputs_Empty(t *testing.T) {
	// Given
	ui := NewStdUI(os.Stdin, os.Stdout, os.Stderr)

	// When
	err := ui.RenderInputs(context.Background(), map[string]templates.Input{})

	// Then
	if err != nil {
		t.Errorf("Expected nil on empty inputs, got %v", err)
	}
}

func TestStdUI_RenderInputs_ClosedInput(t *testing.T) {
	// Given
	var out, errOut bytes.Buffer
	ui := NewStdUI(strings.NewReader(""), &out, &errOut)
	inputs := map[string]templates.Input{
		"req_input": {Description: "A required input"},
	}

	// When
	err := ui.RenderInputs(context.Background(), inputs)

	// Then
	if !errors.Is(err, ErrNoInput) {
		t.Errorf("RenderInputs() error = %v, want ErrNoInput", err)
	}
}

func TestStdUI_RenderQuestion_ClosedInput(t *testing.T) {
	// Given
	var out bytes.Buffer
	ui := NewStdUI(strings.NewReader(""), &out, &out)

	// When
	_, err := ui.RenderQuestion(context.Background(), "Continue?", []string{"y", "n"})

	// Then
	if !errors.Is(err, ErrNoInput) {
		t.Errorf("RenderQuestion() error = %v, want ErrNoInput", err)
	}
}

func TestStdUI_RenderQuestion_TrimsAnswer(t *testing.T) {
	// Given
	var out bytes.Buffer
	ui := NewStdUI(strings.NewReader("  y \r\n"), &out, &out)

	// When
	ans, err := ui.RenderQuestion(context.Background(), "Continue?", []string{"y", "n"})

	// Then
	if err != nil {
		t.Fatalf("RenderQuestion() error = %v", err)
	}

	// And
	if ans != "y" {
		t.Errorf("RenderQuestion() = %q, want %q", ans, "y")
	}

	// And
	if strings.Contains(out.String(), "??") {
		t.Errorf("Expected a single question mark, got %q", out.String())
	}
}

func TestStdUI_RenderQuestion_Cancelled(t *testing.T) {
	// Given a prompt waiting on input that never arrives
	inR, inW := io.Pipe()
	defer func() { _ = inW.Close() }()
	ui := NewStdUI(inR, io.Discard, io.Discard)
	ctx, cancel := context.WithCancel(context.Background())

	// When the context is cancelled mid-prompt
	cancel()
	_, err := ui.RenderQuestion(ctx, "Continue?", []string{"y", "n"})

	// Then the prompt gives up instead of blocking
	if !errors.Is(err, context.Canceled) {
		t.Errorf("RenderQuestion() error = %v, want context.Canceled", err)
	}
}
