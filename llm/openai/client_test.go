package openai_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/openai"
	"github.com/m-mizutani/gt"
	openaiapi "github.com/sashabaranov/go-openai"
)

func TestOpenAIContentGenerate(t *testing.T) {
	apiKey, ok := os.LookupEnv("TEST_OPENAI_API_KEY")
	if !ok {
		t.Skip("TEST_OPENAI_API_KEY is not set")
	}

	ctx := context.Background()
	// Create a debug logger that outputs to testing.T
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx = ctxlog.With(ctx, logger)

	client, err := openai.New(ctx, apiKey)
	gt.NoError(t, err)

	session, err := client.NewSession(ctx)
	gt.NoError(t, err)

	result, err := session.GenerateContent(ctx, gollem.Text("Say hello in one word"))
	gt.NoError(t, err)
	gt.Array(t, result.Texts).Length(1).Required()
	gt.Value(t, len(result.Texts[0])).NotEqual(0)
}

func TestTokenLimitErrorOptions(t *testing.T) {
	type testCase struct {
		name   string
		err    error
		hasTag bool
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			opts := openai.TokenLimitErrorOptions(tc.err)
			if tc.hasTag {
				gt.NotEqual(t, 0, len(opts))
			} else {
				gt.Equal(t, 0, len(opts))
			}
		}
	}

	t.Run("token exceeded error", runTest(testCase{
		name: "context_length_exceeded",
		err: &openaiapi.APIError{
			Type:    "invalid_request_error",
			Code:    "context_length_exceeded",
			Message: "This model's maximum context length is 128000 tokens. However, your messages resulted in 150000 tokens.",
		},
		hasTag: true,
	}))

	t.Run("different error type", runTest(testCase{
		name: "different type",
		err: &openaiapi.APIError{
			Type:    "authentication_error",
			Code:    "invalid_api_key",
			Message: "Invalid API key",
		},
		hasTag: false,
	}))

	t.Run("different error code", runTest(testCase{
		name: "different code",
		err: &openaiapi.APIError{
			Type:    "invalid_request_error",
			Code:    "invalid_model",
			Message: "The model does not exist",
		},
		hasTag: false,
	}))

	t.Run("code is not string", runTest(testCase{
		name: "code as int",
		err: &openaiapi.APIError{
			Type:    "invalid_request_error",
			Code:    12345,
			Message: "Some error",
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

func TestOpenAITokenLimitErrorIntegration(t *testing.T) {
	apiKey, ok := os.LookupEnv("TEST_OPENAI_API_KEY")
	if !ok {
		t.Skip("TEST_OPENAI_API_KEY is not set")
	}

	// Only run if explicitly requested via environment variable
	if os.Getenv("TEST_TOKEN_LIMIT_ERROR") != "true" {
		t.Skip("TEST_TOKEN_LIMIT_ERROR is not set to true")
	}

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx = ctxlog.With(ctx, logger)

	// Use gpt-5 (default model) which has 128k context limit
	client, err := openai.New(ctx, apiKey)
	gt.NoError(t, err)

	session, err := client.NewSession(ctx)
	gt.NoError(t, err)

	// Create a very long prompt to exceed token limit
	// gpt-5 may have larger context limit, so we create much more text
	// Approximately 1 token = 4 characters, aim for ~300k+ tokens
	longText := strings.Repeat("This is a test sentence to make the prompt very long. ", 25000)

	_, err = session.GenerateContent(ctx, gollem.Text(longText))
	gt.Error(t, err)

	// Log error details for debugging
	t.Logf("Error: %+v", err)
	t.Logf("Error tags: %v", goerr.Tags(err))

	// Verify the error has the token exceeded tag
	gt.True(t, goerr.HasTag(err, gollem.ErrTagTokenExceeded))
}
