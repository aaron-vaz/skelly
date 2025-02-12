package templates

type Input struct {
	Description string `yaml:"description"`
	Default     any    `yaml:"default,omitempty"`
	Value       any
}

type ProjectTemplate struct {
	Name        string           `yaml:"name"`
	Description string           `yaml:"description"`
	Renderer    string           `yaml:"renderer,omitempty"`
	Inputs      map[string]Input `yaml:"inputs"`
	Env         map[string]any   `yaml:"env"`
}
