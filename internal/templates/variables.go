package templates

import "fmt"

// VariableType represents the allowed types for template variables
type VariableType string

const (
	StringType VariableType = "string"
	BoolType   VariableType = "bool"
	NumberType VariableType = "number"
)

// TemplateVariable represents a strongly-typed variable used in templates
type TemplateVariable struct {
	Name        string
	Type        VariableType
	Description string
	Default     interface{}
	Required    bool
	Value       interface{}
}

// Validate checks if the variable's value matches its declared type
func (v *TemplateVariable) Validate() error {
	if v.Required && v.Value == nil {
		return fmt.Errorf("required variable %s is not set", v.Name)
	}

	if v.Value == nil {
		return nil
	}

	switch v.Type {
	case StringType:
		if _, ok := v.Value.(string); !ok {
			return fmt.Errorf("variable %s must be a string", v.Name)
		}
	case BoolType:
		if _, ok := v.Value.(bool); !ok {
			return fmt.Errorf("variable %s must be a boolean", v.Name)
		}
	case NumberType:
		switch v.Value.(type) {
		case int, int64, float64:
			return nil
		default:
			return fmt.Errorf("variable %s must be a number", v.Name)
		}
	}

	return nil
}

// GetString safely returns the variable's value as a string
func (v *TemplateVariable) GetString() (string, error) {
	if v.Type != StringType {
		return "", fmt.Errorf("variable %s is not a string", v.Name)
	}
	if str, ok := v.Value.(string); ok {
		return str, nil
	}
	return "", fmt.Errorf("invalid string value for %s", v.Name)
}
