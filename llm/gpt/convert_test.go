package gpt_test

import (
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/servant/llm"
	"github.com/m-mizutani/servant/llm/gpt"
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
