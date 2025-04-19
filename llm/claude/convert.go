package claude

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/m-mizutani/servant/llm"
)

func convertTool(tool llm.Tool) *anthropic.ToolParam {
	schema := convertParametersToJSONSchema(tool.Parameters())

	return &anthropic.ToolParam{
		Name:        tool.Name(),
		Description: anthropic.String(tool.Description()),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: schema.Properties,
		},
	}
}

type jsonSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required,omitempty"`
}

func convertParametersToJSONSchema(params map[string]*llm.Parameter) jsonSchema {
	properties := make(map[string]interface{})
	required := make([]string, 0)

	for name, param := range params {
		schema := convertParameterToSchema(param)
		properties[name] = schema

		if param.Required {
			required = append(required, name)
		}
	}

	return jsonSchema{
		Type:       "object",
		Properties: properties,
		Required:   required,
	}
}

func convertParameterToSchema(param *llm.Parameter) map[string]interface{} {
	schema := map[string]interface{}{
		"type":        getClaudeType(param.Type),
		"description": param.Description,
	}

	if len(param.Enum) > 0 {
		schema["enum"] = param.Enum
	}

	var required []string
	if param.Properties != nil {
		properties := make(map[string]interface{})
		for name, prop := range param.Properties {
			properties[name] = convertParameterToSchema(prop)
			if prop.Required {
				required = append(required, name)
			}
		}

		schema["properties"] = properties
		schema["required"] = required
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
