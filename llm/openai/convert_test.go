package openai

import (
	"context"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
)

type complexTool struct{}

func (t *complexTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "complex_tool",
		Description: "A tool with complex parameter structure",
		Parameters: map[string]*gollem.Parameter{
			"user": {
				Type:     gollem.TypeObject,
				Required: []string{"name"},
				Properties: map[string]*gollem.Parameter{
					"name": {
						Type:        gollem.TypeString,
						Description: "User's name",
					},
					"address": {
						Type: gollem.TypeObject,
						Properties: map[string]*gollem.Parameter{
							"street": {
								Type:        gollem.TypeString,
								Description: "Street address",
							},
							"city": {
								Type:        gollem.TypeString,
								Description: "City name",
							},
						},
					},
				},
			},
			"items": {
				Type: gollem.TypeArray,
				Items: &gollem.Parameter{
					Type: gollem.TypeObject,
					Properties: map[string]*gollem.Parameter{
						"id": {
							Type:        gollem.TypeString,
							Description: "Item ID",
						},
						"quantity": {
							Type:        gollem.TypeNumber,
							Description: "Item quantity",
						},
					},
				},
			},
		},
	}
}

func (t *complexTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return nil, nil
}

func TestConvertTool(t *testing.T) {
	tool := &complexTool{}
	openaiTool := ConvertTool(tool)

	gt.Value(t, openaiTool.Type).Equal("function")
	gt.Value(t, openaiTool.Function.Name).Equal("complex_tool")
	gt.Value(t, openaiTool.Function.Description).Equal("A tool with complex parameter structure")

	params := openaiTool.Function.Parameters.(map[string]interface{})
	gt.Value(t, params["type"]).Equal("object")

	// Check user object
	user := params["properties"].(map[string]interface{})["user"].(map[string]interface{})
	gt.Value(t, user["type"]).Equal("object")
	gt.Value(t, user["properties"].(map[string]interface{})["name"].(map[string]interface{})["type"]).Equal("string")
	gt.Value(t, user["properties"].(map[string]interface{})["name"].(map[string]interface{})["description"]).Equal("User's name")
	gt.Value(t, user["required"]).Equal([]string{"name"})

	// Check address object
	address := user["properties"].(map[string]interface{})["address"].(map[string]interface{})
	gt.Value(t, address["type"]).Equal("object")
	gt.Value(t, address["properties"].(map[string]interface{})["street"].(map[string]interface{})["type"]).Equal("string")
	gt.Value(t, address["properties"].(map[string]interface{})["city"].(map[string]interface{})["type"]).Equal("string")

	// Check items array
	items := params["properties"].(map[string]interface{})["items"].(map[string]interface{})
	gt.Value(t, items["type"]).Equal("array")
	gt.Value(t, items["items"].(map[string]interface{})["type"]).Equal("object")
	gt.Value(t, items["items"].(map[string]interface{})["properties"].(map[string]interface{})["id"].(map[string]interface{})["type"]).Equal("string")
	gt.Value(t, items["items"].(map[string]interface{})["properties"].(map[string]interface{})["quantity"].(map[string]interface{})["type"]).Equal("number")
}

func TestConvertParameterToSchema(t *testing.T) {
	t.Run("number constraints", func(t *testing.T) {
		p := &gollem.Parameter{
			Type:    gollem.TypeNumber,
			Minimum: ptr(1.0),
			Maximum: ptr(10.0),
		}
		schema := convertParameterToSchema(p)
		gt.Value(t, schema["minimum"]).Equal(1.0)
		gt.Value(t, schema["maximum"]).Equal(10.0)
	})

	t.Run("string constraints", func(t *testing.T) {
		p := &gollem.Parameter{
			Type:      gollem.TypeString,
			MinLength: ptr(1),
			MaxLength: ptr(10),
			Pattern:   "^[a-z]+$",
		}
		schema := convertParameterToSchema(p)
		gt.Value(t, schema["minLength"]).Equal(1)
		gt.Value(t, schema["maxLength"]).Equal(10)
		gt.Value(t, schema["pattern"]).Equal("^[a-z]+$")
	})

	t.Run("array constraints", func(t *testing.T) {
		p := &gollem.Parameter{
			Type:     gollem.TypeArray,
			Items:    &gollem.Parameter{Type: gollem.TypeString},
			MinItems: ptr(1),
			MaxItems: ptr(10),
		}
		schema := convertParameterToSchema(p)
		gt.Value(t, schema["minItems"]).Equal(1)
		gt.Value(t, schema["maxItems"]).Equal(10)
		gt.Value(t, schema["items"].(map[string]interface{})["type"]).Equal("string")
	})

	t.Run("default value", func(t *testing.T) {
		p := &gollem.Parameter{
			Type:    gollem.TypeString,
			Default: "default value",
		}
		schema := convertParameterToSchema(p)
		gt.Value(t, schema["default"]).Equal("default value")
	})
}

func ptr[T any](v T) *T {
	return &v
}
