package gemini_test

import (
	"testing"

	"cloud.google.com/go/vertexai/genai"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/servant/llm"
	"github.com/m-mizutani/servant/llm/gemini"
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
	genaiTool := gemini.ConvertTool(tool)

	gt.Value(t, genaiTool.FunctionDeclarations[0].Name).Equal("complex_tool")
	gt.Value(t, genaiTool.FunctionDeclarations[0].Description).Equal("A tool with complex parameter structure")

	params := genaiTool.FunctionDeclarations[0].Parameters
	gt.Value(t, params.Type).Equal(genai.TypeObject)

	// Check user object
	user := params.Properties["user"]
	gt.Value(t, user.Type).Equal(genai.TypeObject)
	gt.Value(t, user.Properties["name"].Type).Equal(genai.TypeString)
	gt.Value(t, user.Properties["name"].Description).Equal("User's name")
	gt.Value(t, user.Required).Equal([]string{"name"})

	// Check address object
	address := user.Properties["address"]
	gt.Value(t, address.Type).Equal(genai.TypeObject)
	gt.Value(t, address.Properties["street"].Type).Equal(genai.TypeString)
	gt.Value(t, address.Properties["city"].Type).Equal(genai.TypeString)

	// Check items array
	items := params.Properties["items"]
	gt.Value(t, items.Type).Equal(genai.TypeArray)
	gt.Value(t, items.Items.Type).Equal(genai.TypeObject)
	gt.Value(t, items.Items.Properties["id"].Type).Equal(genai.TypeString)
	gt.Value(t, items.Items.Properties["quantity"].Type).Equal(genai.TypeNumber)
}
