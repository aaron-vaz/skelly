package templates

import (
	"strings"
	"testing"
)

func TestStdRenderer_Supports(t *testing.T) {
	// Given
	r := &StdRenderer{}

	// When/Then
	if !r.Supports(ProjectTemplate{}) {
		t.Errorf("Expected StdRenderer to support any config")
	}
}

func TestStdRenderer_Render(t *testing.T) {
	cfg := ProjectTemplate{
		Name:   "Foo",
		Inputs: map[string]Input{"author": {Value: "Jane"}},
	}

	tests := []struct {
		name    string
		text    string
		want    string
		wantErr bool
	}{
		{name: "plain text is unchanged", text: "hello", want: "hello"},
		{name: "struct field", text: "Name is {{.Name}}", want: "Name is Foo"},
		{name: "input value", text: "by {{.Inputs.author.Value}}", want: "by Jane"},
		{name: "invalid syntax", text: "Name is {{.", wantErr: true},
		{name: "missing input key is an error", text: "{{.Inputs.nope}}", wantErr: true},
		{name: "unknown field is an error", text: "{{.NonExistentField}}", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			r := &StdRenderer{}

			// When
			got, err := r.Render(cfg, "test.txt", tt.text)

			// Then
			if tt.wantErr {
				if err == nil {
					t.Errorf("Render() = %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			// And
			if got != tt.want {
				t.Errorf("Render() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStdRenderer_Render_Functions(t *testing.T) {
	config := ProjectTemplate{Inputs: map[string]Input{
		"owner":   {Value: "Aaron-Vaz"},
		"package": {Value: "io.github.aaronvaz.my_app"},
		"padded":  {Value: "  spaced  "},
	}}

	tests := []struct {
		name    string
		text    string
		want    string
		wantErr string
	}{
		{name: "lower", text: `{{lower .Inputs.owner.Value}}`, want: "aaron-vaz"},
		{name: "upper", text: `{{upper .Inputs.owner.Value}}`, want: "AARON-VAZ"},
		{name: "trim", text: `[{{trim .Inputs.padded.Value}}]`, want: "[spaced]"},
		{name: "replace takes old, new and then the string", text: `{{replace "." "/" .Inputs.package.Value}}`, want: "io/github/aaronvaz/my_app"},
		{name: "replace replaces every occurrence", text: `{{replace "a" "" "banana"}}`, want: "bnn"},
		{name: "trimPrefix", text: `{{trimPrefix "io.github." .Inputs.package.Value}}`, want: "aaronvaz.my_app"},
		{name: "trimSuffix", text: `{{trimSuffix ".my_app" .Inputs.package.Value}}`, want: "io.github.aaronvaz"},
		{name: "calls read as a pipeline", text: `{{.Inputs.owner.Value | lower | replace "-" ""}}`, want: "aaronvaz"},
		{name: "calls can be nested", text: `io.github.{{replace "-" "" (lower .Inputs.owner.Value)}}`, want: "io.github.aaronvaz"},
		{name: "other functions are not available", text: `{{exec "ls"}}`, wantErr: `function "exec" not defined`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got, err := (&StdRenderer{}).Render(config, tt.name, tt.text)

			// Then
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Render() error = %v, want error containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("Render() = %q, %v, want %q", got, err, tt.want)
			}
		})
	}
}
