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

// Tests moved from schema_validation_test.go

func TestComplexSchemaValidation(t *testing.T) {
	tool := &complexSchemaTestTool{}
	converted := gemini.ConvertTool(tool)

	t.Logf("Complex schema tool structure:")
	t.Logf("Name: %s", converted.Name)
	t.Logf("Description: %s", converted.Description)

	// Check root parameters
	rootParams := converted.Parameters
	gt.Value(t, rootParams.Type).Equal(genai.TypeObject)
	gt.Value(t, rootParams.Required).Equal([]string{"config"})

	// Check config object
	config := rootParams.Properties["config"]
	gt.Value(t, config).NotEqual(nil)
	gt.Value(t, config.Type).Equal(genai.TypeObject)
	gt.Value(t, config.Required).Equal([]string{"required_field"})

	// Check nested object without explicit Required field
	optionalNested := config.Properties["optional_nested"]
	gt.Value(t, optionalNested).NotEqual(nil)
	gt.Value(t, optionalNested.Type).Equal(genai.TypeObject)

	// Critical: This should be an empty slice, not nil
	if optionalNested.Required == nil {
		t.Errorf("CRITICAL: optional_nested.Required is nil, should be empty slice")
	} else {
		gt.Value(t, optionalNested.Required).Equal([]string{})
		t.Logf("SUCCESS: optional_nested.Required is empty slice: %v", optionalNested.Required)
	}

	// Check array items object
	arrayField := config.Properties["array_field"]
	gt.Value(t, arrayField).NotEqual(nil)
	gt.Value(t, arrayField.Type).Equal(genai.TypeArray)
	gt.Value(t, arrayField.Items).NotEqual(nil)
	gt.Value(t, arrayField.Items.Type).Equal(genai.TypeObject)

	// Critical: Array items object Required field should also be empty slice
	if arrayField.Items.Required == nil {
		t.Errorf("CRITICAL: array_field.Items.Required is nil, should be empty slice")
	} else {
		gt.Value(t, arrayField.Items.Required).Equal([]string{})
		t.Logf("SUCCESS: array_field.Items.Required is empty slice: %v", arrayField.Items.Required)
	}
}

func TestConstraintsValidation(t *testing.T) {
	tool := &constraintsTestTool{}
	converted := gemini.ConvertTool(tool)

	t.Logf("Constraints tool structure:")

	// Check string constraints
	constrainedString := converted.Parameters.Properties["constrained_string"]
	gt.Value(t, constrainedString).NotEqual(nil)
	gt.Value(t, constrainedString.Type).Equal(genai.TypeString)
	gt.Value(t, constrainedString.MinLength).Equal(int64(1))
	gt.Value(t, constrainedString.MaxLength).Equal(int64(100))
	gt.Value(t, constrainedString.Pattern).Equal("^[a-zA-Z0-9]+$")

	// Check number constraints
	constrainedNumber := converted.Parameters.Properties["constrained_number"]
	gt.Value(t, constrainedNumber).NotEqual(nil)
	gt.Value(t, constrainedNumber.Type).Equal(genai.TypeNumber)
	gt.Value(t, constrainedNumber.Minimum).Equal(0.0)
	gt.Value(t, constrainedNumber.Maximum).Equal(100.0)

	// Check array constraints
	constrainedArray := converted.Parameters.Properties["constrained_array"]
	gt.Value(t, constrainedArray).NotEqual(nil)
	gt.Value(t, constrainedArray.Type).Equal(genai.TypeArray)
	gt.Value(t, constrainedArray.MinItems).Equal(int64(1))
	gt.Value(t, constrainedArray.MaxItems).Equal(int64(10))
	gt.Value(t, constrainedArray.Items.Type).Equal(genai.TypeString)

	// Check enum field
	enumField := converted.Parameters.Properties["enum_field"]
	gt.Value(t, enumField).NotEqual(nil)
	gt.Value(t, enumField.Type).Equal(genai.TypeString)
	gt.Value(t, enumField.Enum).Equal([]string{"option1", "option2", "option3"})
}

