package claude

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/m-mizutani/servant"
)

func convertTool(tool servant.Tool) *anthropic.ToolParam {
	spec := tool.Spec()
	schema := convertParametersToJSONSchema(spec.Parameters)

	return &anthropic.ToolParam{
		Name:        spec.Name,
		Description: anthropic.String(spec.Description),
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

func convertParametersToJSONSchema(params map[string]*servant.Parameter) jsonSchema {
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

func convertParameterToSchema(param *servant.Parameter) map[string]interface{} {
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

func getClaudeType(paramType servant.ParameterType) string {
	switch paramType {
	case servant.TypeString:
		return "string"
	case servant.TypeNumber:
		return "number"
	case servant.TypeInteger:
		return "integer"
	case servant.TypeBoolean:
		return "boolean"
	case servant.TypeArray:
		return "array"
	case servant.TypeObject:
		return "object"
	default:
		return "string"
	}
}
