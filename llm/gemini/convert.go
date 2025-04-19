package gemini

import (
	"cloud.google.com/go/vertexai/genai"
	"github.com/m-mizutani/servant/llm"
)

func convertTool(tool llm.Tool) *genai.Tool {
	parameters := &genai.Schema{
		Type:       genai.TypeObject,
		Properties: make(map[string]*genai.Schema),
	}

	for name, param := range tool.Parameters() {
		parameters.Properties[name] = convertParameterToSchema(name, param)
	}

	return &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  parameters,
			},
		},
	}
}

func convertParameterToSchema(name string, param *llm.Parameter) *genai.Schema {
	schema := &genai.Schema{
		Type:        getGenaiType(param.Type),
		Description: param.Description,
	}

	if param.Required {
		schema.Required = []string{name}
	}

	if len(param.Enum) > 0 {
		schema.Enum = param.Enum
	}

	if param.Properties != nil {
		schema.Properties = make(map[string]*genai.Schema)
		required := make([]string, 0)
		for propName, prop := range param.Properties {
			schema.Properties[propName] = convertParameterToSchema(propName, prop)
			if prop.Required {
				required = append(required, propName)
			}
		}
		if len(required) > 0 {
			schema.Required = required
		}
	}

	if param.Items != nil {
		schema.Items = convertParameterToSchema("", param.Items)
	}

	return schema
}

func getGenaiType(paramType llm.ParameterType) genai.Type {
	switch paramType {
	case llm.TypeString:
		return genai.TypeString
	case llm.TypeNumber:
		return genai.TypeNumber
	case llm.TypeInteger:
		return genai.TypeInteger
	case llm.TypeBoolean:
		return genai.TypeBoolean
	case llm.TypeArray:
		return genai.TypeArray
	case llm.TypeObject:
		return genai.TypeObject
	default:
		return genai.TypeString
	}
}
