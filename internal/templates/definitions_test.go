package templates

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestProjectTemplate_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr []string
	}{
		{
			name:   "minimal config",
			config: `name: test`,
		},
		{
			name: "scalar defaults",
			config: `
inputs:
  text: {default: hello}
  count: {default: 3}
  ratio: {default: 0.5}
  enabled: {default: true}
  required: {description: no default}`,
		},
		{
			name: "blank input name",
			config: `
inputs:
  " ": {description: blank}`,
			wantErr: []string{"input name must not be blank"},
		},
		{
			name: "list and map defaults",
			config: `
inputs:
  list: {default: [1, 2]}
  dict: {default: {a: b}}`,
			wantErr: []string{`input "dict": default must be`, `input "list": default must be`},
		},
		{
			name: "default within choices",
			config: `
inputs:
  kind: {default: cli, choices: [cli, service]}
  other: {choices: [a, b]}`,
		},
		{
			name: "default outside choices",
			config: `
inputs:
  kind: {default: web, choices: [cli, service]}`,
			wantErr: []string{`input "kind": default web is not one of its choices (cli, service)`},
		},
		{
			name: "valid exclude",
			config: `
exclude:
  - path: internal/server
    when: '{{eq .Inputs.kind.Value "cli"}}'`,
		},
		{
			name: "valid exclude with several paths and recursive globs",
			config: `
exclude:
  - paths: [internal/server, "**/*_test.go", "Dockerfile{,.dev}"]
    when: "true"
  - path: docs
    paths: [site]
    when: "false"`,
		},
		{
			name: "invalid exclude paths",
			config: `
exclude:
  - paths: ['', '[unclosed']
    when: "true"`,
			wantErr: []string{"exclude path must not be blank", `invalid exclude pattern "[unclosed"`},
		},
		{
			name: "invalid exclude",
			config: `
exclude:
  - when: "true"
  - path: '[unclosed'
    when: "true"
  - path: docs`,
			wantErr: []string{"exclude path must not be blank", `invalid exclude pattern "[unclosed"`, `exclude "docs": when must not be blank`},
		},
		{
			name: "valid rename",
			config: `
rename:
  - from: github
    to: .github
  - from: gitignore
    to: config/.gitignore`,
		},
		{
			name: "invalid renames",
			config: `
rename:
  - from: a
  - to: b
  - from: ../outside
    to: b
  - from: c
    to: /abs
  - from: d
    to: d
  - from: ./e
    to: e
  - from: .skelly.yml
    to: x
  - from: f
    to: dup
  - from: g
    to: ./dup`,
			wantErr: []string{
				"rename needs both from and to",
				`rename "../outside" to "b": paths must be relative and stay inside the project`,
				`rename "c" to "/abs": paths must be relative and stay inside the project`,
				`rename "d": from and to are the same`,
				`rename "./e": from and to are the same`,
				`rename ".skelly.yml": the config file is never copied`,
				`rename "g": more than one rename ends up at "./dup"`,
			},
		},
		{
			name:   "recursive raw pattern",
			config: `raw: ["**/*.tpl"]`,
		},
		{
			name:    "invalid raw pattern",
			config:  `raw: ['[unclosed']`,
			wantErr: []string{`invalid raw pattern "[unclosed"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			var config ProjectTemplate
			if err := yaml.Unmarshal([]byte(tt.config), &config); err != nil {
				t.Fatalf("failed to parse config: %v", err)
			}

			// When
			err := config.Validate()

			// Then
			if len(tt.wantErr) == 0 {
				if err != nil {
					t.Errorf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() error = nil, want %v", tt.wantErr)
			}
			for _, want := range tt.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("Validate() error = %q, want it to contain %q", err, want)
				}
			}
		})
	}
}
