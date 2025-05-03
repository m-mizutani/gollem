package gemini_test

import (
	"context"
	"testing"

	"cloud.google.com/go/vertexai/genai"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/gemini"
	"github.com/m-mizutani/gt"
)

type complexTool struct{}

func (t *complexTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "complex_tool",
		Description: "A tool with complex parameter structure",
		Required:    []string{"user", "items"},
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
	genaiTool := gemini.ConvertTool(tool)

	gt.Value(t, genaiTool.Name).Equal("complex_tool")
	gt.Value(t, genaiTool.Description).Equal("A tool with complex parameter structure")

	params := genaiTool.Parameters
	gt.Value(t, params.Type).Equal(genai.TypeObject)
	gt.Value(t, params.Required).Equal([]string{"user", "items"})

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

func TestConvertParameterToSchema(t *testing.T) {
	t.Run("number constraints", func(t *testing.T) {
		p := &gollem.Parameter{
			Type:    gollem.TypeNumber,
			Minimum: ptr(1.0),
			Maximum: ptr(10.0),
		}
		schema := gemini.ConvertParameterToSchema(p)
		gt.Value(t, schema.Minimum).Equal(1.0)
		gt.Value(t, schema.Maximum).Equal(10.0)
	})

	t.Run("string constraints", func(t *testing.T) {
		p := &gollem.Parameter{
			Type:      gollem.TypeString,
			MinLength: ptr(1),
			MaxLength: ptr(10),
			Pattern:   "^[a-z]+$",
		}
		schema := gemini.ConvertParameterToSchema(p)
		gt.Value(t, schema.MinLength).Equal(int64(1))
		gt.Value(t, schema.MaxLength).Equal(int64(10))
		gt.Value(t, schema.Pattern).Equal("^[a-z]+$")
	})

	t.Run("array constraints", func(t *testing.T) {
		p := &gollem.Parameter{
			Type:     gollem.TypeArray,
			Items:    &gollem.Parameter{Type: gollem.TypeString},
			MinItems: ptr(1),
			MaxItems: ptr(10),
		}
		schema := gemini.ConvertParameterToSchema(p)
		gt.Value(t, schema.MinItems).Equal(int64(1))
		gt.Value(t, schema.MaxItems).Equal(int64(10))
		gt.Value(t, schema.Items.Type).Equal(genai.TypeString)
	})
}

func ptr[T any](v T) *T {
	return &v
}
