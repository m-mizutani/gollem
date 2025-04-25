package claude_test

import (
	"context"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/servantic"
	"github.com/m-mizutani/servantic/llm/claude"
)

type complexTool struct{}

func (t *complexTool) Spec() *servantic.ToolSpec {
	return &servantic.ToolSpec{
		Name:        "complex_tool",
		Description: "A tool with complex parameter structure",
		Parameters: map[string]*servantic.Parameter{
			"user": {
				Type:     servantic.TypeObject,
				Required: []string{"name"},
				Properties: map[string]*servantic.Parameter{
					"name": {
						Type:        servantic.TypeString,
						Description: "User's name",
					},
					"address": {
						Type: servantic.TypeObject,
						Properties: map[string]*servantic.Parameter{
							"street": {
								Type:        servantic.TypeString,
								Description: "Street address",
							},
							"city": {
								Type:        servantic.TypeString,
								Description: "City name",
							},
						},
						Required: []string{"street"},
					},
				},
			},
			"items": {
				Type: servantic.TypeArray,
				Items: &servantic.Parameter{
					Type: servantic.TypeObject,
					Properties: map[string]*servantic.Parameter{
						"id": {
							Type:        servantic.TypeString,
							Description: "Item ID",
						},
						"quantity": {
							Type:        servantic.TypeNumber,
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
	claudeTool := claude.ConvertTool(tool)

	// Check basic properties
	gt.Equal(t, claudeTool.Name, "complex_tool")
	gt.Equal(t, claudeTool.Description, anthropic.String("A tool with complex parameter structure"))

	// Check schema properties
	schema := claudeTool.InputSchema.Properties.(map[string]interface{})

	// Check user parameter
	user := schema["user"].(map[string]interface{})
	gt.Equal(t, user["type"], "object")

	userProps := user["properties"].(map[string]interface{})
	gt.Equal(t, userProps["name"].(map[string]interface{})["type"], "string")
	gt.Equal(t, userProps["name"].(map[string]interface{})["description"], "User's name")
	userRequired := gt.Cast[[]string](t, user["required"])
	gt.Equal(t, userRequired, []string{"name"})

	addressProps := userProps["address"].(map[string]interface{})["properties"].(map[string]interface{})
	gt.Equal(t, addressProps["street"].(map[string]interface{})["type"], "string")
	gt.Equal(t, addressProps["city"].(map[string]interface{})["type"], "string")

	// Check items parameter
	itemsProp := schema["items"].(map[string]interface{})
	gt.Equal(t, itemsProp["type"], "array")

	itemsProps := itemsProp["items"].(map[string]interface{})["properties"].(map[string]interface{})
	gt.Equal(t, itemsProps["id"].(map[string]interface{})["type"], "string")
	gt.Equal(t, itemsProps["quantity"].(map[string]interface{})["type"], "number")
}

func TestConvertParameterToSchema(t *testing.T) {
	t.Run("number constraints", func(t *testing.T) {
		p := &servantic.Parameter{
			Type:    servantic.TypeNumber,
			Minimum: ptr(1.0),
			Maximum: ptr(10.0),
		}
		schema := claude.ConvertParameterToSchema(p)
		gt.Value(t, schema["minimum"]).Equal(1.0)
		gt.Value(t, schema["maximum"]).Equal(10.0)
	})

	t.Run("string constraints", func(t *testing.T) {
		p := &servantic.Parameter{
			Type:      servantic.TypeString,
			MinLength: ptr(1),
			MaxLength: ptr(10),
			Pattern:   "^[a-z]+$",
		}
		schema := claude.ConvertParameterToSchema(p)
		gt.Value(t, schema["minLength"]).Equal(1)
		gt.Value(t, schema["maxLength"]).Equal(10)
		gt.Value(t, schema["pattern"]).Equal("^[a-z]+$")
	})

	t.Run("array constraints", func(t *testing.T) {
		p := &servantic.Parameter{
			Type:     servantic.TypeArray,
			Items:    &servantic.Parameter{Type: servantic.TypeString},
			MinItems: ptr(1),
			MaxItems: ptr(10),
		}
		schema := claude.ConvertParameterToSchema(p)
		gt.Value(t, schema["minItems"]).Equal(1)
		gt.Value(t, schema["maxItems"]).Equal(10)
		gt.Value(t, schema["items"].(map[string]interface{})["type"]).Equal("string")
	})

	t.Run("default value", func(t *testing.T) {
		p := &servantic.Parameter{
			Type:    servantic.TypeString,
			Default: "default value",
		}
		schema := claude.ConvertParameterToSchema(p)
		gt.Value(t, schema["default"]).Equal("default value")
	})
}

func ptr[T any](v T) *T {
	return &v
}
