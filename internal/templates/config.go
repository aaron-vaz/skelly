package templates

import "fmt"

// TemplateConfig represents the configuration for a template with type-safe variables
type TemplateConfig struct {
	Variables map[string]*TemplateVariable
	Files     []string
}

// ValidateConfig ensures all template variables are valid
func ValidateConfig(config *TemplateConfig) error {
	if config == nil {
		return fmt.Errorf("template configuration is nil")
	}

	for _, v := range config.Variables {
		if err := v.Validate(); err != nil {
			return fmt.Errorf("invalid variable configuration: %w", err)
		}
	}

	return nil
}

// GetVariable safely retrieves a variable from the configuration
func (c *TemplateConfig) GetVariable(name string) (*TemplateVariable, error) {
	if v, exists := c.Variables[name]; exists {
		return v, nil
	}
	return nil, fmt.Errorf("variable %s not found in template configuration", name)
}
