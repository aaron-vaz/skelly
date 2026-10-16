package templates

import (
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
