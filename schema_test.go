package gollem_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/claude"
	"github.com/m-mizutani/gollem/llm/gemini"
	"github.com/m-mizutani/gollem/llm/openai"
	"github.com/m-mizutani/gt"
)

// TestResponseSchema_Validate tests the validation of ResponseSchema
func TestResponseSchema_Validate(t *testing.T) {
	runTest := func(tc struct {
		name      string
		schema    *gollem.ResponseSchema
		expectErr bool
	}) func(*testing.T) {
		return func(t *testing.T) {
			err := tc.schema.Validate()
			if tc.expectErr {
				gt.Error(t, err)
			} else {
				gt.NoError(t, err)
			}
		}
	}

	t.Run("valid schema", runTest(struct {
		name      string
		schema    *gollem.ResponseSchema
		expectErr bool
	}{
		name: "valid schema",
		schema: &gollem.ResponseSchema{
			Name:        "TestSchema",
			Description: "Test schema",
			Schema: &gollem.Parameter{
				Type: gollem.TypeObject,
				Properties: map[string]*gollem.Parameter{
					"name": {Type: gollem.TypeString},
				},
			},
		},
		expectErr: false,
	}))

	t.Run("nil schema", runTest(struct {
		name      string
		schema    *gollem.ResponseSchema
		expectErr bool
	}{
		name:      "nil schema",
		schema:    nil,
		expectErr: false,
	}))

	t.Run("missing schema field", runTest(struct {
		name      string
		schema    *gollem.ResponseSchema
		expectErr bool
	}{
		name: "missing schema field",
		schema: &gollem.ResponseSchema{
			Name:        "TestSchema",
			Description: "Test schema",
			Schema:      nil,
		},
		expectErr: true,
	}))

	t.Run("non-object root type", runTest(struct {
		name      string
		schema    *gollem.ResponseSchema
		expectErr bool
	}{
		name: "non-object root type",
		schema: &gollem.ResponseSchema{
			Name:        "TestSchema",
			Description: "Test schema",
			Schema: &gollem.Parameter{
				Type: gollem.TypeString,
			},
		},
		expectErr: true,
	}))

	t.Run("invalid nested parameter", runTest(struct {
		name      string
		schema    *gollem.ResponseSchema
		expectErr bool
	}{
		name: "invalid nested parameter",
		schema: &gollem.ResponseSchema{
			Name:        "TestSchema",
			Description: "Test schema",
			Schema: &gollem.Parameter{
				Type: gollem.TypeObject,
				Properties: map[string]*gollem.Parameter{
					"items": {
						Type:  gollem.TypeArray,
						Items: nil, // Invalid: array must have items
					},
				},
			},
		},
		expectErr: true,
	}))
}

// validateJSONAgainstSchema validates that the JSON response matches the expected schema
func validateJSONAgainstSchema(t *testing.T, jsonStr string, schema *gollem.ResponseSchema) {
	t.Helper()

	// Parse JSON
	var data map[string]any
	err := json.Unmarshal([]byte(jsonStr), &data)
	if err != nil {
		t.Fatalf("response should be valid JSON: %v", err)
	}

	// Validate the root object
	validateParameter(t, "", data, schema.Schema)
}

