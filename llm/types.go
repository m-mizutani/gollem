package llm

import "github.com/m-mizutani/goerr/v2"

type FunctionCall struct {
	ID        string
	Name      string
	Arguments map[string]any
}

// Response is a general response type for each LLM.
type Response struct {
	Texts         []string
	FunctionCalls []*FunctionCall
}

// ParameterType is the type of a parameter.
type ParameterType string

const (
	TypeString  ParameterType = "string"
	TypeNumber  ParameterType = "number"
	TypeInteger ParameterType = "integer"
	TypeBoolean ParameterType = "boolean"
	TypeArray   ParameterType = "array"
	TypeObject  ParameterType = "object"
)

// Parameter is a parameter of a tool.
type Parameter struct {
	// Name is the name of the parameter. It's required and must be unique.
	Name string

	// Type is the type of the parameter. It's required.
	Type ParameterType

	// Description is the description of the parameter. It's optional.
	Description string

	// Required is the required flag of the parameter.
	Required bool

	// Enum is the enum of the parameter. It's optional.
	Enum []string

	// Properties is the properties of the parameter. It's used for object type.
	Properties map[string]*Parameter

	// Items is the items of the parameter. It's used for array type.
	Items *Parameter
}

// Validate validates the parameter.
func (p *Parameter) Validate() error {
	// Name is required
	if p.Name == "" {
		return goerr.Wrap(ErrInvalidParameter, "name is required")
	}

	// Type is required
	if p.Type == "" {
		return goerr.Wrap(ErrInvalidParameter, "type is required")
	}

	// Properties is required for object type
	if p.Type == TypeObject {
		if p.Properties == nil {
			return goerr.Wrap(ErrInvalidParameter, "properties is required for object type")
		}
		// Validate nested properties
		for _, prop := range p.Properties {
			if err := prop.Validate(); err != nil {
				return goerr.Wrap(err, "invalid property")
			}
		}
	}

	// Items is required for array type
	if p.Type == TypeArray {
		if p.Items == nil {
			return goerr.Wrap(ErrInvalidParameter, "items is required for array type")
		}
		// Validate items
		if err := p.Items.Validate(); err != nil {
			return goerr.Wrap(err, "invalid items")
		}
	}

	// Enum is only valid for string type
	if len(p.Enum) > 0 && p.Type != TypeString {
		return goerr.Wrap(ErrInvalidParameter, "enum is only valid for string type")
	}

	return nil
}

type Input interface {
	restricted() inputRestricted
}

type inputRestricted struct{}

// Text is a text input as prompt.
// Usage:
// input := llm.Text("Hello, world!")
type Text string

func (t Text) restricted() inputRestricted {
	return inputRestricted{}
}

// FunctionResponse is a function response.
// Usage:
//
//	input := llm.FunctionResponse{
//		Name:      "function_name",
//		Arguments: map[string]any{"key": "value"},
//	}
type FunctionResponse struct {
	ID    string
	Name  string
	Data  map[string]any
	Error error
}

func (f FunctionResponse) restricted() inputRestricted {
	return inputRestricted{}
}
