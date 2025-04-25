package gemini

import (
	"cloud.google.com/go/vertexai/genai"
	"github.com/m-mizutani/servantic"
)

// convertTool converts servantic.Tool to Gemini tool
func convertTool(tool servantic.Tool) *genai.FunctionDeclaration {
	spec := tool.Spec()
	parameters := &genai.Schema{
		Type:       genai.TypeObject,
		Properties: make(map[string]*genai.Schema),
		Required:   spec.Required,
	}

	for name, param := range spec.Parameters {
		parameters.Properties[name] = convertParameterToSchema(param)
	}

	return &genai.FunctionDeclaration{
		Name:        spec.Name,
		Description: spec.Description,
		Parameters:  parameters,
	}
}

// convertParameterToSchema converts servantic.Parameter to Gemini schema
func convertParameterToSchema(param *servantic.Parameter) *genai.Schema {
	schema := &genai.Schema{
		Type:        getGeminiType(param.Type),
		Description: param.Description,
		Title:       param.Title,
	}

	if len(param.Enum) > 0 {
		schema.Enum = param.Enum
	}

	if param.Properties != nil {
		schema.Properties = make(map[string]*genai.Schema)
		for name, prop := range param.Properties {
			schema.Properties[name] = convertParameterToSchema(prop)
		}
		if len(param.Required) > 0 {
			schema.Required = param.Required
		}
	}

	if param.Items != nil {
		schema.Items = convertParameterToSchema(param.Items)
	}

	// Add number constraints
	if param.Type == servantic.TypeNumber || param.Type == servantic.TypeInteger {
		if param.Minimum != nil {
			schema.Minimum = *param.Minimum
		}
		if param.Maximum != nil {
			schema.Maximum = *param.Maximum
		}
	}

	// Add string constraints
	if param.Type == servantic.TypeString {
		if param.MinLength != nil {
			schema.MinLength = int64(*param.MinLength)
		}
		if param.MaxLength != nil {
			schema.MaxLength = int64(*param.MaxLength)
		}
		if param.Pattern != "" {
			schema.Pattern = param.Pattern
		}
	}

	// Add array constraints
	if param.Type == servantic.TypeArray {
		if param.MinItems != nil {
			schema.MinItems = int64(*param.MinItems)
		}
		if param.MaxItems != nil {
			schema.MaxItems = int64(*param.MaxItems)
		}
	}

	// Add default value
	if param.Default != nil {
		schema.Default = param.Default
	}

	return schema
}

func getGeminiType(paramType servantic.ParameterType) genai.Type {
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
