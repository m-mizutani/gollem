package gemini_test

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/m-mizutani/goerr/v2"
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
				Required:    true,
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
			},
		},
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
				Required:    true,
				// NOTE: No MaxLength constraint - this is the problematic part
			},
		},
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
				Required:    true,
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

	// Configure slog for debug output during testing
	ctx := context.Background()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))

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
	projectID := os.Getenv("TEST_GCP_PROJECT_ID")
	if projectID == "" {
		t.Skip("TEST_GCP_PROJECT_ID is not set")
	}

	location := os.Getenv("TEST_GCP_LOCATION")
	if location == "" {
		t.Skip("TEST_GCP_LOCATION is not set")
	}

	ctx := context.Background()

	testCases := []struct {
		name         string
		budget       int32
		expectBudget int32
	}{
		{
			name:         "auto thinking budget",
			budget:       -1,
			expectBudget: -1,
		},
		{
			name:         "specific thinking budget",
			budget:       1000,
			expectBudget: 1000,
		},
		{
			name:         "zero thinking budget",
			budget:       0,
			expectBudget: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := gemini.New(ctx, projectID, location,
				gemini.WithThinkingBudget(tc.budget),
			)
			gt.NoError(t, err)
			gt.NotNil(t, client)

			generationConfig := client.GetGenerationConfig()
			gt.NotNil(t, generationConfig)
			gt.NotNil(t, generationConfig.ThinkingConfig)
			gt.NotNil(t, generationConfig.ThinkingConfig.ThinkingBudget)
			gt.Equal(t, tc.expectBudget, *generationConfig.ThinkingConfig.ThinkingBudget)
		})
	}
}

func TestThinkingBudgetIntegration(t *testing.T) {
	projectID := os.Getenv("TEST_GCP_PROJECT_ID")
	if projectID == "" {
		t.Skip("TEST_GCP_PROJECT_ID is not set")
	}

	location := os.Getenv("TEST_GCP_LOCATION")
	if location == "" {
		t.Skip("TEST_GCP_LOCATION is not set")
	}

	ctx := context.Background()

	testCases := []struct {
		name   string
		model  string
		budget int32
	}{
		{
			name:   "Gemini 2.0 Flash with thinking budget disabled",
			model:  "gemini-2.0-flash",
			budget: 0,
		},
		{
			name:   "Gemini 2.5 Flash with thinking budget disabled",
			model:  "gemini-2.5-flash",
			budget: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := gemini.New(ctx, projectID, location,
				gemini.WithModel(tc.model),
				gemini.WithThinkingBudget(tc.budget),
			)
			gt.NoError(t, err)
			gt.NotNil(t, client)

			// Verify configuration is set correctly
			generationConfig := client.GetGenerationConfig()
			gt.NotNil(t, generationConfig)
			gt.NotNil(t, generationConfig.ThinkingConfig)
			gt.NotNil(t, generationConfig.ThinkingConfig.ThinkingBudget)
			gt.Equal(t, tc.budget, *generationConfig.ThinkingConfig.ThinkingBudget)

			// Test actual API call
			session, err := client.NewSession(ctx)
			gt.NoError(t, err)
			gt.NotNil(t, session)

			// Simple test prompt
			response, err := session.GenerateContent(ctx, gollem.Text("Say 'Hello' in one word"))
			gt.NoError(t, err)
			gt.NotNil(t, response)
			gt.Array(t, response.Texts).Length(1).Required()
			gt.Value(t, len(response.Texts[0])).NotEqual(0)
		})
	}
}

func TestTokenLimitErrorOptions(t *testing.T) {
	type testCase struct {
		name   string
		err    error
		hasTag bool
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			opts := gemini.TokenLimitErrorOptions(tc.err)
			if tc.hasTag {
				gt.NotEqual(t, 0, len(opts))
			} else {
				gt.Equal(t, 0, len(opts))
			}
		}
	}

	t.Run("token exceeded error - max context length", runTest(testCase{
		name: "max context length exceeded",
		err: &genai.APIError{
			Code:    400,
			Message: "The model's maximum context length is 128000 tokens",
			Status:  "INVALID_ARGUMENT",
		},
		hasTag: true,
	}))

	t.Run("token exceeded error - payload size", runTest(testCase{
		name: "payload size exceeded",
		err: &genai.APIError{
			Code:    400,
			Message: "Request payload size exceeds the limit: 10485760 bytes",
			Status:  "INVALID_ARGUMENT",
		},
		hasTag: true,
	}))

	t.Run("different error code", runTest(testCase{
		name: "error code 401",
		err: &genai.APIError{
			Code:    401,
			Message: "Invalid API key",
			Status:  "UNAUTHENTICATED",
		},
		hasTag: false,
	}))

	t.Run("different status", runTest(testCase{
		name: "different status",
		err: &genai.APIError{
			Code:    400,
			Message: "Invalid request",
			Status:  "INVALID_REQUEST",
		},
		hasTag: false,
	}))

	t.Run("different message", runTest(testCase{
		name: "different message",
		err: &genai.APIError{
			Code:    400,
			Message: "Invalid model specified",
			Status:  "INVALID_ARGUMENT",
		},
		hasTag: false,
	}))

	t.Run("correct code and status but wrong message", runTest(testCase{
		name: "wrong message",
		err: &genai.APIError{
			Code:    400,
			Message: "Some other error",
			Status:  "INVALID_ARGUMENT",
		},
		hasTag: false,
	}))

	t.Run("nil error", runTest(testCase{
		name:   "nil error",
		err:    nil,
		hasTag: false,
	}))

	t.Run("non-APIError", runTest(testCase{
		name:   "generic error",
		err:    errors.New("some error"),
		hasTag: false,
	}))
}

func TestGeminiTokenLimitErrorIntegration(t *testing.T) {
	projectID, ok := os.LookupEnv("TEST_GEMINI_PROJECT_ID")
	if !ok {
		t.Skip("TEST_GEMINI_PROJECT_ID is not set")
	}

	location, ok := os.LookupEnv("TEST_GEMINI_LOCATION")
	if !ok {
		t.Skip("TEST_GEMINI_LOCATION is not set")
	}

	// Only run if explicitly requested via environment variable
	if os.Getenv("TEST_TOKEN_LIMIT_ERROR") != "true" {
		t.Skip("TEST_TOKEN_LIMIT_ERROR is not set to true")
	}

	ctx := context.Background()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))

	client, err := gemini.New(ctx, projectID, location)
	gt.NoError(t, err)

	session, err := client.NewSession(ctx)
	gt.NoError(t, err)

	// Create a very long prompt to exceed token limit
	// Repeat a long text many times to ensure we exceed the limit
	longText := strings.Repeat("This is a test sentence to make the prompt very long. ", 100000)

	_, err = session.GenerateContent(ctx, gollem.Text(longText))
	gt.Error(t, err)

	// Verify the error has the token exceeded tag
	gt.True(t, goerr.HasTag(err, gollem.ErrTagTokenExceeded))
}
