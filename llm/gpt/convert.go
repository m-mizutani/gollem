package gpt

import (
	"github.com/m-mizutani/servant/llm"
	"github.com/sashabaranov/go-openai"
)

// ConvertTool converts llm.Tool to openai.FunctionDefinition
func convertTool(tool llm.Tool) openai.FunctionDefinition {
	parameters := make(map[string]interface{})
	properties := make(map[string]interface{})
	required := make([]string, 0)

	for name, param := range tool.Parameters() {
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
		Name:        tool.Name(),
		Description: tool.Description(),
		Parameters:  parameters,
	}
}

func convertParameterToSchema(param *llm.Parameter) map[string]interface{} {
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

func getOpenAIType(paramType llm.ParameterType) string {
	switch paramType {
	case llm.TypeString:
		return "string"
	case llm.TypeNumber:
		return "number"
	case llm.TypeInteger:
		return "integer"
	case llm.TypeBoolean:
		return "boolean"
	case llm.TypeArray:
		return "array"
	case llm.TypeObject:
		return "object"
	default:
		return "string"
	}
}
