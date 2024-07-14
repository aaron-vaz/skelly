package template

type Data struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
}

type Input struct {
	Data
	Default any `yaml:"default"`
}

type Env struct {
	Data
	Value any `yaml:"value"`
}

type ProjectTemplate struct {
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	Inputs      []Input `yaml:"inputs"`
	Env         []Env   `yaml:"env"`
}
