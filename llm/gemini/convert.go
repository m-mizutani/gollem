package gemini

import (
	"cloud.google.com/go/vertexai/genai"
	"github.com/m-mizutani/servantic"
)

func convertTool(tool servantic.Tool) *genai.FunctionDeclaration {
	spec := tool.Spec()

	parameters := &genai.Schema{
		Type:       genai.TypeObject,
		Properties: make(map[string]*genai.Schema),
		Required:   spec.Required,
	}

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
		Title:       param.Title,
		Description: param.Description,
	}

	if param.Properties != nil {
		schema.Properties = make(map[string]*genai.Schema)
		for propName, prop := range param.Properties {
			schema.Properties[propName] = convertParameterToSchema(propName, prop)
		}
		if len(param.Required) > 0 {
			schema.Required = param.Required
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