func TestEmptyParametersValidation(t *testing.T) {
	tool := &emptyParametersTool{}
	converted := gemini.ConvertTool(tool)

	t.Logf("Empty parameters tool structure:")

	// Check that empty parameters work correctly
	gt.Value(t, converted.Name).Equal("empty_params")
	gt.Value(t, converted.Parameters.Type).Equal(genai.TypeObject)
	gt.Value(t, len(converted.Parameters.Properties)).Equal(0)
	gt.Value(t, converted.Parameters.Required).Equal([]string{})
}

// Test schema validation against OpenAPI 3.0 requirements
func TestOpenAPICompliance(t *testing.T) {
	tool := &complexSchemaTestTool{}
	converted := gemini.ConvertTool(tool)

	// OpenAPI 3.0 compliance checks
	var validateSchema func(schema *genai.Schema, path string)
	validateSchema = func(schema *genai.Schema, path string) {
		// Every schema must have a valid Type
		gt.Value(t, schema.Type).NotEqual(genai.TypeUnspecified)

		// Object types must have Properties and Required fields
		if schema.Type == genai.TypeObject {
			gt.Value(t, schema.Properties).NotEqual(nil)
			gt.Value(t, schema.Required).NotEqual(nil) // This is critical!

			// Validate nested properties
			for propName, propSchema := range schema.Properties {
				validateSchema(propSchema, path+"."+propName)
			}
		}

		// Array types must have Items
		if schema.Type == genai.TypeArray {
			gt.Value(t, schema.Items).NotEqual(nil)
			validateSchema(schema.Items, path+"[]")
		}

		t.Logf("Schema at %s is valid: Type=%s, Required=%v",
			path, schema.Type, schema.Required)
	}

	validateSchema(converted.Parameters, "root")
}

// Tool definitions for schema validation testing

type complexSchemaTestTool struct{}

func (t *complexSchemaTestTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "complex_schema_test",
		Description: "Tool to test complex schema structures that might cause Gemini validation issues",
		Required:    []string{"config"},
		Parameters: map[string]*gollem.Parameter{
			"config": {
				Type:        gollem.TypeObject,
				Description: "Complex configuration object",
				Required:    []string{"required_field"},
				Properties: map[string]*gollem.Parameter{
					"required_field": {
						Type:        gollem.TypeString,
						Description: "A required field in the config",
					},
					"optional_nested": {
						Type:        gollem.TypeObject,
						Description: "Optional nested object",
						Properties: map[string]*gollem.Parameter{
							"deep_field": {
								Type:        gollem.TypeString,
								Description: "A deep nested field",
							},
						},
						// No Required field set - this should default to empty slice
					},
					"array_field": {
						Type:        gollem.TypeArray,
						Description: "Array of objects",
						Items: &gollem.Parameter{
							Type: gollem.TypeObject,
							Properties: map[string]*gollem.Parameter{
								"item_id": {
									Type:        gollem.TypeString,
									Description: "Item identifier",
								},
								"item_value": {
									Type:        gollem.TypeNumber,
									Description: "Item value",
								},
							},
							// Array items object also has no Required field
						},
					},
				},
			},
		},
	}
}

func (t *complexSchemaTestTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return nil, nil
}

type constraintsTestTool struct{}

func (t *constraintsTestTool) Spec() gollem.ToolSpec {
	minLength := 1
	maxLength := 100
	minItems := 1
	maxItems := 10
	minimum := 0.0
	maximum := 100.0

	return gollem.ToolSpec{
		Name:        "constraints_test",
		Description: "Tool to test various parameter constraints",
		Parameters: map[string]*gollem.Parameter{
			"constrained_string": {
				Type:        gollem.TypeString,
				Description: "String with length constraints",
				MinLength:   &minLength,
				MaxLength:   &maxLength,
				Pattern:     "^[a-zA-Z0-9]+$",
			},
			"constrained_number": {
				Type:        gollem.TypeNumber,
				Description: "Number with min/max constraints",
				Minimum:     &minimum,
				Maximum:     &maximum,
			},
			"constrained_array": {
				Type:        gollem.TypeArray,
				Description: "Array with item constraints",
				MinItems:    &minItems,
				MaxItems:    &maxItems,
				Items: &gollem.Parameter{
					Type:        gollem.TypeString,
					Description: "Array item",
				},
			},
			"enum_field": {
				Type:        gollem.TypeString,
				Description: "String with enum values",
				Enum:        []string{"option1", "option2", "option3"},
			},
		},
	}
}

