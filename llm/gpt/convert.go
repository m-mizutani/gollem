package gpt

import (
	"github.com/m-mizutani/servantic"
	"github.com/sashabaranov/go-openai"
)

// ConvertTool converts servantic.Tool to openai.FunctionDefinition
func convertTool(tool servantic.Tool) openai.FunctionDefinition {
	parameters := make(map[string]interface{})
	properties := make(map[string]interface{})
	required := make([]string, 0)
	spec := tool.Spec()

	for name, param := range spec.Parameters {
		properties[name] = convertParameterToSchema(param)
		if param.Required {
			required = append(required, name)
		}
	}

	parameters["type"] = "object"
	parameters["properties"] = properties
	if len(required) > 0 {
		parameters["required"] = required
	}

	return openai.FunctionDefinition{
		Name:        spec.Name,
		Description: spec.Description,
		Parameters:  parameters,
	}
}

func convertParameterToSchema(param *servantic.Parameter) map[string]interface{} {
	schema := map[string]interface{}{
		"type":        getOpenAIType(param.Type),
		"description": param.Description,
	}

	if len(param.Enum) > 0 {
		schema["enum"] = param.Enum
	}

	if param.Properties != nil {
		properties := make(map[string]interface{})
		required := make([]string, 0)
		for name, prop := range param.Properties {
			properties[name] = convertParameterToSchema(prop)
			if prop.Required {
				required = append(required, name)
			}
		}
		schema["properties"] = properties
		if len(required) > 0 {
			schema["required"] = required
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
