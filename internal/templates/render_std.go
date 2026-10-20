package templates

import (
	"strings"
	"text/template"
)

// templateFuncs are the functions a template can call, on top of those of
// text/template. They only work on strings, and the string is the last
// argument so a call also reads as a pipeline: {{.Inputs.name.Value | replace "-" "_"}}.
// Parsing a template needs them too, or its calls are reported as undefined.
var templateFuncs = template.FuncMap{
	"lower":      strings.ToLower,
	"upper":      strings.ToUpper,
	"trim":       strings.TrimSpace,
	"replace":    func(old string, replacement string, s string) string { return strings.ReplaceAll(s, old, replacement) },
	"trimPrefix": func(prefix string, s string) string { return strings.TrimPrefix(s, prefix) },
	"trimSuffix": func(suffix string, s string) string { return strings.TrimSuffix(s, suffix) },
}

// StdRenderer implements the generic text/template behavior renderer.
type StdRenderer struct{}

// Supports always returns true as this is the universally active default fallback.
func (r *StdRenderer) Supports(config ProjectTemplate) bool {
	return true
}

// Render executes text as a Go text/template against config. Referencing a
// key that does not exist is an error rather than a silent "<no value>".
func (r *StdRenderer) Render(config ProjectTemplate, name string, text string) (string, error) {
	tmpl, err := template.New(name).Funcs(templateFuncs).Option("missingkey=error").Parse(text)
	if err != nil {
		return "", err
	}

	var rendered strings.Builder
	if err := tmpl.Execute(&rendered, config); err != nil {
		return "", err
	}

	return rendered.String(), nil
}
