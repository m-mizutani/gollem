package claude

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/m-mizutani/servantic"
)

func convertTool(tool servantic.Tool) *anthropic.ToolParam {
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

func convertParametersToJSONSchema(params map[string]*servantic.Parameter) jsonSchema {
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

func convertParameterToSchema(param *servantic.Parameter) map[string]interface{} {
	schema := map[string]interface{}{
		"type":        getClaudeType(param.Type),
		"title":       param.Title,
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

func getClaudeType(paramType servantic.ParameterType) string {
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
