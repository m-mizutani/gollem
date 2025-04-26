package gpt

import (
	"github.com/m-mizutani/gollam"
	"github.com/sashabaranov/go-openai"
)

// convertTool converts gollam.Tool to openai.Tool
func convertTool(tool gollam.Tool) openai.Tool {
	parameters := make(map[string]interface{})
	properties := make(map[string]interface{})
	spec := tool.Spec()

	for name, param := range spec.Parameters {
		properties[name] = convertParameterToSchema(param)
	}

	if len(properties) > 0 {
		parameters["type"] = "object"
		parameters["properties"] = properties
		parameters["required"] = spec.Required
	}

	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        spec.Name,
			Description: spec.Description,
			Parameters:  parameters,
		},
	}
}

// convertParameterToSchema converts gollam.Parameter to OpenAI schema
func convertParameterToSchema(param *gollam.Parameter) map[string]interface{} {
	schema := map[string]interface{}{
		"type":        getOpenAIType(param.Type),
		"description": param.Description,
		"title":       param.Title,
	}

	if len(param.Enum) > 0 {
		schema["enum"] = param.Enum
	}

	if param.Properties != nil {
		properties := make(map[string]interface{})
		for name, prop := range param.Properties {
			properties[name] = convertParameterToSchema(prop)
		}
		schema["properties"] = properties
		if len(param.Required) > 0 {
			schema["required"] = param.Required
		}
	}

	if param.Items != nil {
		schema["items"] = convertParameterToSchema(param.Items)
	}

	// Add number constraints
	if param.Type == gollam.TypeNumber || param.Type == gollam.TypeInteger {
		if param.Minimum != nil {
			schema["minimum"] = *param.Minimum
		}
		if param.Maximum != nil {
			schema["maximum"] = *param.Maximum
		}
	}

	// Add string constraints
	if param.Type == gollam.TypeString {
		if param.MinLength != nil {
			schema["minLength"] = *param.MinLength
		}
		if param.MaxLength != nil {
			schema["maxLength"] = *param.MaxLength
		}
		if param.Pattern != "" {
			schema["pattern"] = param.Pattern
		}
	}

	// Add array constraints
	if param.Type == gollam.TypeArray {
		if param.MinItems != nil {
			schema["minItems"] = *param.MinItems
		}
		if param.MaxItems != nil {
			schema["maxItems"] = *param.MaxItems
		}
	}

	// Add default value
	if param.Default != nil {
		schema["default"] = param.Default
	}

	return schema
}

func getOpenAIType(paramType gollam.ParameterType) string {
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
