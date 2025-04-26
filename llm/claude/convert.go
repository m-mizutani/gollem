package claude

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/m-mizutani/gollam"
)

func convertTool(tool gollam.Tool) anthropic.ToolUnionParam {
	spec := tool.Spec()
	schema := convertParametersToJSONSchema(spec.Parameters)

	return anthropic.ToolUnionParamOfTool(
		anthropic.ToolInputSchemaParam{
			Properties: schema.Properties,
		},
		spec.Name,
	)
}

type jsonSchema struct {
	Type        string                `json:"type"`
	Properties  map[string]jsonSchema `json:"properties,omitempty"`
	Required    []string              `json:"required,omitempty"`
	Items       *jsonSchema           `json:"items,omitempty"`
	Minimum     *float64              `json:"minimum,omitempty"`
	Maximum     *float64              `json:"maximum,omitempty"`
	MinLength   *int                  `json:"minLength,omitempty"`
	MaxLength   *int                  `json:"maxLength,omitempty"`
	Pattern     string                `json:"pattern,omitempty"`
	MinItems    *int                  `json:"minItems,omitempty"`
	MaxItems    *int                  `json:"maxItems,omitempty"`
	Default     interface{}           `json:"default,omitempty"`
	Enum        []interface{}         `json:"enum,omitempty"`
	Description string                `json:"description,omitempty"`
	Title       string                `json:"title,omitempty"`
}

func convertParametersToJSONSchema(params map[string]*gollam.Parameter) jsonSchema {
	properties := make(map[string]jsonSchema)

	for name, param := range params {
		properties[name] = convertParameterToSchema(param)
	}

	return jsonSchema{
		Type:       "object",
		Properties: properties,
	}
}

// convertParameterToSchema converts gollam.Parameter to Claude schema
func convertParameterToSchema(param *gollam.Parameter) jsonSchema {
	schema := jsonSchema{
		Type:        getClaudeType(param.Type),
		Description: param.Description,
		Title:       param.Title,
	}

	if len(param.Enum) > 0 {
		enum := make([]interface{}, len(param.Enum))
		for i, v := range param.Enum {
			enum[i] = v
		}
		schema.Enum = enum
	}

	if param.Properties != nil {
		properties := make(map[string]jsonSchema)
		for name, prop := range param.Properties {
			properties[name] = convertParameterToSchema(prop)
		}
		schema.Properties = properties
		if len(param.Required) > 0 {
			schema.Required = param.Required
		}
	}

	if param.Items != nil {
		items := convertParameterToSchema(param.Items)
		schema.Items = &items
	}

	// Add number constraints
	if param.Type == gollam.TypeNumber || param.Type == gollam.TypeInteger {
		if param.Minimum != nil {
			schema.Minimum = param.Minimum
		}
		if param.Maximum != nil {
			schema.Maximum = param.Maximum
		}
	}

	// Add string constraints
	if param.Type == gollam.TypeString {
		if param.MinLength != nil {
			schema.MinLength = param.MinLength
		}
		if param.MaxLength != nil {
			schema.MaxLength = param.MaxLength
		}
		if param.Pattern != "" {
			schema.Pattern = param.Pattern
		}
	}

	// Add array constraints
	if param.Type == gollam.TypeArray {
		if param.MinItems != nil {
			schema.MinItems = param.MinItems
		}
		if param.MaxItems != nil {
			schema.MaxItems = param.MaxItems
		}
	}

	// Add default value
	if param.Default != nil {
		schema.Default = param.Default
	}

	return schema
}

func getClaudeType(paramType gollam.ParameterType) string {
	switch paramType {
	case gollam.TypeString:
		return "string"
	case gollam.TypeNumber:
		return "number"
	case gollam.TypeInteger:
		return "integer"
	case gollam.TypeBoolean:
		return "boolean"
	case gollam.TypeArray:
		return "array"
	case gollam.TypeObject:
		return "object"
	default:
		return "string"
	}
}
