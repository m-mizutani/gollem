package claude

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/m-mizutani/servant/llm"
)

// ConvertTool converts llm.Tool to anthropic.Tool
func ConvertTool(tool llm.Tool) anthropic.ToolUnionParam {
	properties := make(map[string]interface{})

	for name, param := range tool.Parameters() {
		schema := convertParameterToSchema(param)
		if param.Required {
			schema["required"] = true
		}
		properties[name] = schema
	}

	return anthropic.ToolUnionParamOfTool(
		anthropic.ToolInputSchemaParam{
			Type:       "object",
			Properties: properties,
		},
		tool.Name(),
	)
}

func convertParameterToSchema(param *llm.Parameter) map[string]interface{} {
	schema := map[string]interface{}{
		"type":        getClaudeType(param.Type),
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

func getClaudeType(paramType llm.ParameterType) string {
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
