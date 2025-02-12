package templates

type Input struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Default     any    `yaml:"default,omitempty"`
}

type ProjectTemplate struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Renderer    string         `yaml:"renderer,omitempty"`
	Inputs      []Input        `yaml:"inputs"`
	Env         map[string]any `yaml:"env"`
}
