package claude_test

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/servant/llm"
	"github.com/m-mizutani/servant/llm/claude"
)

type complexTool struct{}

func (t *complexTool) Name() string {
	return "complex_tool"
}

func (t *complexTool) Description() string {
	return "A tool with complex parameter structure"
}

func (t *complexTool) Parameters() map[string]*llm.Parameter {
	return map[string]*llm.Parameter{
		"user": {
			Type: llm.TypeObject,
			Properties: map[string]*llm.Parameter{
				"name": {
					Type:        llm.TypeString,
					Description: "User's name",
					Required:    true,
				},
				"address": {
					Type: llm.TypeObject,
					Properties: map[string]*llm.Parameter{
						"street": {
							Type:        llm.TypeString,
							Description: "Street address",
						},
						"city": {
							Type:        llm.TypeString,
							Description: "City name",
						},
					},
				},
			},
		},
		"items": {
			Type: llm.TypeArray,
			Items: &llm.Parameter{
				Type: llm.TypeObject,
				Properties: map[string]*llm.Parameter{
					"id": {
						Type:        llm.TypeString,
						Description: "Item ID",
					},
					"quantity": {
						Type:        llm.TypeNumber,
						Description: "Item quantity",
					},
				},
			},
		},
	}
}

func (t *complexTool) Run(args map[string]any) (map[string]any, error) {
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
	gt.Equal(t, schema["type"], "object")
	props := schema["properties"].(map[string]interface{})

	// Check user parameter
	userProp := props["user"].(map[string]interface{})
	gt.Equal(t, userProp["type"], "object")

	userProps := userProp["properties"].(map[string]interface{})
	gt.Equal(t, userProps["name"].(map[string]interface{})["type"], "string")
	gt.Equal(t, userProps["name"].(map[string]interface{})["description"], "User's name")
	gt.Equal(t, userProps["name"].(map[string]interface{})["required"], true)

	addressProps := userProps["address"].(map[string]interface{})["properties"].(map[string]interface{})
	gt.Equal(t, addressProps["street"].(map[string]interface{})["type"], "string")
	gt.Equal(t, addressProps["city"].(map[string]interface{})["type"], "string")

	// Check items parameter
	itemsProp := props["items"].(map[string]interface{})
	gt.Equal(t, itemsProp["type"], "array")

	itemsProps := itemsProp["items"].(map[string]interface{})["properties"].(map[string]interface{})
	gt.Equal(t, itemsProps["id"].(map[string]interface{})["type"], "string")
	gt.Equal(t, itemsProps["quantity"].(map[string]interface{})["type"], "number")
}
