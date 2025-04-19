package gemini

import (
	"cloud.google.com/go/vertexai/genai"
	"github.com/m-mizutani/servant"
)

func convertTool(tool servant.Tool) *genai.Tool {
	parameters := &genai.Schema{
		Type:       genai.TypeObject,
		Properties: make(map[string]*genai.Schema),
	}

	spec := tool.Spec()
	for name, param := range spec.Parameters {
		parameters.Properties[name] = convertParameterToSchema(name, param)
	}

	return &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        spec.Name,
				Description: spec.Description,
				Parameters:  parameters,
			},
		},
	}
}

func convertParameterToSchema(name string, param *servant.Parameter) *genai.Schema {
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

func getGenaiType(paramType servant.ParameterType) genai.Type {
	switch paramType {
	case servant.TypeString:
		return genai.TypeString
	case servant.TypeNumber:
		return genai.TypeNumber
	case servant.TypeInteger:
		return genai.TypeInteger
	case servant.TypeBoolean:
		return genai.TypeBoolean
	case servant.TypeArray:
		return genai.TypeArray
	case servant.TypeObject:
		return genai.TypeObject
	default:
		return genai.TypeString
	}
}
