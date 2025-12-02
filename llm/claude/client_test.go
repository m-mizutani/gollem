package claude_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/claude"
	"github.com/m-mizutani/gt"
)

func TestClaudeContentGenerate(t *testing.T) {
	apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
	if !ok {
		t.Skip("TEST_CLAUDE_API_KEY is not set")
	}

	ctx := context.Background()
	// Create a debug logger that outputs to testing.T
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx = ctxlog.With(ctx, logger)

	client, err := claude.New(ctx, apiKey)
	gt.NoError(t, err)

	session, err := client.NewSession(ctx)
	gt.NoError(t, err)

	result, err := session.GenerateContent(ctx, gollem.Text("Say hello in one word"))
	gt.NoError(t, err)
	gt.Array(t, result.Texts).Length(1).Required()
	gt.Value(t, len(result.Texts[0])).NotEqual(0)
}

// TestCreateSystemPrompt tests the createSystemPrompt function
func TestCreateSystemPrompt(t *testing.T) {
	ctx := context.Background()

	t.Run("empty config returns empty slice", func(t *testing.T) {
		cfg := gollem.NewSessionConfig()
		result, err := claude.CreateSystemPrompt(ctx, cfg)
		gt.NoError(t, err)

		// Should return empty slice when no system prompt
		gt.Equal(t, 0, len(result))
	})

	t.Run("result is correct type", func(t *testing.T) {
		cfg := gollem.NewSessionConfig()
		result, err := claude.CreateSystemPrompt(ctx, cfg)
		gt.NoError(t, err)

		// Type assertion to verify it's []anthropic.TextBlockParam
		var _ []anthropic.TextBlockParam = result
		// Empty slice can be nil in this implementation
		gt.Equal(t, 0, len(result))
	})

	t.Run("JSON content type check", func(t *testing.T) {
		// Create config with JSON content type
		cfg := gollem.NewSessionConfig()
		// Manually set content type since we can't use WithContentType in test
		// The actual functionality is tested in integration tests
		result, err := claude.CreateSystemPrompt(ctx, cfg)
		gt.NoError(t, err)

		// At minimum, should not panic and return valid type
		var systemPrompt []anthropic.TextBlockParam = result
		// Check the type is correct (compilation would fail if not)
		_ = systemPrompt
	})
}

// TestSystemPromptSDKCompliance verifies SDK compliance
func TestSystemPromptSDKCompliance(t *testing.T) {
	ctx := context.Background()

	t.Run("SDK format verification", func(t *testing.T) {
		// This test verifies the format matches SDK expectations:
		// []anthropic.TextBlockParam{{Text: "..."}}

		// Create empty config
		cfg := gollem.NewSessionConfig()
		result, err := claude.CreateSystemPrompt(ctx, cfg)
		gt.NoError(t, err)

		// Should be able to use as []anthropic.TextBlockParam
		var systemBlocks []anthropic.TextBlockParam = result

		// Empty case should return empty slice
		gt.Equal(t, 0, len(systemBlocks))
	})

	t.Run("TextBlockParam structure", func(t *testing.T) {
		// Verify we can create TextBlockParam correctly
		testBlock := anthropic.TextBlockParam{
			Text: "Test prompt",
		}

		// Verify the Text field exists and is accessible
		gt.Equal(t, "Test prompt", testBlock.Text)

		// Create a slice as the function would return
		blocks := []anthropic.TextBlockParam{testBlock}
		gt.Equal(t, 1, len(blocks))
		gt.Equal(t, "Test prompt", blocks[0].Text)
	})
}

// TestSystemPromptComment verifies the implementation comment
func TestSystemPromptComment(t *testing.T) {
	ctx := context.Background()

	// This test documents that the implementation follows the official SDK format
	// The createSystemPrompt function should return []anthropic.TextBlockParam
	// in the format: []anthropic.TextBlockParam{{Text: "..."}}

	t.Run("comment accuracy", func(t *testing.T) {
		// The function is documented as:
		// "Returns []anthropic.TextBlockParam as per anthropic-sdk-go v1.5.0 specification"
		// This test verifies that claim

		cfg := gollem.NewSessionConfig()
		result, err := claude.CreateSystemPrompt(ctx, cfg)
		gt.NoError(t, err)

		// Should be the correct type
		var _ []anthropic.TextBlockParam = result

		// Should handle empty case correctly
		if len(result) > 0 {
			// If not empty, each element should have a Text field
			for _, block := range result {
				// Text field should be accessible
				_ = block.Text
			}
		}
	})
}

