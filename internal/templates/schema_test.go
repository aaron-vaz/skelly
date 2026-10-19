package templates

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

const schemaPath = "../../skelly.schema.json"

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()

	f, err := os.Open(schemaPath)
	if err != nil {
		t.Fatalf("failed to open schema: %v", err)
	}
	defer func() { _ = f.Close() }()

	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}

	c := jsonschema.NewCompiler()
	if err := c.AddResource("skelly.schema.json", doc); err != nil {
		t.Fatalf("failed to add schema: %v", err)
	}
	schema, err := c.Compile("skelly.schema.json")
	if err != nil {
		t.Fatalf("schema does not compile: %v", err)
	}
	return schema
}

// validateAgainstSchema converts YAML to plain JSON values first, the way an
// editor's YAML language server does.
func validateAgainstSchema(schema *jsonschema.Schema, config string) error {
	var parsed any
	if err := yaml.Unmarshal([]byte(config), &parsed); err != nil {
		return err
	}
	encoded, err := json.Marshal(parsed)
	if err != nil {
		return err
	}
	value, err := jsonschema.UnmarshalJSON(strings.NewReader(string(encoded)))
	if err != nil {
		return err
	}
	return schema.Validate(value)
}

func TestSchema_Configs(t *testing.T) {
	tests := []struct {
		name   string
		config string
		valid  bool
	}{
		{name: "minimal", config: `name: test`, valid: true},
		{name: "empty", config: `{}`, valid: true},
		{
			name: "full config",
			config: `
name: go
description: A Go project
renderer: std
inputs:
  project_name: {description: Name, default: my-app}
  count: {default: 3}
  enabled: {default: true}
  required: {description: no default}
  kind: {default: cli, choices: [cli, service]}
env: {anything: [goes, here]}
raw: [.github, "*.tpl", "**/*.yml"]
exclude:
  - path: internal/server
    when: '{{eq .Inputs.kind.Value "cli"}}'
  - paths: [Dockerfile, "**/*_test.go"]
    when: "true"
  - path: a
    paths: [b]
    when: "false"`,
			valid: true,
		},
		{name: "unknown top-level key", config: `nmae: typo`, valid: false},
		{name: "unknown input key", config: "inputs:\n  a: {defualt: x}", valid: false},
		{name: "list default", config: "inputs:\n  a: {default: [x]}", valid: false},
		{name: "map default", config: "inputs:\n  a: {default: {k: v}}", valid: false},
		{name: "choices must be strings", config: "inputs:\n  a: {choices: [1, 2]}", valid: false},
		{name: "duplicate choices", config: "inputs:\n  a: {choices: [x, x]}", valid: false},
		{name: "blank input name", config: "inputs:\n  ' ': {default: x}", valid: false},
		{name: "raw must be a list", config: `raw: .github`, valid: false},
		{name: "blank raw pattern", config: `raw: [" "]`, valid: false},
		{name: "exclude without when", config: "exclude:\n  - path: a", valid: false},
		{name: "exclude without a path", config: "exclude:\n  - when: 'true'", valid: false},
		{name: "exclude with empty paths", config: "exclude:\n  - paths: []\n    when: 'true'", valid: false},
		{name: "exclude with blank path", config: "exclude:\n  - path: ''\n    when: 'true'", valid: false},
		{name: "exclude with blank when", config: "exclude:\n  - path: a\n    when: ''", valid: false},
		{name: "exclude with unknown key", config: "exclude:\n  - path: a\n    when: 'true'\n    pth: b", valid: false},
	}

	schema := compileSchema(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			err := validateAgainstSchema(schema, tt.config)

			// Then
			if tt.valid && err != nil {
				t.Errorf("schema rejected a valid config: %v", err)
			}
			if !tt.valid && err == nil {
				t.Error("schema accepted an invalid config")
			}
		})
	}
}

// TestSchema_AgreesWithValidate keeps the editor schema from drifting away from
// the Go validation: wherever the schema covers a rule, both must give the same verdict.
func TestSchema_AgreesWithValidate(t *testing.T) {
	tests := []struct {
		name   string
		config string
	}{
		{name: "valid exclude path", config: "exclude:\n  - path: a\n    when: 'true'"},
		{name: "valid exclude paths", config: "exclude:\n  - paths: [a, '**/b']\n    when: 'true'"},
		{name: "exclude without a path", config: "exclude:\n  - when: 'true'"},
		{name: "exclude with empty paths", config: "exclude:\n  - paths: []\n    when: 'true'"},
		{name: "exclude with blank path", config: "exclude:\n  - paths: ['']\n    when: 'true'"},
		{name: "exclude without when", config: "exclude:\n  - path: a"},
		{name: "list default", config: "inputs:\n  a: {default: [x]}"},
		{name: "scalar default", config: "inputs:\n  a: {default: 1}"},
		{name: "blank input name", config: "inputs:\n  ' ': {default: x}"},
		{name: "valid choices", config: "inputs:\n  a: {default: x, choices: [x, y]}"},
	}

	schema := compileSchema(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			var config ProjectTemplate
			if err := yaml.Unmarshal([]byte(tt.config), &config); err != nil {
				t.Fatalf("failed to parse config: %v", err)
			}

			// When
			schemaErr := validateAgainstSchema(schema, tt.config)
			goErr := config.Validate()

			// Then
			if (schemaErr == nil) != (goErr == nil) {
				t.Errorf("schema error = %v, Validate error = %v; both should agree", schemaErr, goErr)
			}
		})
	}
}
