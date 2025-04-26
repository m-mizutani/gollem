package gollam

import (
	"context"
	"regexp"

	"github.com/m-mizutani/goerr/v2"
)

// ToolSpec is the specification of a tool.
// It defines the interface and behavior of a tool that can be used by LLMs.
type ToolSpec struct {
	// Name is the unique identifier for the tool.
	// It must be unique across all tools in the system.
	Name string

	// Description is a human-readable description of what the tool does.
	// It should be clear and concise to help LLMs understand the tool's purpose.
	Description string

	// Parameters defines the input parameters that the tool accepts.
	// Each parameter is defined by its name and specification.
	Parameters map[string]*Parameter

	// Required is the list of required parameter names.
	// These parameters must be provided when the tool is called.
	Required []string
}

// Validate validates the tool specification.
func (s *ToolSpec) Validate() error {
	eb := goerr.NewBuilder(goerr.V("tool", s))
	if s.Name == "" {
		return eb.Wrap(ErrInvalidTool, "name is required")
	}

	for _, param := range s.Parameters {
		if err := param.Validate(); err != nil {
			return eb.Wrap(ErrInvalidTool, "invalid parameter")
		}
	}

	return nil
}

// ParameterType is the type of a parameter.
// It defines the allowed data types for tool parameters.
type ParameterType string

const (
	// TypeString represents a string parameter.
	// It can be used for text input, file paths, URLs, etc.
	TypeString ParameterType = "string"

	// TypeNumber represents a numeric parameter.
	// It can be used for floating-point numbers.
	TypeNumber ParameterType = "number"

	// TypeInteger represents an integer parameter.
	// It can be used for whole numbers.
	TypeInteger ParameterType = "integer"

	// TypeBoolean represents a boolean parameter.
	// It can be used for true/false values.
	TypeBoolean ParameterType = "boolean"

	// TypeArray represents an array parameter.
	// It can be used for lists of values of the same type.
	TypeArray ParameterType = "array"

	// TypeObject represents an object parameter.
	// It can be used for structured data with multiple fields.
	TypeObject ParameterType = "object"
)

// Parameter is a parameter of a tool.
// It defines the specification and constraints of a single input parameter.
type Parameter struct {
	// Title is the user-friendly name of the parameter.
	// It's optional and can be used to provide a more readable name than the parameter key.
	Title string

	// Type is the type of the parameter.
	// It must be one of the predefined ParameterType values.
	Type ParameterType

	// Description is the description of the parameter.
	// It should explain the purpose and expected format of the parameter.
	Description string

	// Required is the list of required field names when Type is Object.
	// These fields must be provided when the parameter is an object type.
	Required []string

	// Enum is the list of allowed values for the parameter.
	// If specified, the parameter value must be one of these values.
	Enum []string

	// Properties is the properties of the parameter.
	// It's used for object type parameters to define the structure of the object.
	Properties map[string]*Parameter

	// Items is the items of the parameter.
	// It's used for array type parameters to define the type of array elements.
	Items *Parameter

	// Number constraints
	// Minimum and Maximum define the valid range for number type parameters.
	Minimum *float64
	Maximum *float64

	// String constraints
	// MinLength and MaxLength define the valid length range for string type parameters.
	// Pattern defines a regular expression that the string must match.
	MinLength *int
	MaxLength *int
	Pattern   string

	// Array constraints
	// MinItems and MaxItems define the valid size range for array type parameters.
	MinItems *int
	MaxItems *int

	// Default value for the parameter.
	// If not provided, this value will be used when the parameter is omitted.
	Default any
}

// Validate validates the parameter.
func (p *Parameter) Validate() error {
	eb := goerr.NewBuilder(goerr.V("parameter", p))

	// Type is required
	if p.Type == "" {
		return eb.Wrap(ErrInvalidParameter, "type is required")
	}

	// Properties is required for object type
	if p.Type == TypeObject {
		if p.Properties == nil {
			return eb.Wrap(ErrInvalidParameter, "properties is required for object type")
		}
		// Validate nested properties
		for _, prop := range p.Properties {
			if err := prop.Validate(); err != nil {
				return eb.Wrap(ErrInvalidParameter, "invalid property")
			}
		}
		// Validate required fields exist in properties
		for _, req := range p.Required {
			if _, ok := p.Properties[req]; !ok {
				return eb.Wrap(ErrInvalidParameter, "required field not found in properties", goerr.V("field", req))
			}
		}
	}

	// Items is required for array type
	if p.Type == TypeArray {
		if p.Items == nil {
			return eb.Wrap(ErrInvalidParameter, "items is required for array type")
		}
		// Validate items
		if err := p.Items.Validate(); err != nil {
			return eb.Wrap(ErrInvalidParameter, "invalid items")
		}
	}

	// Validate number constraints
	if p.Type == TypeNumber || p.Type == TypeInteger {
		if p.Minimum != nil && p.Maximum != nil && *p.Minimum > *p.Maximum {
			return eb.Wrap(ErrInvalidParameter, "minimum must be less than or equal to maximum")
		}
	}

	// Validate string constraints
	if p.Type == TypeString {
		if p.MinLength != nil && p.MaxLength != nil && *p.MinLength > *p.MaxLength {
			return eb.Wrap(ErrInvalidParameter, "minLength must be less than or equal to maxLength")
		}
		if p.Pattern != "" {
			if _, err := regexp.Compile(p.Pattern); err != nil {
				return eb.Wrap(ErrInvalidParameter, "invalid pattern", goerr.V("pattern", p.Pattern))
			}
		}
	}

	// Validate array constraints
	if p.Type == TypeArray {
		if p.MinItems != nil && p.MaxItems != nil && *p.MinItems > *p.MaxItems {
			return eb.Wrap(ErrInvalidParameter, "minItems must be less than or equal to maxItems")
		}
	}

	return nil
}

// Tool is specification and execution of an action that can be called by the LLM.
// It defines the interface that all tools must implement to be used with gollam.
type Tool interface {
	// Spec returns the specification of the tool.
	// It's called when starting a LLM chat session in Order() to register the tool with the LLM.
	Spec() *ToolSpec

	// Run is the execution of the tool.
	// It's called when receiving a tool call from the LLM.
	// Even if the method returns an error, the tool execution is not aborted.
	// Error will be passed to LLM as a response.
	// If you want to abort the tool execution, you need to return an error from the callback function of WithErrCallback().
	Run(ctx context.Context, args map[string]any) (map[string]any, error)
}

// ToolSet is a set of tools.
// It's useful for providing a set of tools to the LLM.
// A ToolSet can be used to group related tools together and manage them as a single unit.
type ToolSet interface {
	// Specs returns the specifications of the tools.
	// It returns a list of ToolSpec objects that describe all tools in the set.
	Specs() []*ToolSpec

	// Run is the execution of the tool.
	// It's called when receiving a tool call from the LLM.
	// The name parameter identifies which tool in the set should be executed.
	Run(ctx context.Context, name string, args map[string]any) (map[string]any, error)
}