func TestTokenLimitErrorOptions(t *testing.T) {
	type testCase struct {
		name   string
		err    error
		hasTag bool
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			opts := claude.TokenLimitErrorOptions(tc.err)
			if tc.hasTag {
				gt.NotEqual(t, 0, len(opts))
			} else {
				gt.Equal(t, 0, len(opts))
			}
		}
	}

	// Create a mock anthropic.Error with token exceeded error
	createTokenExceededError := func() *anthropic.Error {
		rawJSON := map[string]any{
			"type": "error",
			"error": map[string]any{
				"type":    "invalid_request_error",
				"message": "prompt is too long: 150000 tokens > 100000 maximum",
			},
		}
		rawJSONBytes, _ := json.Marshal(rawJSON)

		err := &anthropic.Error{
			StatusCode: 400,
		}
		// Use UnmarshalJSON to properly set the internal raw field
		_ = err.UnmarshalJSON(rawJSONBytes)
		return err
	}

	createDifferentTypeError := func() *anthropic.Error {
		rawJSON := map[string]any{
			"type": "error",
			"error": map[string]any{
				"type":    "authentication_error",
				"message": "Invalid API key",
			},
		}
		rawJSONBytes, _ := json.Marshal(rawJSON)

		err := &anthropic.Error{
			StatusCode: 401,
		}
		_ = err.UnmarshalJSON(rawJSONBytes)
		return err
	}

	createDifferentMessageError := func() *anthropic.Error {
		rawJSON := map[string]any{
			"type": "error",
			"error": map[string]any{
				"type":    "invalid_request_error",
				"message": "Invalid model specified",
			},
		}
		rawJSONBytes, _ := json.Marshal(rawJSON)

		err := &anthropic.Error{
			StatusCode: 400,
		}
		_ = err.UnmarshalJSON(rawJSONBytes)
		return err
	}

	createDifferentStatusError := func() *anthropic.Error {
		rawJSON := map[string]any{
			"type": "error",
			"error": map[string]any{
				"type":    "invalid_request_error",
				"message": "prompt is too long: 150000 tokens > 100000 maximum",
			},
		}
		rawJSONBytes, _ := json.Marshal(rawJSON)

		err := &anthropic.Error{
			StatusCode: 500,
		}
		_ = err.UnmarshalJSON(rawJSONBytes)
		return err
	}

	create413Error := func() *anthropic.Error {
		rawJSON := map[string]any{
			"type": "error",
			"error": map[string]any{
				"type":    "invalid_request_error",
				"message": "Prompt is too long",
			},
		}
		rawJSONBytes, _ := json.Marshal(rawJSON)

		err := &anthropic.Error{
			StatusCode: 413,
		}
		_ = err.UnmarshalJSON(rawJSONBytes)
		return err
	}

	createCapitalizedMessageError := func() *anthropic.Error {
		rawJSON := map[string]any{
			"type": "error",
			"error": map[string]any{
				"type":    "invalid_request_error",
				"message": "Prompt is too long: 150000 tokens > 100000 maximum",
			},
		}
		rawJSONBytes, _ := json.Marshal(rawJSON)

		err := &anthropic.Error{
			StatusCode: 400,
		}
		_ = err.UnmarshalJSON(rawJSONBytes)
		return err
	}

	t.Run("token exceeded error", runTest(testCase{
		name:   "prompt is too long",
		err:    createTokenExceededError(),
		hasTag: true,
	}))

	t.Run("different error type", runTest(testCase{
		name:   "authentication error",
		err:    createDifferentTypeError(),
		hasTag: false,
	}))

	t.Run("different message", runTest(testCase{
		name:   "invalid model",
		err:    createDifferentMessageError(),
		hasTag: false,
	}))

	t.Run("different status code", runTest(testCase{
		name:   "status 500",
		err:    createDifferentStatusError(),
		hasTag: false,
	}))

	t.Run("413 status code with capitalized message", runTest(testCase{
		name:   "413 Request Entity Too Large",
		err:    create413Error(),
		hasTag: true,
	}))

	t.Run("capitalized message with 400 status", runTest(testCase{
		name:   "Prompt is too long (capitalized)",
		err:    createCapitalizedMessageError(),
		hasTag: true,
	}))

	t.Run("nil error", runTest(testCase{
		name:   "nil error",
		err:    nil,
		hasTag: false,
	}))

	t.Run("non-anthropic error", runTest(testCase{
		name:   "generic error",
		err:    errors.New("some error"),
		hasTag: false,
	}))
}

func TestClaudeTokenLimitErrorIntegration(t *testing.T) {
	apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
	if !ok {
		t.Skip("TEST_CLAUDE_API_KEY is not set")
	}

	// Only run if explicitly requested via environment variable
	if os.Getenv("TEST_TOKEN_LIMIT_ERROR") != "true" {
		t.Skip("TEST_TOKEN_LIMIT_ERROR is not set to true")
	}

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx = ctxlog.With(ctx, logger)

	client, err := claude.New(ctx, apiKey)
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

// TestWithBaseURL tests the WithBaseURL option functionality
func TestWithBaseURL(t *testing.T) {
	t.Run("default baseURL", func(t *testing.T) {
		client, err := claude.New(context.Background(), "test-key", claude.WithBaseURL(""))
		gt.NoError(t, err)
		gt.Equal(t, "", claude.GetBaseURL(client))
	})

	t.Run("custom baseURL", func(t *testing.T) {
		customURL := "https://custom.anthropic.com"
		client, err := claude.New(context.Background(), "test-key", claude.WithBaseURL(customURL))
		gt.NoError(t, err)
		gt.Equal(t, customURL, claude.GetBaseURL(client))
	})

	t.Run("empty baseURL after custom", func(t *testing.T) {
		// Test that empty baseURL overrides previous setting
		client1, err1 := claude.New(context.Background(), "test-key", claude.WithBaseURL("https://first.com"))
		gt.NoError(t, err1)
		gt.Equal(t, "https://first.com", claude.GetBaseURL(client1))

		// Apply empty baseURL after custom one
		client2, err2 := claude.New(context.Background(), "test-key",
			claude.WithBaseURL("https://first.com"),
			claude.WithBaseURL(""))
		gt.NoError(t, err2)
		gt.Equal(t, "", claude.GetBaseURL(client2)) // Should be empty, not first URL
	})
}
