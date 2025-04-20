package gpt

import (
	"github.com/m-mizutani/servantic"
	"github.com/sashabaranov/go-openai"
)

// ConvertTool converts servantic.Tool to openai.Tool
func convertTool(tool servantic.Tool) openai.Tool {
	parameters := make(map[string]interface{})
	properties := make(map[string]interface{})
	spec := tool.Spec()

	for name, param := range spec.Parameters {
		properties[name] = convertParameterToSchema(param)
	}

	parameters["type"] = "object"
	parameters["properties"] = properties
	parameters["required"] = spec.Required

	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        spec.Name,
			Description: spec.Description,
			Parameters:  parameters,
		},
	}
}

func convertParameterToSchema(param *servantic.Parameter) map[string]interface{} {
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

	return schema
}

func getOpenAIType(paramType servantic.ParameterType) string {
	switch paramType {
	case servantic.TypeString:
		return "string"
	case servantic.TypeNumber:
		return "number"
	case servantic.TypeInteger:
		return "integer"
	case servantic.TypeBoolean:
		return "boolean"
	case servantic.TypeArray:
		return "array"
	case servantic.TypeObject:
		return "object"
	default:
		return "string"
	}
}
