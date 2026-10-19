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
			name: "exclude without a condition is allowed, as is a blank one",
			config: `
exclude:
  - path: TEMPLATE.md
  - paths: [a, b]
    when: ""`,
		},
		{
			name: "exclude condition that does not parse",
			config: `
exclude:
  - path: a
    when: '{{if'`,
			wantErr: []string{`exclude "a": when:`},
		},
		{
			name: "invalid exclude",
			config: `
exclude:
  - when: "true"
  - path: '[unclosed'
    when: "true"
  - path: docs`,
			wantErr: []string{"exclude path must not be blank", `invalid exclude pattern "[unclosed"`},
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
			name: "renames cannot use .git or the whole project",
			config: `
rename:
  - from: a
    to: .git/hooks
  - from: .GIT
    to: b
  - from: x/.git/y
    to: c
  - from: .
    to: d
  - from: e
    to: ./`,
			wantErr: []string{
				`rename "a" to ".git/hooks": a directory that is never copied from a template (.git) cannot be used`,
				`rename ".GIT" to "b": a directory that is never copied`,
				`rename "x/.git/y" to "c": a directory that is never copied`,
				`rename "." to "d": paths must be relative and stay inside the project`,
				`rename "e" to "./": paths must be relative and stay inside the project`,
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

func TestProjectTemplate_Validate_Templates(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr []string
	}{
		{
			name: "inputs may use other inputs",
			config: `
inputs:
  owner: {default: me}
  name: {description: 'Name for {{.Inputs.owner.Value}}'}
  module: {default: 'github.com/{{.Inputs.owner.Value}}/{{.Inputs.name.Value}}'}
  kind: {default: '{{.Inputs.owner.Value}}', choices: ['{{.Inputs.owner.Value}}', other]}`,
		},
		{
			name: "references inside conditions and pipelines are found",
			config: `
inputs:
  a: {default: x}
  b: {default: '{{if eq .Inputs.a.Value "x"}}{{.Inputs.a.Value | printf "%s!"}}{{end}}'}`,
		},
		{
			name: "env and rename and patterns may be templates",
			config: `
inputs:
  name: {default: app}
env:
  module: 'github.com/me/{{.Inputs.name.Value}}'
  year: 2026
raw: ['{{.Inputs.name.Value}}/**']
exclude:
  - path: 'docs/{{.Inputs.name.Value}}'
    when: "true"
rename:
  - from: 'cmd/{{.Inputs.name.Value}}'
    to: 'app/{{.Inputs.name.Value}}'
    when: '{{eq .Inputs.name.Value "app"}}'`,
		},
		{
			name: "templates that do not parse",
			config: `
inputs:
  a: {default: '{{.Inputs.b'}
  b: {description: '{{if}}'}
env:
  x: '{{end}}'
rename:
  - from: a
    to: b
    when: '{{'
raw: ['{{']`,
			wantErr: []string{`input "a": default:`, `input "b": description:`, `env "x":`, `rename "a": when:`, `invalid raw pattern "{{"`},
		},
		{
			name: "unknown input",
			config: `
inputs:
  a: {default: '{{.Inputs.nope.Value}}'}`,
			wantErr: []string{`input "a": default uses unknown input "nope"`},
		},
		{
			name: "input that uses itself",
			config: `
inputs:
  a: {default: '{{.Inputs.a.Value}}'}`,
			wantErr: []string{"inputs use each other in a cycle, involving: a"},
		},
		{
			name: "inputs that use each other",
			config: `
inputs:
  a: {default: '{{.Inputs.b.Value}}'}
  b: {default: '{{.Inputs.c.Value}}'}
  c: {default: '{{.Inputs.a.Value}}'}
  d: {default: fine}`,
			wantErr: []string{"inputs use each other in a cycle, involving: a, b, c"},
		},
		{
			name: "references through a variable are found, so the order is right",
			config: `
inputs:
  a: {default: x}
  b: {default: '{{$.Inputs.a.Value}}'}
  c: {default: '{{with .Inputs.b}}{{.Value}}{{end}}'}
  d: {default: '{{$x := .Inputs.c}}{{$x.Value}}'}`,
		},
		{
			name: "reaching the inputs without naming one cannot be ordered",
			config: `
inputs:
  a: {default: x}
  b: {default: '{{index .Inputs "a"}}'}
  c: {default: '{{with .Inputs}}{{.a.Value}}{{end}}'}
  d: {default: '{{range $k, $v := .Inputs}}{{$k}}{{end}}'}
  e: {default: '{{$.Inputs}}'}`,
			wantErr: []string{
				`input "b": default: uses .Inputs without naming an input`,
				`input "c": default: uses .Inputs without naming an input`,
				`input "d": default: uses .Inputs without naming an input`,
				`input "e": default: uses .Inputs without naming an input`,
			},
		},
		{
			name: "a templated default is not compared with literal choices until it is rendered",
			config: `
inputs:
  a: {default: x}
  b: {default: '{{.Inputs.a.Value}}', choices: [y]}`,
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

func TestProjectTemplate_InputLayers(t *testing.T) {
	// Given
	var config ProjectTemplate
	err := yaml.Unmarshal([]byte(`
inputs:
  owner: {default: me}
  name: {description: Name}
  module: {default: 'github.com/{{.Inputs.owner.Value}}/{{.Inputs.name.Value}}'}
  image: {default: 'ghcr.io/{{.Inputs.module.Value}}'}
  alone: {default: x}`), &config)
	if err != nil {
		t.Fatal(err)
	}

	// When
	layers, err := config.InputLayers()
	deps := config.InputDependencies()

	// Then
	if err != nil {
		t.Fatalf("InputLayers() error = %v", err)
	}
	want := [][]string{{"alone", "name", "owner"}, {"module"}, {"image"}}
	if len(layers) != len(want) {
		t.Fatalf("layers = %v, want %v", layers, want)
	}
	for i := range want {
		if strings.Join(layers[i], ",") != strings.Join(want[i], ",") {
			t.Errorf("layer %d = %v, want %v", i, layers[i], want[i])
		}
	}

	// And
	if got := strings.Join(deps["module"], ","); got != "name,owner" {
		t.Errorf("module depends on %q, want name,owner", got)
	}
	if _, ok := deps["owner"]; ok {
		t.Errorf("owner depends on nothing, got %v", deps["owner"])
	}
}