// validateParameter recursively validates a value against a parameter schema
func validateParameter(t *testing.T, path string, value any, param *gollem.Parameter) {
	t.Helper()

	if path == "" {
		path = "root"
	}

	// Type validation
	switch param.Type {
	case gollem.TypeString:
		strVal, ok := value.(string)
		if !ok {
			t.Errorf("%s should be string, got %T", path, value)
			return
		}

		// Pattern validation
		if param.Pattern != "" {
			matched, err := regexp.MatchString(param.Pattern, strVal)
			if err != nil {
				t.Errorf("%s pattern validation failed: %v", path, err)
			} else if !matched {
				t.Errorf("%s value %q does not match pattern %q", path, strVal, param.Pattern)
			}
		}

		// Length constraints
		if param.MinLength != nil && len(strVal) < *param.MinLength {
			t.Errorf("%s length %d is less than minLength %d", path, len(strVal), *param.MinLength)
		}
		if param.MaxLength != nil && len(strVal) > *param.MaxLength {
			t.Errorf("%s length %d exceeds maxLength %d", path, len(strVal), *param.MaxLength)
		}

		// Enum validation
		if param.Enum != nil {
			found := false
			for _, enumVal := range param.Enum {
				if strVal == enumVal {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s value %q is not in enum %v", path, strVal, param.Enum)
			}
		}

	case gollem.TypeInteger:
		numVal, ok := value.(float64)
		if !ok {
			t.Errorf("%s should be number, got %T", path, value)
			return
		}

		// Check if it's actually an integer
		if numVal != float64(int(numVal)) {
			t.Errorf("%s should be integer, got float %f", path, numVal)
		}

		// Range constraints
		if param.Minimum != nil && numVal < *param.Minimum {
			t.Errorf("%s value %f is less than minimum %f", path, numVal, *param.Minimum)
		}
		if param.Maximum != nil && numVal > *param.Maximum {
			t.Errorf("%s value %f exceeds maximum %f", path, numVal, *param.Maximum)
		}

	case gollem.TypeNumber:
		numVal, ok := value.(float64)
		if !ok {
			t.Errorf("%s should be number, got %T", path, value)
			return
		}

		// Range constraints
		if param.Minimum != nil && numVal < *param.Minimum {
			t.Errorf("%s value %f is less than minimum %f", path, numVal, *param.Minimum)
		}
		if param.Maximum != nil && numVal > *param.Maximum {
			t.Errorf("%s value %f exceeds maximum %f", path, numVal, *param.Maximum)
		}

	case gollem.TypeBoolean:
		if _, ok := value.(bool); !ok {
			t.Errorf("%s should be boolean, got %T", path, value)
		}

	case gollem.TypeArray:
		arrVal, ok := value.([]any)
		if !ok {
			t.Errorf("%s should be array, got %T", path, value)
			return
		}

		// Array length constraints
		if param.MinItems != nil && len(arrVal) < *param.MinItems {
			t.Errorf("%s array length %d is less than minItems %d", path, len(arrVal), *param.MinItems)
		}
		if param.MaxItems != nil && len(arrVal) > *param.MaxItems {
			t.Errorf("%s array length %d exceeds maxItems %d", path, len(arrVal), *param.MaxItems)
		}

		// Validate each item
		if param.Items != nil {
			for i, item := range arrVal {
				itemPath := fmt.Sprintf("%s[%d]", path, i)
				validateParameter(t, itemPath, item, param.Items)
			}
		}

	case gollem.TypeObject:
		objVal, ok := value.(map[string]any)
		if !ok {
			t.Errorf("%s should be object, got %T", path, value)
			return
		}

		// Check required fields
		for _, required := range param.Required {
			if _, exists := objVal[required]; !exists {
				t.Errorf("%s missing required field %q", path, required)
			}
		}

		// Validate each property
		if param.Properties != nil {
			for propName, propSchema := range param.Properties {
				propValue, exists := objVal[propName]
				if !exists {
					// Skip optional fields
					continue
				}
				propPath := fmt.Sprintf("%s.%s", path, propName)
				validateParameter(t, propPath, propValue, propSchema)
			}
		}
	}
}

// TestResponseSchemaIntegration tests JSON schema functionality with real LLM clients
func TestResponseSchemaIntegration(t *testing.T) {
	t.Parallel()

	testFn := func(t *testing.T, newClient func(t *testing.T) (gollem.LLMClient, error)) {
		client, err := newClient(t)
		gt.NoError(t, err)

		// Create complex test schema
		schema := createComplexBookSchema()

		// Create session with JSON content type and response schema
		session, err := client.NewSession(context.Background(),
			gollem.WithSessionContentType(gollem.ContentTypeJSON),
			gollem.WithSessionResponseSchema(schema),
		)
		gt.NoError(t, err)

		// Generate content with a complex prompt
		prompt := `Create a book review for "1984" by George Orwell.
		Published in 1949, ISBN 978-0451524935.
		Give it a 4.5 rating.
		Summary: A dystopian masterpiece exploring surveillance and totalitarianism.
		Pros: Thought-provoking, relevant today, excellent world-building.
		Cons: Dark and depressing at times.
		Tags: fiction, sci-fi.
		Highly recommended.`

		resp, err := session.GenerateContent(t.Context(), gollem.Text(prompt))
		gt.NoError(t, err)
		gt.NotNil(t, resp)
		gt.True(t, len(resp.Texts) > 0)

		// Get the JSON response
		jsonResponse := strings.Join(resp.Texts, "")
		t.Logf("JSON Response: %s", jsonResponse)

		// Validate the response matches the schema
		validateJSONAgainstSchema(t, jsonResponse, schema)

		// Parse and validate specific fields
		var bookReview map[string]any
		err = json.Unmarshal([]byte(jsonResponse), &bookReview)
		gt.NoError(t, err)

		// Validate book object exists and has required fields
		if bookObj, ok := bookReview["book"].(map[string]any); ok {
			if title, ok := bookObj["title"].(string); ok {
				if !strings.Contains(strings.ToLower(title), "1984") {
					t.Logf("Warning: title should contain '1984', got: %s", title)
				}
			} else {
				t.Error("book.title should be a string")
			}

			if author, ok := bookObj["author"].(string); ok {
				if !strings.Contains(strings.ToLower(author), "orwell") {
					t.Logf("Warning: author should contain 'Orwell', got: %s", author)
				}
			} else {
				t.Error("book.author should be a string")
			}
		} else {
			t.Error("book object should exist")
		}

		// Validate review object
		if reviewObj, ok := bookReview["review"].(map[string]any); ok {
			if rating, ok := reviewObj["rating"].(float64); ok {
				if rating < 1.0 || rating > 5.0 {
					t.Errorf("rating should be between 1.0 and 5.0, got: %f", rating)
				}
			} else {
				t.Error("review.rating should be a number")
			}

			if summary, ok := reviewObj["summary"].(string); ok {
				if len(summary) < 10 {
					t.Errorf("summary should be at least 10 characters, got: %d", len(summary))
				}
			} else {
				t.Error("review.summary should be a string")
			}

			// Validate pros array
			if pros, ok := reviewObj["pros"].([]any); ok {
				if len(pros) < 1 {
					t.Error("pros should have at least 1 item")
				}
			}
		} else {
			t.Error("review object should exist")
		}

		// Validate tags array
		if tags, ok := bookReview["tags"].([]any); ok {
			if len(tags) < 1 || len(tags) > 5 {
				t.Errorf("tags should have 1-5 items, got: %d", len(tags))
			}
		} else {
			t.Error("tags should be an array")
		}

		// Validate recommended boolean
		if _, ok := bookReview["recommended"].(bool); !ok {
			t.Error("recommended should be a boolean")
		}
	}

	t.Run("OpenAI", func(t *testing.T) {
		t.Parallel()
		apiKey, ok := os.LookupEnv("TEST_OPENAI_API_KEY")
		if !ok {
			t.Skip("TEST_OPENAI_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return openai.New(context.Background(), apiKey)
		})
	})

	t.Run("Claude", func(t *testing.T) {
		apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
		if !ok {
			t.Skip("TEST_CLAUDE_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return claude.New(context.Background(), apiKey)
		})
	})

	t.Run("Gemini", func(t *testing.T) {
		t.Parallel()
		projectID, ok := os.LookupEnv("TEST_GCP_PROJECT_ID")
		if !ok {
			t.Skip("TEST_GCP_PROJECT_ID is not set")
		}
		location, ok := os.LookupEnv("TEST_GCP_LOCATION")
		if !ok {
			t.Skip("TEST_GCP_LOCATION is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return gemini.New(context.Background(), projectID, location)
		})
	})
}

// createComplexBookSchema creates a more complex schema for testing nested objects and arrays
func createComplexBookSchema() *gollem.ResponseSchema {
	return &gollem.ResponseSchema{
		Name:        "BookReview",
		Description: "A detailed book review with metadata",
		Schema: &gollem.Parameter{
			Type: gollem.TypeObject,
			Properties: map[string]*gollem.Parameter{
				"book": {
					Type: gollem.TypeObject,
					Properties: map[string]*gollem.Parameter{
						"title": {
							Type:        gollem.TypeString,
							Description: "Book title",
						},
						"author": {
							Type:        gollem.TypeString,
							Description: "Author name",
						},
						"publishedYear": {
							Type:        gollem.TypeInteger,
							Description: "Year of publication",
							Minimum:     Float64Ptr(1000),
							Maximum:     Float64Ptr(2100),
						},
						"isbn": {
							Type:        gollem.TypeString,
							Description: "ISBN number",
							Pattern:     "^[0-9-]+$",
						},
					},
					Required:    []string{"title", "author"},
					Description: "Book information",
				},
				"review": {
					Type: gollem.TypeObject,
					Properties: map[string]*gollem.Parameter{
						"rating": {
							Type:        gollem.TypeNumber,
							Description: "Rating from 1.0 to 5.0",
							Minimum:     Float64Ptr(1.0),
							Maximum:     Float64Ptr(5.0),
						},
						"summary": {
							Type:        gollem.TypeString,
							Description: "Brief review summary",
							MinLength:   IntPtr(10),
							MaxLength:   IntPtr(500),
						},
						"pros": {
							Type: gollem.TypeArray,
							Items: &gollem.Parameter{
								Type: gollem.TypeString,
							},
							Description: "Positive aspects",
							MinItems:    IntPtr(1),
						},
						"cons": {
							Type: gollem.TypeArray,
							Items: &gollem.Parameter{
								Type: gollem.TypeString,
							},
							Description: "Negative aspects",
						},
					},
					Required:    []string{"rating", "summary"},
					Description: "Review content",
				},
				"tags": {
					Type: gollem.TypeArray,
					Items: &gollem.Parameter{
						Type: gollem.TypeString,
						Enum: []string{"fiction", "non-fiction", "mystery", "romance", "sci-fi", "fantasy", "biography", "history", "technical"},
					},
					Description: "Book genre tags",
					MinItems:    IntPtr(1),
					MaxItems:    IntPtr(5),
				},
				"recommended": {
					Type:        gollem.TypeBoolean,
					Description: "Whether the book is recommended",
				},
			},
			Required: []string{"book", "review", "recommended"},
		},
	}
}

// Float64Ptr returns a pointer to a float64 value
func Float64Ptr(v float64) *float64 {
	return &v
}

// IntPtr returns a pointer to an int value
func IntPtr(v int) *int {
	return &v
}
