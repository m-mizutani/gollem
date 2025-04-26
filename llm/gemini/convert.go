package gemini

import (
	"cloud.google.com/go/vertexai/genai"
	"github.com/m-mizutani/gollam"
)

// convertTool converts gollam.Tool to Gemini tool
func convertTool(tool gollam.Tool) *genai.FunctionDeclaration {
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

// convertParameterToSchema converts gollam.Parameter to Gemini schema
func convertParameterToSchema(param *gollam.Parameter) *genai.Schema {
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
	if param.Type == gollam.TypeNumber || param.Type == gollam.TypeInteger {
		if param.Minimum != nil {
			schema.Minimum = *param.Minimum
		}
		if param.Maximum != nil {
			schema.Maximum = *param.Maximum
		}
	}

	// Add string constraints
	if param.Type == gollam.TypeString {
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
	if param.Type == gollam.TypeArray {
		if param.MinItems != nil {
			schema.MinItems = int64(*param.MinItems)
		}
		if param.MaxItems != nil {
			schema.MaxItems = int64(*param.MaxItems)
		}
	}

	// No default value in Gemini

	return schema
}

func getGeminiType(paramType gollam.ParameterType) genai.Type {
	switch paramType {
	case gollam.TypeString:
		return genai.TypeString
	case gollam.TypeNumber:
		return genai.TypeNumber
	case gollam.TypeInteger:
		return genai.TypeInteger
	case gollam.TypeBoolean:
		return genai.TypeBoolean
	case gollam.TypeArray:
		return genai.TypeArray
	case gollam.TypeObject:
		return genai.TypeObject
	default:
		return genai.TypeString
	}
}
