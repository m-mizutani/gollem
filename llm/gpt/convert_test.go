package gpt_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/servant"
	"github.com/m-mizutani/servant/llm/gpt"
)

type complexTool struct{}

func (t *complexTool) Spec() *servant.ToolSpec {
	return &servant.ToolSpec{
		Name:        "complex_tool",
		Description: "A tool with complex parameter structure",
		Parameters: map[string]*servant.Parameter{
			"user": {
				Type: servant.TypeObject,
				Properties: map[string]*servant.Parameter{
					"name": {
						Type:        servant.TypeString,
						Description: "User's name",
						Required:    true,
					},
					"address": {
						Type: servant.TypeObject,
						Properties: map[string]*servant.Parameter{
							"street": {
								Type:        servant.TypeString,
								Description: "Street address",
							},
							"city": {
								Type:        servant.TypeString,
								Description: "City name",
							},
						},
					},
				},
			},
			"items": {
				Type: servant.TypeArray,
				Items: &servant.Parameter{
					Type: servant.TypeObject,
					Properties: map[string]*servant.Parameter{
						"id": {
							Type:        servant.TypeString,
							Description: "Item ID",
						},
						"quantity": {
							Type:        servant.TypeNumber,
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
	openaiTool := gpt.ConvertTool(tool)

	gt.Value(t, openaiTool.Name).Equal("complex_tool")
	gt.Value(t, openaiTool.Description).Equal("A tool with complex parameter structure")

	params := openaiTool.Parameters.(map[string]interface{})
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
