package gemini_test

import (
	"context"
	"testing"

	"cloud.google.com/go/vertexai/genai"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/servant"
	"github.com/m-mizutani/servant/llm/gemini"
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
