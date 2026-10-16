package templates

import (
	"strings"
	"text/template"
)

// StdRenderer implements the generic text/template behavior renderer.
type StdRenderer struct{}

// Supports always returns true as this is the universally active default fallback.
func (r *StdRenderer) Supports(config ProjectTemplate) bool {
	return true
}

// Render executes text as a Go text/template against config. Referencing a
// key that does not exist is an error rather than a silent "<no value>".
func (r *StdRenderer) Render(config ProjectTemplate, name string, text string) (string, error) {
	tmpl, err := template.New(name).Option("missingkey=error").Parse(text)
	if err != nil {
		return "", err
	}

	var rendered strings.Builder
	if err := tmpl.Execute(&rendered, config); err != nil {
		return "", err
	}

	return rendered.String(), nil
}
