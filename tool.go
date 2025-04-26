package gollam

import (
	"context"
	"regexp"

	"github.com/m-mizutani/goerr/v2"
)

// ToolSpec is the specification of a tool.
type ToolSpec struct {
	Name        string
	Description string
	Parameters  map[string]*Parameter
	Required    []string
}

// Validate validates the tool specification.
func (s *ToolSpec) Validate() error {
	eb := goerr.NewBuilder(goerr.V("tool", s))
	if s.Name == "" {
		return eb.Wrap(ErrInvalidTool, "name is required")
	}

	paramNames := make(map[string]struct{})
	for name, param := range s.Parameters {
		if _, ok := paramNames[name]; ok {
			return eb.Wrap(ErrInvalidTool, "duplicate parameter name", goerr.V("name", name))
		}
		paramNames[name] = struct{}{}

		if err := param.Validate(); err != nil {
			return eb.Wrap(ErrInvalidTool, "invalid parameter")
		}
	}

	for _, req := range s.Required {
		if _, ok := paramNames[req]; !ok {
			return eb.Wrap(ErrInvalidTool, "required parameter not found", goerr.V("name", req))
		}
	}

	return nil
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
	// Title is the user friendly  of the parameter. It's optional.
	Title string

	// Type is the type of the parameter. It's required.
	Type ParameterType

	// Description is the description of the parameter. It's optional.
	Description string

	// Required is the list of required field names when Type is Object.
	Required []string

	// Enum is the enum of the parameter. It's optional.
	Enum []string

	// Properties is the properties of the parameter. It's used for object type.
	Properties map[string]*Parameter

	// Items is the items of the parameter. It's used for array type.
	Items *Parameter

	// Number constraints
	Minimum *float64
	Maximum *float64

	// String constraints
	MinLength *int
	MaxLength *int
	Pattern   string

	// Array constraints
	MinItems *int
	MaxItems *int

	// Default value
	Default any
}

// Validate validates the parameter.
func (p *Parameter) Validate() error {
	eb := goerr.NewBuilder(goerr.V("parameter", p))

	// Type is required
	if p.Type == "" {
		return eb.Wrap(ErrInvalidParameter, "type is required")
	}

	// Validate parameter type
	switch p.Type {
	case TypeString, TypeNumber, TypeInteger, TypeBoolean, TypeArray, TypeObject:
		// Valid type
	default:
		return eb.Wrap(ErrInvalidParameter, "invalid parameter type", goerr.V("type", p.Type))
	}

	// Properties is required for object type
	if p.Type == TypeObject {
		if p.Properties == nil {
			return eb.Wrap(ErrInvalidParameter, "properties is required for object type")
		}

		// Check for duplicate property names
		propNames := make(map[string]struct{})
		for name := range p.Properties {
			if _, ok := propNames[name]; ok {
				return eb.Wrap(ErrInvalidParameter, "duplicate property name", goerr.V("name", name))
			}
			propNames[name] = struct{}{}
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
type Tool interface {
	// Spec returns the specification of the tool. It's called when starting a LLM chat session in Order().
	Spec() ToolSpec

	// Run is the execution of the tool.
	// It's called when receiving a tool call from the LLM. Even if the method returns an error, the tool execution is not aborted. Error will be passed to LLM as a response. If you want to abort the tool execution, you need to return an error from the callback function of WithErrCallback().
	Run(ctx context.Context, args map[string]any) (map[string]any, error)
}

// ToolSet is a set of tools.
// It's useful for providing a set of tools to the LLM.
type ToolSet interface {
	// Specs returns the specifications of the tools.
	Specs() []ToolSpec

	// Run is the execution of the tool.
	// It's called when receiving a tool call from the LLM.
	Run(ctx context.Context, name string, args map[string]any) (map[string]any, error)
}
