package gemini

import (
	"cloud.google.com/go/vertexai/genai"
	"github.com/m-mizutani/servantic"
)

func convertTool(tool servantic.Tool) *genai.FunctionDeclaration {
	parameters := &genai.Schema{
		Type:       genai.TypeObject,
		Properties: make(map[string]*genai.Schema),
	}

	spec := tool.Spec()
	for name, param := range spec.Parameters {
		parameters.Properties[name] = convertParameterToSchema(name, param)
	}

	return &genai.FunctionDeclaration{
		Name:        spec.Name,
		Description: spec.Description,
		Parameters:  parameters,
	}
}

func convertParameterToSchema(name string, param *servantic.Parameter) *genai.Schema {
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

func getGenaiType(paramType servantic.ParameterType) genai.Type {
	switch paramType {
	case servantic.TypeString:
		return genai.TypeString
	case servantic.TypeNumber:
		return genai.TypeNumber
	case servantic.TypeInteger:
		return genai.TypeInteger
	case servantic.TypeBoolean:
		return genai.TypeBoolean
	case servantic.TypeArray:
		return genai.TypeArray
	case servantic.TypeObject:
		return genai.TypeObject
	default:
		return genai.TypeString
	}
}
