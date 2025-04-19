package gemini_test

import (
	"context"
	"testing"

	"cloud.google.com/go/vertexai/genai"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/servantic"
	"github.com/m-mizutani/servantic/llm/gemini"
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