func (t *constraintsTestTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return nil, nil
}

type emptyParametersTool struct{}

func (t *emptyParametersTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "empty_params",
		Description: "Tool with no parameters",
		Parameters:  map[string]*gollem.Parameter{},
		Required:    []string{},
	}
}

func (t *emptyParametersTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return nil, nil
}

// Tests moved from debug_test.go

type respondToUserTool struct{}

func (t *respondToUserTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "respond_to_user",
		Description: "Call this tool when you have gathered all necessary information, completed all required actions, and already provided the final answer to the user's original request. This signals that your work on the current request is finished.",
		Parameters: map[string]*gollem.Parameter{
			"summary": {
				Type:        gollem.TypeString,
				Description: "Brief summary of what was accomplished",
			},
		},
		// Note: Required is empty, making summary optional
	}
}

func (t *respondToUserTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return nil, nil
}

type parameterlessTool struct{}

func (t *parameterlessTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "no_params_tool",
		Description: "A tool with no parameters",
		Parameters:  map[string]*gollem.Parameter{}, // Empty parameters
		Required:    []string{},                     // Empty required
	}
}

func (t *parameterlessTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return nil, nil
}

type nestedObjectTool struct{}

func (t *nestedObjectTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "nested_object_tool",
		Description: "A tool with nested object parameters",
		Parameters: map[string]*gollem.Parameter{
			"user": {
				Type: gollem.TypeObject,
				Properties: map[string]*gollem.Parameter{
					"name": {
						Type:        gollem.TypeString,
						Description: "User's name",
					},
					"email": {
						Type:        gollem.TypeString,
						Description: "User's email",
					},
				},
				// Note: Required is not set, should default to empty slice
			},
		},
	}
}

func (t *nestedObjectTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return nil, nil
}

func TestRespondToUserTool(t *testing.T) {
	tool := &respondToUserTool{}
	converted := gemini.ConvertTool(tool)

	t.Logf("Converted tool: %+v", converted)
	t.Logf("Parameters: %+v", converted.Parameters)
	t.Logf("Properties: %+v", converted.Parameters.Properties)
	t.Logf("Required: %+v", converted.Parameters.Required)

	// Verify the structure
	gt.Value(t, converted.Name).Equal("respond_to_user")
	gt.Value(t, len(converted.Parameters.Properties)).Equal(1)

	// Critical finding: Required is nil, not empty slice!
	if converted.Parameters.Required == nil {
		t.Logf("CRITICAL: Required field is nil, not empty slice!")
	} else {
		t.Logf("Required field is empty slice: %v", converted.Parameters.Required)
	}

	summary := converted.Parameters.Properties["summary"]
	gt.Value(t, summary).NotEqual(nil)
	t.Logf("Summary type: %v (String representation: %s)", summary.Type, summary.Type.String())
}

func TestParameterlessTool(t *testing.T) {
	tool := &parameterlessTool{}
	converted := gemini.ConvertTool(tool)

	t.Logf("Parameterless tool: %+v", converted)
	t.Logf("Parameters: %+v", converted.Parameters)

	gt.Value(t, converted.Name).Equal("no_params_tool")
	gt.Value(t, len(converted.Parameters.Properties)).Equal(0)
	gt.Value(t, converted.Parameters.Required).Equal([]string{})
}

func TestNestedObjectRequiredField(t *testing.T) {
	tool := &nestedObjectTool{}
	converted := gemini.ConvertTool(tool)

	t.Logf("Nested object tool: %+v", converted)
	t.Logf("Parameters: %+v", converted.Parameters)

	userParam := converted.Parameters.Properties["user"]
	gt.Value(t, userParam).NotEqual(nil)
	t.Logf("User parameter Required: %+v", userParam.Required)

	// This should be an empty slice, not nil
	if userParam.Required == nil {
		t.Errorf("CRITICAL: Nested object Required field is nil, should be empty slice!")
	} else {
		t.Logf("SUCCESS: Nested object Required field is empty slice: %v", userParam.Required)
	}

	gt.Value(t, userParam.Required).Equal([]string{})
}
