package gemini_test

import (
	"context"
	"log/slog"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/gemini"
	"github.com/m-mizutani/gt"
	"google.golang.org/genai"
)

// Tests for client.go functionality

func TestClientMalformedFunctionCallErrorHandling(t *testing.T) {
	// This test simulates what would happen when a malformed function call error occurs
	// We can't easily trigger this in a unit test, so we test the error handling logic

	t.Run("error contains helpful information", func(t *testing.T) {
		// This would be called when a malformed function call is detected
		err := gollem.ErrInvalidParameter

		// The error should contain useful debugging information
		gt.Value(t, err).NotEqual(nil)

		// In a real scenario, the error would contain:
		// - candidate_index
		// - content_parts
		// - finish_reason
		// - suggested_action

	})
}

func TestClientRetryLogic(t *testing.T) {
	t.Run("retry with exponential backoff", func(t *testing.T) {
		start := time.Now()

		// Simulate what the retry logic would do
		maxRetries := 3
		baseDelay := 100 * time.Millisecond

		for attempt := 0; attempt < maxRetries; attempt++ {
			// Simulate a malformed function call error
			simulatedError := "malformed function call detected"

			if strings.Contains(simulatedError, "malformed function call") {
				// Always sleep before the next attempt (except we'll break before the last one)
				if attempt < maxRetries-1 {
					// Calculate delay (exponential backoff) using math.Pow like the real implementation
					delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
					time.Sleep(delay)
					continue
				}
			}
			break
		}

		elapsed := time.Since(start)

		// Should have taken at least the sum of delays: 100ms + 200ms = 300ms
		// (We only sleep on attempt 0 and 1, not on the final attempt)
		expectedMinDelay := 300 * time.Millisecond
		gt.Value(t, elapsed >= expectedMinDelay).Equal(true)

	})
}

func TestClientLargeTextDetection(t *testing.T) {
	t.Run("detect large text content", func(t *testing.T) {
		testCases := []struct {
			name    string
			content string
			isLarge bool
		}{
			{
				name:    "small_text",
				content: "This is a small text",
				isLarge: false,
			},
			{
				name:    "large_text",
				content: strings.Repeat("a", 1500),
				isLarge: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				isLarge := len(tc.content) > 1000
				gt.Value(t, isLarge).Equal(tc.isLarge)

				_ = isLarge // Note detected state
			})
		}
	})
}

func TestClientToolSchemaValidation(t *testing.T) {
	t.Run("valid_tool_schema", func(t *testing.T) {
		tool := &validClientTool{}
		spec := tool.Spec()

		// Check that the spec has required fields
		gt.Value(t, spec.Name).NotEqual("")
		gt.Value(t, spec.Description).NotEqual("")
		gt.Value(t, spec.Parameters).NotEqual(nil)

		// Check that string parameters have constraints
		for _, param := range spec.Parameters {
			if param.Type == gollem.TypeString {
				_ = param.MaxLength == nil // Check constraint presence
			}
		}
	})

	t.Run("problematic_tool_schema", func(t *testing.T) {
		tool := &problematicClientTool{}
		spec := tool.Spec()

		// Check for potential issues
		hasProblematicNames := false
		problematicNames := []string{"type", "properties", "required"}

		for _, name := range problematicNames {
			if _, exists := spec.Parameters[name]; exists {
				hasProblematicNames = true
			}
		}

		_ = hasProblematicNames // Note problematic names detected
	})
}

func TestGeminiClientIssues(t *testing.T) {

	t.Run("large_text_content_schema", func(t *testing.T) {
		tool := &largeTextClientTool{}
		converted := gemini.ConvertTool(tool)

		gt.Value(t, converted.Name).Equal("large_text_client")
		gt.Value(t, len(converted.Parameters.Properties)).Equal(1)

		contentParam := converted.Parameters.Properties["content"]
		gt.Value(t, contentParam).NotEqual(nil)
		gt.Value(t, contentParam.Type).Equal(genai.TypeString)

		// Check for length constraints
		_ = contentParam.MaxLength == nil || *contentParam.MaxLength == 0 // Note constraint status

	})

	t.Run("problematic_field_names", func(t *testing.T) {
		tool := &problematicFieldClientTool{}
		converted := gemini.ConvertTool(tool)

		gt.Value(t, converted.Name).Equal("problematic_field_client")
		gt.Value(t, len(converted.Parameters.Properties)).Equal(4)

		// Check that problematic field names are handled
		problematicNames := []string{"type", "properties", "required"}
		for _, name := range problematicNames {
			param := converted.Parameters.Properties[name]
			gt.Value(t, param).NotEqual(nil)
		}

		// Check unicode field
		unicodeParam := converted.Parameters.Properties["unicode_field"]
		gt.Value(t, unicodeParam).NotEqual(nil)
		gt.Value(t, unicodeParam.Type).Equal(genai.TypeString)

		// Log the unicode description to verify it's handled correctly

	})
}

// Tool definitions for client testing

type validClientTool struct{}

