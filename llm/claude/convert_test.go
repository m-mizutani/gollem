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
				Type: servantic.TypeObject,
				Properties: map[string]*servantic.Parameter{
					"name": {
						Type:        servantic.TypeString,
						Description: "User's name",
						Required:    true,
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
