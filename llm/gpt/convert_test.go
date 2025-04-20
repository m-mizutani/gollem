package gpt_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/servantic"
	"github.com/m-mizutani/servantic/llm/gpt"
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
	openaiTool := gpt.ConvertTool(tool)

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
