package claude_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gollam"
	"github.com/m-mizutani/gollam/llm/claude"
	"github.com/m-mizutani/gt"
)

type complexTool struct{}

func (t *complexTool) Spec() gollam.ToolSpec {
	return gollam.ToolSpec{
		Name:        "complex_tool",
		Description: "A tool with complex parameter structure",
		Required:    []string{"user"},
		Parameters: map[string]*gollam.Parameter{
			"user": {
				Type:     gollam.TypeObject,
				Required: []string{"name"},
				Properties: map[string]*gollam.Parameter{
					"name": {
						Type:        gollam.TypeString,
						Description: "User's name",
					},
					"address": {
						Type: gollam.TypeObject,
						Properties: map[string]*gollam.Parameter{
							"street": {
								Type:        gollam.TypeString,
								Description: "Street address",
							},
							"city": {
								Type:        gollam.TypeString,
								Description: "City name",
							},
						},
						Required: []string{"street"},
					},
				},
			},
			"items": {
				Type: gollam.TypeArray,
				Items: &gollam.Parameter{
					Type: gollam.TypeObject,
					Properties: map[string]*gollam.Parameter{
						"id": {
							Type:        gollam.TypeString,
							Description: "Item ID",
						},
						"quantity": {
							Type:        gollam.TypeNumber,
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
	gt.Equal(t, claudeTool.OfTool.Name, "complex_tool")

	// Check schema properties
	schemaProps := claudeTool.OfTool.InputSchema.Properties.(map[string]claude.JsonSchema)

	// Check user parameter
	user := schemaProps["user"]
	gt.Equal(t, user.Type, "object")

	userProps := user.Properties
	nameProps := userProps["name"]
	gt.Equal(t, nameProps.Type, "string")
	gt.Equal(t, nameProps.Description, "User's name")
	gt.Array(t, user.Required).Equal([]string{"name"})

	addressProps := userProps["address"].Properties
	gt.Equal(t, addressProps["street"].Type, "string")
	gt.Equal(t, addressProps["city"].Type, "string")

	// Check items parameter
	itemsProp := schemaProps["items"]
	gt.Equal(t, itemsProp.Type, "array")

	itemsSchema := *itemsProp.Items
	itemsProps := itemsSchema.Properties
	gt.Equal(t, itemsProps["id"].Type, "string")
	gt.Equal(t, itemsProps["quantity"].Type, "number")
}

func TestConvertParameterToSchema(t *testing.T) {
	type testCase struct {
		name     string
		schema   *gollam.Parameter
		expected claude.JsonSchema
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			actual := claude.ConvertParameterToSchema(tc.schema)
			gt.Value(t, actual).Equal(tc.expected)
		}
	}

	t.Run("number constraints", runTest(testCase{
		name: "number constraints",
		schema: &gollam.Parameter{
			Type:    gollam.TypeNumber,
			Minimum: ptr(1.0),
			Maximum: ptr(10.0),
		},
		expected: claude.JsonSchema{
			Type:    "number",
			Minimum: ptr(1.0),
			Maximum: ptr(10.0),
		},
	}))

	t.Run("string constraints", runTest(testCase{
		name: "string constraints",
		schema: &gollam.Parameter{
			Type:      gollam.TypeString,
			MinLength: ptr(1),
			MaxLength: ptr(10),
			Pattern:   "^[a-z]+$",
		},
		expected: claude.JsonSchema{
			Type:      "string",
			MinLength: ptr(1),
			MaxLength: ptr(10),
			Pattern:   "^[a-z]+$",
		},
	}))

	t.Run("array constraints", runTest(testCase{
		name: "array constraints",
		schema: &gollam.Parameter{
			Type:     gollam.TypeArray,
			Items:    &gollam.Parameter{Type: gollam.TypeString},
			MinItems: ptr(1),
			MaxItems: ptr(10),
		},
		expected: claude.JsonSchema{
			Type:     "array",
			MinItems: ptr(1),
			MaxItems: ptr(10),
			Items:    &claude.JsonSchema{Type: "string"},
		},
	}))

	t.Run("default value", runTest(testCase{
		name: "default value",
		schema: &gollam.Parameter{
			Type:    gollam.TypeString,
			Default: "default value",
		},
		expected: claude.JsonSchema{
			Type:    "string",
			Default: "default value",
		},
	}))
}

func ptr[T any](v T) *T {
	return &v
}

func TestConvertSchema(t *testing.T) {
	type testCase struct {
		name     string
		schema   *gollam.Parameter
		expected claude.JsonSchema
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			actual := claude.ConvertParameterToSchema(tc.schema)
			gt.Value(t, actual).Equal(tc.expected)
		}
	}

	t.Run("string type", runTest(testCase{
		name: "string type",
		schema: &gollam.Parameter{
			Type: gollam.TypeString,
		},
		expected: claude.JsonSchema{
			Type: "string",
		},
	}))
}