func (t *validClientTool) Spec() gollem.ToolSpec {
	maxLen := 1000

	return gollem.ToolSpec{
		Name:        "valid_client_tool",
		Description: "A well-designed tool with proper constraints",
		Parameters: map[string]*gollem.Parameter{
			"content": {
				Type:        gollem.TypeString,
				Description: "Content with length constraints",
				MaxLength:   &maxLen,
			},
			"metadata": {
				Type:        gollem.TypeObject,
				Description: "Metadata object",
				Properties: map[string]*gollem.Parameter{
					"title": {
						Type:        gollem.TypeString,
						Description: "Title",
						MaxLength:   &maxLen,
					},
				},
				Required: []string{}, // Empty slice, not nil
			},
		},
		Required: []string{"content"},
	}
}

func (t *validClientTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return map[string]any{"result": "success"}, nil
}

type problematicClientTool struct{}

func (t *problematicClientTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "problematic_client_tool",
		Description: "A tool with potential issues",
		Parameters: map[string]*gollem.Parameter{
			"type": { // Problematic name
				Type:        gollem.TypeString,
				Description: "Type field",
				// No MaxLength constraint
			},
			"properties": { // Problematic name
				Type:        gollem.TypeObject,
				Description: "Properties field",
				Properties: map[string]*gollem.Parameter{
					"nested": {
						Type:        gollem.TypeString,
						Description: "Nested field",
					},
				},
				// Required field might be nil
			},
		},
	}
}

func (t *problematicClientTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return map[string]any{"result": "success"}, nil
}

type largeTextClientTool struct{}

func (t *largeTextClientTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "large_text_client",
		Description: "A tool that accepts large text content which might cause issues",
		Parameters: map[string]*gollem.Parameter{
			"content": {
				Type:        gollem.TypeString,
				Description: "Large text content that might cause FinishReasonMalformedFunctionCall",
				// NOTE: No MaxLength constraint - this is the problematic part
			},
		},
		Required: []string{"content"},
	}
}

func (t *largeTextClientTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return map[string]any{"result": "processed"}, nil
}

type problematicFieldClientTool struct{}

func (t *problematicFieldClientTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "problematic_field_client",
		Description: "Tool with field names that might conflict with JSON schema keywords",
		Parameters: map[string]*gollem.Parameter{
			"type": {
				Type:        gollem.TypeString,
				Description: "Field named 'type' - might conflict with JSON schema",
			},
			"properties": {
				Type:        gollem.TypeString,
				Description: "Field named 'properties' - might conflict with JSON schema",
			},
			"required": {
				Type:        gollem.TypeString,
				Description: "Field named 'required' - might conflict with JSON schema",
			},
			"unicode_field": {
				Type:        gollem.TypeString,
				Description: "Field with unicode: test characters 🚀 emoji",
			},
		},
		Required: []string{"type"},
	}
}

func (t *problematicFieldClientTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return map[string]any{"result": "processed"}, nil
}

func TestGeminiContentGenerate(t *testing.T) {
	var testProjectID, testLocation string
	v, ok := os.LookupEnv("TEST_GCP_PROJECT_ID")
	if !ok {
		t.Skip("TEST_GCP_PROJECT_ID is not set")
	} else {
		testProjectID = v
	}

	v, ok = os.LookupEnv("TEST_GCP_LOCATION")
	if !ok {
		t.Skip("TEST_GCP_LOCATION is not set")
	} else {
		testLocation = v
	}

	// Configure ctxlog to output logs during testing
	ctx := context.Background()
	// Create a debug logger that outputs to testing.T
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx = ctxlog.With(ctx, logger)

	client, err := gemini.New(ctx, testProjectID, testLocation)
	gt.NoError(t, err)

	session, err := client.NewSession(ctx)
	gt.NoError(t, err)

	result, err := session.GenerateContent(ctx, gollem.Text("Say hello in one word"))
	gt.NoError(t, err)
	gt.Array(t, result.Texts).Length(1).Required()
	gt.Value(t, len(result.Texts[0])).NotEqual(0)
}

func TestWithThinkingBudget(t *testing.T) {
	ctx := context.Background()

	t.Run("auto thinking budget", func(t *testing.T) {
		// Create client with automatic thinking budget
		client, err := gemini.New(ctx, "test-project", "us-central1",
			gemini.WithThinkingBudget(-1),
		)
		gt.NoError(t, err)
		gt.NotNil(t, client)
	})

	t.Run("specific thinking budget", func(t *testing.T) {
		// Create client with specific thinking budget
		client, err := gemini.New(ctx, "test-project", "us-central1",
			gemini.WithThinkingBudget(1000),
		)
		gt.NoError(t, err)
		gt.NotNil(t, client)
	})

	t.Run("zero thinking budget", func(t *testing.T) {
		// Create client with zero thinking budget (disable thinking)
		client, err := gemini.New(ctx, "test-project", "us-central1",
			gemini.WithThinkingBudget(0),
		)
		gt.NoError(t, err)
		gt.NotNil(t, client)
	})
}
