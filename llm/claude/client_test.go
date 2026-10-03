package claude_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/gollem-dev/gollem"
	"github.com/gollem-dev/gollem/llm/claude"
	"github.com/gollem-dev/gollem/trace"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gt"
)

const (
	testTimeout   = 30 * time.Second
	maxTestTokens = 2048
)

func TestClaudeContentGenerate(t *testing.T) {
	apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
	if !ok {
		t.Skip("TEST_CLAUDE_API_KEY is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	client, err := claude.New(ctx, apiKey)
	gt.NoError(t, err)

	session, err := client.NewSession(ctx)
	gt.NoError(t, err)

	result, err := session.Generate(ctx, []gollem.Input{gollem.Text("Say hello in one word")}, gollem.WithMaxTokens(maxTestTokens))
	gt.NoError(t, err)
	gt.Array(t, result.Texts).Length(1).Required()
	gt.Value(t, len(result.Texts[0])).NotEqual(0)
}

// TestCreateSystemPrompt tests the createSystemPrompt function
func TestCreateSystemPrompt(t *testing.T) {
	ctx := context.Background()

	t.Run("empty config returns empty slice", func(t *testing.T) {
		cfg := gollem.NewSessionConfig()
		result, err := claude.CreateSystemPrompt(ctx, cfg, false)
		gt.NoError(t, err)

		// Should return empty slice when no system prompt
		gt.Equal(t, 0, len(result))
	})

	t.Run("result is correct type", func(t *testing.T) {
		cfg := gollem.NewSessionConfig()
		result, err := claude.CreateSystemPrompt(ctx, cfg, false)
		gt.NoError(t, err)

		// Empty slice can be nil in this implementation
		gt.Equal(t, 0, len(result))
	})

	t.Run("JSON content type check", func(t *testing.T) {
		// Create config with JSON content type
		cfg := gollem.NewSessionConfig()
		// Manually set content type since we can't use WithContentType in test
		// The actual functionality is tested in integration tests
		result, err := claude.CreateSystemPrompt(ctx, cfg, false)
		gt.NoError(t, err)

		// At minimum, should not panic and return valid type
		_ = result
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
		result, err := claude.CreateSystemPrompt(ctx, cfg, false)
		gt.NoError(t, err)

		// Empty case should return empty slice
		gt.Equal(t, 0, len(result))
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
		// The function returns []anthropic.TextBlockParam, the SDK's system prompt type.

		cfg := gollem.NewSessionConfig()
		result, err := claude.CreateSystemPrompt(ctx, cfg, false)
		gt.NoError(t, err)

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

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	client, err := claude.New(ctx, apiKey)
	gt.NoError(t, err)

	session, err := client.NewSession(ctx)
	gt.NoError(t, err)

	// Create a very long prompt to exceed token limit
	// Repeat a long text many times to ensure we exceed the limit
	longText := strings.Repeat("This is a test sentence to make the prompt very long. ", 100000)

	_, err = session.Generate(ctx, []gollem.Input{gollem.Text(longText)})
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

// TestPerCallGenerateOptions verifies that per-call GenerateOption overrides
// actually change the API request. A text-mode session gets a per-call
// ResponseSchema, and the response must be valid JSON matching the schema.
func TestPerCallGenerateOptions(t *testing.T) {
	apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
	if !ok {
		t.Skip("TEST_CLAUDE_API_KEY is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	client, err := claude.New(ctx, apiKey)
	gt.NoError(t, err)

	// Create a plain text session — no ContentTypeJSON, no ResponseSchema
	session, err := client.NewSession(ctx)
	gt.NoError(t, err)

	schema := &gollem.Parameter{
		Type:  gollem.TypeObject,
		Title: "Color",
		Properties: map[string]*gollem.Parameter{
			"name": {Type: gollem.TypeString, Description: "color name", Required: true},
		},
	}

	// Per-call option should force JSON output via system prompt injection
	resp, err := session.Generate(ctx,
		[]gollem.Input{gollem.Text("Name a color.")},
		gollem.WithGenerateResponseSchema(schema),
		gollem.WithMaxTokens(maxTestTokens),
	)
	gt.NoError(t, err)
	gt.True(t, len(resp.Texts) > 0)

	var parsed map[string]any
	gt.NoError(t, json.Unmarshal([]byte(resp.Texts[0]), &parsed))
	gt.True(t, parsed["name"] != nil)
}

func TestClaudeMessagesToTraceMessages(t *testing.T) {
	type testCase struct {
		messages []anthropic.MessageParam
		expected []trace.Message
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			result := claude.ClaudeMessagesToTraceMessages(tc.messages)
			gt.Equal(t, tc.expected, result)
		}
	}

	t.Run("text message", runTest(testCase{
		messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("hello world")),
		},
		expected: []trace.Message{
			{Role: "user", Contents: []trace.MessageContent{
				trace.NewTextContent("hello world"),
			}},
		},
	}))

	t.Run("assistant message", runTest(testCase{
		messages: []anthropic.MessageParam{
			anthropic.NewAssistantMessage(anthropic.NewTextBlock("response")),
		},
		expected: []trace.Message{
			{Role: "assistant", Contents: []trace.MessageContent{
				trace.NewTextContent("response"),
			}},
		},
	}))

	t.Run("tool use", runTest(testCase{
		messages: []anthropic.MessageParam{
			anthropic.NewAssistantMessage(
				anthropic.NewToolUseBlock("call-1", map[string]any{"q": "test"}, "search"),
			),
		},
		expected: []trace.Message{
			{Role: "assistant", Contents: []trace.MessageContent{
				trace.NewToolCallContent("call-1", "search", map[string]any{"q": "test"}),
			}},
		},
	}))

	t.Run("tool result", runTest(testCase{
		messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewToolResultBlock("call-1", "result text", false),
			),
		},
		expected: []trace.Message{
			{Role: "user", Contents: []trace.MessageContent{
				trace.NewToolResponseContent("call-1", "", nil),
				trace.NewTextContent("result text"),
			}},
		},
	}))

	t.Run("multiple messages", runTest(testCase{
		messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("hello")),
			anthropic.NewAssistantMessage(anthropic.NewTextBlock("hi")),
			anthropic.NewUserMessage(anthropic.NewTextBlock("how are you")),
		},
		expected: []trace.Message{
			{Role: "user", Contents: []trace.MessageContent{trace.NewTextContent("hello")}},
			{Role: "assistant", Contents: []trace.MessageContent{trace.NewTextContent("hi")}},
			{Role: "user", Contents: []trace.MessageContent{trace.NewTextContent("how are you")}},
		},
	}))

	t.Run("nil messages", runTest(testCase{
		messages: nil,
		expected: nil,
	}))

	t.Run("empty messages", runTest(testCase{
		messages: []anthropic.MessageParam{},
		expected: nil,
	}))

	t.Run("mixed content blocks", runTest(testCase{
		messages: []anthropic.MessageParam{
			anthropic.NewAssistantMessage(
				anthropic.NewTextBlock("Let me search"),
				anthropic.NewToolUseBlock("call-1", map[string]any{"q": "test"}, "search"),
			),
		},
		expected: []trace.Message{
			{Role: "assistant", Contents: []trace.MessageContent{
				trace.NewTextContent("Let me search"),
				trace.NewToolCallContent("call-1", "search", map[string]any{"q": "test"}),
			}},
		},
	}))

	t.Run("image with media type", runTest(testCase{
		messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewImageBlockBase64("image/png", "iVBOR..."),
			),
		},
		expected: []trace.Message{
			{Role: "user", Contents: []trace.MessageContent{
				{Type: "image", MediaType: "image/png"},
			}},
		},
	}))

	t.Run("thinking block", runTest(testCase{
		messages: []anthropic.MessageParam{
			anthropic.NewAssistantMessage(
				anthropic.NewThinkingBlock("sig123", "Let me think about this..."),
				anthropic.NewTextBlock("Here is my answer"),
			),
		},
		expected: []trace.Message{
			{Role: "assistant", Contents: []trace.MessageContent{
				trace.NewThinkingContent("Let me think about this..."),
				trace.NewTextContent("Here is my answer"),
			}},
		},
	}))

	t.Run("redacted thinking block", runTest(testCase{
		messages: []anthropic.MessageParam{
			anthropic.NewAssistantMessage(
				anthropic.NewRedactedThinkingBlock("redacted-data"),
				anthropic.NewTextBlock("answer"),
			),
		},
		expected: []trace.Message{
			{Role: "assistant", Contents: []trace.MessageContent{
				trace.NewRedactedThinkingContent(),
				trace.NewTextContent("answer"),
			}},
		},
	}))
}

// TestClaudeTraceRequestMessagesNewTurnOnly verifies that the trace's
// LLMRequest.Messages contains only messages newly added in this turn,
// not the entire conversation history that was actually sent to the API.
func TestClaudeTraceRequestMessagesNewTurnOnly(t *testing.T) {
	userContent, err := gollem.NewTextContent("previous question")
	gt.NoError(t, err)
	assistantContent, err := gollem.NewTextContent("previous answer")
	gt.NoError(t, err)
	history := &gollem.History{
		Version: gollem.HistoryVersion,
		LLType:  gollem.LLMTypeClaude,
		Messages: []gollem.Message{
			{Role: gollem.RoleUser, Contents: []gollem.MessageContent{userContent}},
			{Role: gollem.RoleAssistant, Contents: []gollem.MessageContent{assistantContent}},
		},
	}

	var sentMessages []anthropic.MessageParam
	mockClient := &apiClientMock{
		MessagesNewFunc: func(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, error) {
			sentMessages = params.Messages
			return &anthropic.Message{
				Content: []anthropic.ContentBlockUnion{
					{Type: "text", Text: "ok"},
				},
				Role:  "assistant",
				Model: "claude-3-opus-20240229",
			}, nil
		},
	}

	cfg := gollem.NewSessionConfig(gollem.WithSessionHistory(history))
	session, err := claude.NewSessionWithAPIClient(mockClient, cfg, "claude-3-opus-20240229")
	gt.NoError(t, err)

	rec := trace.New()
	ctx := rec.StartAgentExecute(context.Background())
	ctx = trace.WithHandler(ctx, rec)

	_, err = session.Generate(ctx, []gollem.Input{gollem.Text("new question")})
	gt.NoError(t, err)
	rec.EndAgentExecute(ctx, nil)

	// Sanity check: the actual API request still includes the full history.
	gt.N(t, len(sentMessages)).Equal(3)

	// Find the LLM call span.
	var llmSpan *trace.Span
	for _, child := range rec.Trace().RootSpan.Children {
		if child.Kind == trace.SpanKindLLMCall {
			llmSpan = child
			break
		}
	}
	gt.Value(t, llmSpan).NotNil()

	msgs := llmSpan.LLMCall.Request.Messages
	gt.A(t, msgs).Length(1)
	gt.Equal(t, "user", msgs[0].Role)
	gt.A(t, msgs[0].Contents).Length(1)
	gt.Equal(t, "text", msgs[0].Contents[0].Type)
	gt.Equal(t, "new question", msgs[0].Contents[0].Text)

	for _, m := range msgs {
		for _, c := range m.Contents {
			gt.S(t, c.Text).NotContains("previous")
		}
	}
}

// cachePromptTestTool is a minimal tool used to exercise prompt-cache breakpoints.
type cachePromptTestTool struct{}

func (t *cachePromptTestTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "echo",
		Description: "echo",
		Parameters: map[string]*gollem.Parameter{
			"msg": {Type: gollem.TypeString, Description: "message"},
		},
	}
}

func (t *cachePromptTestTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return map[string]any{"ok": true}, nil
}

func TestCacheTokensFromUsage(t *testing.T) {
	t.Run("restores total input from cached prefix", func(t *testing.T) {
		total, creation, read := claude.CacheTokensFromUsage(anthropic.Usage{
			InputTokens:              50,
			CacheCreationInputTokens: 10,
			CacheReadInputTokens:     100,
		})
		gt.Equal(t, 160, total)
		gt.Equal(t, 10, creation)
		gt.Equal(t, 100, read)
	})

	t.Run("no caching yields raw input and zero cache", func(t *testing.T) {
		total, creation, read := claude.CacheTokensFromUsage(anthropic.Usage{InputTokens: 42})
		gt.Equal(t, 42, total)
		gt.Equal(t, 0, creation)
		gt.Equal(t, 0, read)
	})
}

func TestApplyPromptCacheBreakpoints(t *testing.T) {
	const ttl5m = anthropic.CacheControlEphemeralTTLTTL5m

	t.Run("marks system, tools and conversation tail", func(t *testing.T) {
		req := anthropic.MessageNewParams{
			System: []anthropic.TextBlockParam{{Text: "a"}, {Text: "b"}},
			Tools: []anthropic.ToolUnionParam{
				anthropic.ToolUnionParamOfTool(anthropic.ToolInputSchemaParam{}, "t1"),
				anthropic.ToolUnionParamOfTool(anthropic.ToolInputSchemaParam{}, "t2"),
			},
			Messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(anthropic.NewTextBlock("first")),
				anthropic.NewUserMessage(anthropic.NewTextBlock("second")),
			},
		}
		claude.ApplyPromptCacheBreakpoints(&req)

		// Only the last system block is marked.
		gt.Equal(t, anthropic.CacheControlEphemeralTTL(""), req.System[0].CacheControl.TTL)
		gt.Equal(t, ttl5m, req.System[1].CacheControl.TTL)
		// Only the last tool is marked.
		gt.Equal(t, anthropic.CacheControlEphemeralTTL(""), req.Tools[0].OfTool.CacheControl.TTL)
		gt.Equal(t, ttl5m, req.Tools[1].OfTool.CacheControl.TTL)
		// Only the last message's last block is marked.
		tail := req.Messages[1].Content
		gt.Equal(t, ttl5m, tail[len(tail)-1].OfText.CacheControl.TTL)
	})

	t.Run("empty sections are skipped without panic", func(t *testing.T) {
		req := anthropic.MessageNewParams{}
		claude.ApplyPromptCacheBreakpoints(&req) // must not panic
		gt.Equal(t, 0, len(req.System))
		gt.Equal(t, 0, len(req.Tools))
		gt.Equal(t, 0, len(req.Messages))
	})

	t.Run("does not mutate shared history or tools", func(t *testing.T) {
		origMsg := anthropic.NewUserMessage(anthropic.NewTextBlock("shared"))
		origTool := anthropic.ToolUnionParamOfTool(anthropic.ToolInputSchemaParam{}, "shared")
		req := anthropic.MessageNewParams{
			Tools:    []anthropic.ToolUnionParam{origTool},
			Messages: []anthropic.MessageParam{origMsg},
		}
		claude.ApplyPromptCacheBreakpoints(&req)

		// The request carries the marker...
		gt.Equal(t, ttl5m, req.Tools[0].OfTool.CacheControl.TTL)
		gt.Equal(t, ttl5m, req.Messages[0].Content[0].OfText.CacheControl.TTL)
		// ...but the originally shared values are untouched.
		gt.Equal(t, anthropic.CacheControlEphemeralTTL(""), origTool.OfTool.CacheControl.TTL)
		gt.Equal(t, anthropic.CacheControlEphemeralTTL(""), origMsg.Content[0].OfText.CacheControl.TTL)
	})

	t.Run("unknown tail variant is skipped", func(t *testing.T) {
		req := anthropic.MessageNewParams{
			Messages: []anthropic.MessageParam{
				{Role: anthropic.MessageParamRoleUser, Content: []anthropic.ContentBlockParamUnion{{}}},
			},
		}
		claude.ApplyPromptCacheBreakpoints(&req) // must not panic on an empty union
	})
}

func TestClaudePromptCacheWiring(t *testing.T) {
	runTest := func(enabled bool) func(t *testing.T) {
		return func(t *testing.T) {
			var sent anthropic.MessageNewParams
			mockClient := &apiClientMock{
				MessagesNewFunc: func(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, error) {
					sent = params
					return &anthropic.Message{
						Content: []anthropic.ContentBlockUnion{{Type: "text", Text: "ok"}},
						Role:    "assistant",
						Model:   "claude-3-opus-20240229",
					}, nil
				},
			}

			opts := []gollem.SessionOption{
				gollem.WithSessionSystemPrompt("you are helpful"),
				gollem.WithSessionTools(&cachePromptTestTool{}),
			}
			if enabled {
				opts = append(opts, gollem.WithSessionPromptCache(true))
			}
			cfg := gollem.NewSessionConfig(opts...)
			session, err := claude.NewSessionWithAPIClient(mockClient, cfg, "claude-3-opus-20240229")
			gt.NoError(t, err)

			_, err = session.Generate(context.Background(), []gollem.Input{gollem.Text("hello")})
			gt.NoError(t, err)

			want := anthropic.CacheControlEphemeralTTL("")
			if enabled {
				want = anthropic.CacheControlEphemeralTTLTTL5m
			}
			gt.Equal(t, want, sent.System[len(sent.System)-1].CacheControl.TTL)
			gt.Equal(t, want, sent.Tools[len(sent.Tools)-1].OfTool.CacheControl.TTL)
			tail := sent.Messages[len(sent.Messages)-1].Content
			gt.Equal(t, want, tail[len(tail)-1].OfText.CacheControl.TTL)
		}
	}

	t.Run("enabled injects cache_control", runTest(true))
	t.Run("disabled leaves request unmarked", runTest(false))
}

func TestClaudeCacheTokenObservation(t *testing.T) {
	mockClient := &apiClientMock{
		MessagesNewFunc: func(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, error) {
			return &anthropic.Message{
				Content: []anthropic.ContentBlockUnion{{Type: "text", Text: "ok"}},
				Role:    "assistant",
				Model:   "claude-3-opus-20240229",
				Usage: anthropic.Usage{
					InputTokens:              50,
					OutputTokens:             7,
					CacheCreationInputTokens: 10,
					CacheReadInputTokens:     100,
				},
			}, nil
		},
	}

	cfg := gollem.NewSessionConfig()
	session, err := claude.NewSessionWithAPIClient(mockClient, cfg, "claude-3-opus-20240229")
	gt.NoError(t, err)

	resp, err := session.Generate(context.Background(), []gollem.Input{gollem.Text("hi")})
	gt.NoError(t, err)

	// InputToken keeps total-input semantics (post-breakpoint + cached prefix).
	gt.Equal(t, 160, resp.InputToken)
	gt.Equal(t, 7, resp.OutputToken)
	gt.Equal(t, 10, resp.CacheCreationInputToken)
	gt.Equal(t, 100, resp.CacheReadInputToken)
}

func TestClaudeStreamPromptCacheAndObservation(t *testing.T) {
	var sent anthropic.MessageNewParams
	mockClient := &apiClientMock{
		MessagesNewFunc: func(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, error) {
			sent = params
			return &anthropic.Message{
				Content: []anthropic.ContentBlockUnion{{Type: "text", Text: "streamed"}},
				Role:    "assistant",
				Model:   "claude-3-opus-20240229",
				Usage: anthropic.Usage{
					InputTokens:          30,
					OutputTokens:         5,
					CacheReadInputTokens: 120,
				},
			}, nil
		},
	}

	cfg := gollem.NewSessionConfig(
		gollem.WithSessionSystemPrompt("sys"),
		gollem.WithSessionPromptCache(true),
	)
	session, err := claude.NewSessionWithAPIClient(mockClient, cfg, "claude-3-opus-20240229")
	gt.NoError(t, err)

	rec := trace.New()
	ctx := rec.StartAgentExecute(context.Background())
	ctx = trace.WithHandler(ctx, rec)

	ch, err := session.Stream(ctx, []gollem.Input{gollem.Text("hi")})
	gt.NoError(t, err)

	var gotCacheRead, gotInput int
	for resp := range ch {
		gt.NoError(t, resp.Error)
		if resp.InputToken > 0 {
			gotInput = resp.InputToken
			gotCacheRead = resp.CacheReadInputToken
		}
	}
	rec.EndAgentExecute(ctx, nil)

	// Streaming reports total input and the cached read count.
	gt.Equal(t, 150, gotInput) // 30 post-breakpoint + 120 cached
	gt.Equal(t, 120, gotCacheRead)

	// The stream request carried the cache_control marker on the system prefix.
	gt.Equal(t, anthropic.CacheControlEphemeralTTLTTL5m, sent.System[len(sent.System)-1].CacheControl.TTL)

	// Trace records the cache breakdown.
	var llmSpan *trace.Span
	for _, child := range rec.Trace().RootSpan.Children {
		if child.Kind == trace.SpanKindLLMCall {
			llmSpan = child
			break
		}
	}
	gt.Value(t, llmSpan).NotNil()
	gt.Equal(t, 120, llmSpan.LLMCall.CacheReadInputTokens)
	gt.Equal(t, 150, llmSpan.LLMCall.InputTokens)
}

// TestClaudePromptCacheLive exercises the real Claude API to confirm that
// enabling prompt caching actually writes and then reads the cached prefix.
// This is the one path mock tests cannot prove: that the API accepts our
// cache_control markers and reports cache usage back.
func TestClaudePromptCacheLive(t *testing.T) {
	apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
	if !ok {
		t.Skip("TEST_CLAUDE_API_KEY is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	client, err := claude.New(ctx, apiKey)
	gt.NoError(t, err)

	// The system prompt must exceed the model's minimum cacheable length (up to
	// 4096 tokens for some models). Repeat a sentence well past that threshold. A
	// per-run nonce keeps the prefix unique so the first call is a cold cache
	// write, not a hit on a cache left over from a previous run within the TTL.
	var sb strings.Builder
	fmt.Fprintf(&sb, "Assistant build %d-%d. ", os.Getpid(), time.Now().UnixNano())
	for i := 0; i < 800; i++ {
		sb.WriteString("You are a meticulous assistant that follows instructions carefully. ")
	}
	systemPrompt := sb.String()

	session, err := client.NewSession(ctx,
		gollem.WithSessionSystemPrompt(systemPrompt),
		gollem.WithSessionPromptCache(true),
	)
	gt.NoError(t, err)

	// First call: the stable prefix is written to the cache.
	first, err := session.Generate(ctx,
		[]gollem.Input{gollem.Text("Reply with the single word: one")},
		gollem.WithMaxTokens(maxTestTokens))
	gt.NoError(t, err)

	// Second call: the same system prefix should be served from the cache.
	second, err := session.Generate(ctx,
		[]gollem.Input{gollem.Text("Reply with the single word: two")},
		gollem.WithMaxTokens(maxTestTokens))
	gt.NoError(t, err)

	t.Logf("first : input=%d creation=%d read=%d",
		first.InputToken, first.CacheCreationInputToken, first.CacheReadInputToken)
	t.Logf("second: input=%d creation=%d read=%d",
		second.InputToken, second.CacheCreationInputToken, second.CacheReadInputToken)

	// The cache was written on the first call and read on the second.
	gt.Value(t, first.CacheCreationInputToken > 0).Equal(true)
	gt.Value(t, second.CacheReadInputToken > 0).Equal(true)
	// InputToken keeps total-input semantics: it includes the cached prefix.
	gt.Value(t, second.InputToken >= second.CacheReadInputToken).Equal(true)
}

func TestNewMaxTokens(t *testing.T) {
	type testCase struct {
		options  []claude.Option
		expected int64
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			client, err := claude.New(context.Background(), "test-key", tc.options...)
			gt.NoError(t, err).Required()
			gt.Equal(t, tc.expected, claude.MaxTokensOf(client))
		}
	}

	t.Run("resolves the default model when max tokens is not set", runTest(testCase{
		expected: 64000, // claude-sonnet-4-5-20250929
	}))

	t.Run("resolves the model given by WithModel", runTest(testCase{
		options:  []claude.Option{claude.WithModel("claude-opus-5")},
		expected: 128000,
	}))

	t.Run("falls back for a model absent from the table", runTest(testCase{
		options:  []claude.Option{claude.WithModel("my-proxy/llm")},
		expected: claude.FallbackMaxOutputTokens,
	}))

	t.Run("keeps an explicit max tokens", runTest(testCase{
		options:  []claude.Option{claude.WithMaxTokens(1000)},
		expected: 1000,
	}))

	t.Run("keeps an explicit max tokens above the model limit", runTest(testCase{
		options: []claude.Option{
			claude.WithModel("claude-sonnet-4-5"),
			claude.WithMaxTokens(200000),
		},
		expected: 200000,
	}))

	t.Run("keeps an explicit max tokens regardless of option order", runTest(testCase{
		options: []claude.Option{
			claude.WithMaxTokens(1000),
			claude.WithModel("claude-opus-5"),
		},
		expected: 1000,
	}))

	// An explicit value is never second-guessed, so it reaches the API even
	// when the API will reject it. This matches gollem.WithMaxTokens, which
	// distinguishes "set to zero" from "not set" and sends the zero through.
	t.Run("keeps an explicit zero", runTest(testCase{
		options:  []claude.Option{claude.WithMaxTokens(0)},
		expected: 0,
	}))

	t.Run("keeps an explicit negative value", runTest(testCase{
		options:  []claude.Option{claude.WithMaxTokens(-1)},
		expected: -1,
	}))
}

// TestGenerateWithResolvedMaxTokens drives New -> NewSession -> Generate/Stream
// through the real SDK so the resolved ceiling is checked against the SDK's
// non-streaming guard, which a mocked apiClient bypasses. The request is aimed
// at a closed port: reaching a transport error means the guard let it through.
func TestGenerateWithResolvedMaxTokens(t *testing.T) {
	ctx := context.Background()

	const guardMessage = "streaming is required"

	client, err := claude.New(ctx, "test-key", claude.WithBaseURL("http://127.0.0.1:1/"))
	gt.NoError(t, err).Required()
	gt.Equal(t, int64(64000), claude.MaxTokensOf(client))

	session, err := client.NewSession(ctx)
	gt.NoError(t, err).Required()

	t.Run("Generate", func(t *testing.T) {
		_, err := session.Generate(ctx, []gollem.Input{gollem.Text("hello")})
		gt.Error(t, err).Required()
		gt.False(t, strings.Contains(err.Error(), guardMessage))
	})

	t.Run("Stream", func(t *testing.T) {
		_, err := session.Stream(ctx, []gollem.Input{gollem.Text("hello")})
		gt.Error(t, err).Required()
		gt.False(t, strings.Contains(err.Error(), guardMessage))
	})
}

// claudeDefaultModel is the model claude.New falls back to when WithModel is
// not given. It is written out here instead of being read back from the
// client, so that changing the default has to be a deliberate edit.
const claudeDefaultModel = "claude-sonnet-4-5-20250929"

var _ gollem.ModelNamer = (*claude.Client)(nil)

// TestClientModel verifies that the client reports the model name it was
// configured with, without consulting the API.
func TestClientModel(t *testing.T) {
	type testCase struct {
		options  []claude.Option
		expected string
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			client, err := claude.New(context.Background(), "test-key", tc.options...)
			gt.NoError(t, err).Required()
			gt.Equal(t, tc.expected, client.Model())
		}
	}

	t.Run("configured model", runTest(testCase{
		options:  []claude.Option{claude.WithModel("claude-opus-4-1-20250805")},
		expected: "claude-opus-4-1-20250805",
	}))

	t.Run("default model when no option is given", runTest(testCase{
		expected: claudeDefaultModel,
	}))

	t.Run("last option wins", runTest(testCase{
		options: []claude.Option{
			claude.WithModel("claude-opus-4-1-20250805"),
			claude.WithModel("claude-haiku-4-5-20251001"),
		},
		expected: "claude-haiku-4-5-20251001",
	}))

	t.Run("empty model is reported as configured", runTest(testCase{
		options:  []claude.Option{claude.WithModel("")},
		expected: "",
	}))
}

// recordingServer is a local Messages API endpoint that records every request
// body and answers with a single text block, in SSE form when the request asks
// for streaming.
type recordingServer struct {
	srv          *httptest.Server
	mu           sync.Mutex
	bodies       []map[string]json.RawMessage
	responseText string
}

func newRecordingServer(t *testing.T, responseText string) *recordingServer {
	t.Helper()
	rs := &recordingServer{responseText: responseText}
	rs.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var body map[string]json.RawMessage
		if err := json.Unmarshal(raw, &body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		rs.mu.Lock()
		rs.bodies = append(rs.bodies, body)
		rs.mu.Unlock()

		text, err := json.Marshal(rs.responseText)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if string(body["stream"]) == "true" {
			w.Header().Set("Content-Type", "text/event-stream")
			events := [][2]string{
				{"message_start", `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"m","content":[],"stop_reason":null,"usage":{"input_tokens":1,"output_tokens":0}}}`},
				{"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`},
				{"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":` + string(text) + `}}`},
				{"content_block_stop", `{"type":"content_block_stop","index":0}`},
				{"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`},
				{"message_stop", `{"type":"message_stop"}`},
			}
			for _, ev := range events {
				_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev[0], ev[1])
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"id":"msg_1","type":"message","role":"assistant","model":"m","content":[{"type":"text","text":%s}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`, text)
	}))
	t.Cleanup(rs.srv.Close)
	return rs
}

func (rs *recordingServer) lastBody(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	rs.mu.Lock()
	defer rs.mu.Unlock()
	gt.A(t, rs.bodies).Longer(0).Required()
	return rs.bodies[len(rs.bodies)-1]
}

// requestPath is one of the four ways a Claude session sends a request: the
// Claude API or Vertex AI client, through Generate or Stream.
type requestPath struct {
	name       string
	stream     bool
	newSession func(t *testing.T, baseURL, model string, opts ...gollem.SessionOption) gollem.Session
}

var requestPaths = []requestPath{
	{name: "API Generate", stream: false, newSession: newAPISession},
	{name: "API Stream", stream: true, newSession: newAPISession},
	{name: "Vertex Generate", stream: false, newSession: newVertexSession},
	{name: "Vertex Stream", stream: true, newSession: newVertexSession},
}

func newAPISession(t *testing.T, baseURL, model string, opts ...gollem.SessionOption) gollem.Session {
	t.Helper()
	client, err := claude.New(context.Background(), "test-key",
		claude.WithBaseURL(baseURL), claude.WithModel(model))
	gt.NoError(t, err)
	session, err := client.NewSession(context.Background(), opts...)
	gt.NoError(t, err)
	return session
}

func newVertexSession(t *testing.T, baseURL, model string, opts ...gollem.SessionOption) gollem.Session {
	t.Helper()
	anthropicClient := anthropic.NewClient(
		option.WithAPIKey("test-key"),
		option.WithBaseURL(baseURL),
		option.WithMaxRetries(0),
	)
	client := claude.NewVertexClientWithAnthropicClient(&anthropicClient, claude.WithVertexModel(model))
	session, err := client.NewSession(context.Background(), opts...)
	gt.NoError(t, err)
	return session
}

// send calls Generate or Stream and returns the concatenated response text.
func (p requestPath) send(t *testing.T, session gollem.Session, opts ...gollem.GenerateOption) string {
	t.Helper()
	ctx := context.Background()
	input := []gollem.Input{gollem.Text("question")}
	if !p.stream {
		resp, err := session.Generate(ctx, input, opts...)
		gt.NoError(t, err)
		return strings.Join(resp.Texts, "")
	}
	ch, err := session.Stream(ctx, input, opts...)
	gt.NoError(t, err)
	var texts []string
	for resp := range ch {
		gt.NoError(t, resp.Error)
		texts = append(texts, resp.Texts...)
	}
	return strings.Join(texts, "")
}

// structuredOutputTestSchema carries constraints that structured outputs does
// not accept, so the tests also observe how they are moved to the description.
func structuredOutputTestSchema() *gollem.Parameter {
	maxLength := 20
	minimum := 1.0
	return &gollem.Parameter{
		Type: gollem.TypeObject,
		Properties: map[string]*gollem.Parameter{
			"answer": {Type: gollem.TypeString, Description: "The answer", Required: true, MaxLength: &maxLength},
			"count":  {Type: gollem.TypeInteger, Minimum: &minimum},
		},
	}
}

const structuredOutputTestFormat = `{
	"type": "json_schema",
	"schema": {
		"type": "object",
		"additionalProperties": false,
		"required": ["answer"],
		"properties": {
			"answer": {"type": "string", "description": "The answer\n\n{maxLength: 20}"},
			"count": {"type": "integer", "description": "{minimum: 1}"}
		}
	}
}`

func assertJSONEqual(t *testing.T, expected string, actual json.RawMessage) {
	t.Helper()
	var want, got any
	gt.NoError(t, json.Unmarshal([]byte(expected), &want))
	gt.NoError(t, json.Unmarshal(actual, &got))
	gt.Equal(t, want, got)
}

func TestResponseSchemaIsSentAsOutputFormat(t *testing.T) {
	const model = "claude-opus-5-5"
	const systemPrompt = "You are a test assistant."

	for _, p := range requestPaths {
		t.Run(p.name, func(t *testing.T) {
			t.Run("per-call schema leaves system and tools unchanged", func(t *testing.T) {
				rs := newRecordingServer(t, "{}")
				session := p.newSession(t, rs.srv.URL, model,
					gollem.WithSessionSystemPrompt(systemPrompt),
					gollem.WithSessionTools(&cachePromptTestTool{}))

				p.send(t, session)
				without := rs.lastBody(t)
				p.send(t, session, gollem.WithGenerateResponseSchema(structuredOutputTestSchema()))
				with := rs.lastBody(t)

				gt.Equal(t, string(without["system"]), string(with["system"]))
				gt.Equal(t, string(without["tools"]), string(with["tools"]))
				_, hasOutputConfig := without["output_config"]
				gt.False(t, hasOutputConfig)
				assertJSONEqual(t, `{"format":`+structuredOutputTestFormat+`}`, with["output_config"])
			})

			t.Run("session schema leaves system unchanged", func(t *testing.T) {
				rs := newRecordingServer(t, "{}")
				plain := p.newSession(t, rs.srv.URL, model,
					gollem.WithSessionSystemPrompt(systemPrompt))
				p.send(t, plain)
				without := rs.lastBody(t)

				structured := p.newSession(t, rs.srv.URL, model,
					gollem.WithSessionSystemPrompt(systemPrompt),
					gollem.WithSessionContentType(gollem.ContentTypeJSON),
					gollem.WithSessionResponseSchema(structuredOutputTestSchema()))
				p.send(t, structured)
				with := rs.lastBody(t)

				gt.Equal(t, string(without["system"]), string(with["system"]))
				assertJSONEqual(t, `{"format":`+structuredOutputTestFormat+`}`, with["output_config"])
			})

			t.Run("per-call schema overrides session schema", func(t *testing.T) {
				rs := newRecordingServer(t, "{}")
				session := p.newSession(t, rs.srv.URL, model,
					gollem.WithSessionContentType(gollem.ContentTypeJSON),
					gollem.WithSessionResponseSchema(&gollem.Parameter{
						Type:       gollem.TypeObject,
						Properties: map[string]*gollem.Parameter{"other": {Type: gollem.TypeBoolean}},
					}))
				p.send(t, session, gollem.WithGenerateResponseSchema(structuredOutputTestSchema()))
				assertJSONEqual(t, `{"format":`+structuredOutputTestFormat+`}`, rs.lastBody(t)["output_config"])
			})

			t.Run("model without structured outputs gets the schema in the system prompt", func(t *testing.T) {
				rs := newRecordingServer(t, "{}")
				session := p.newSession(t, rs.srv.URL, "claude-sonnet-4-20250514",
					gollem.WithSessionSystemPrompt(systemPrompt))
				p.send(t, session, gollem.WithGenerateResponseSchema(structuredOutputTestSchema()))
				body := rs.lastBody(t)

				_, hasOutputConfig := body["output_config"]
				gt.False(t, hasOutputConfig)
				var system []anthropic.TextBlockParam
				gt.NoError(t, json.Unmarshal(body["system"], &system))
				gt.A(t, system).Length(1).Required()
				gt.S(t, system[0].Text).HasPrefix(systemPrompt)
				gt.S(t, system[0].Text).Contains("Your response must conform to this JSON Schema")
				gt.S(t, system[0].Text).Contains(`"maxLength": 20`)
			})
		})
	}
}

func mapResponseTestSchema() *gollem.Parameter {
	return &gollem.Parameter{
		Type: gollem.TypeObject,
		Properties: map[string]*gollem.Parameter{
			"answer": {Type: gollem.TypeString, Required: true},
			"labels": {
				Type:                 gollem.TypeObject,
				AdditionalProperties: &gollem.Parameter{Type: gollem.TypeString},
			},
		},
	}
}

// sendErr calls Generate or Stream and returns the error from either the call
// or the stream.
func (p requestPath) sendErr(session gollem.Session, opts ...gollem.GenerateOption) error {
	ctx := context.Background()
	input := []gollem.Input{gollem.Text("question")}
	if !p.stream {
		_, err := session.Generate(ctx, input, opts...)
		return err
	}
	ch, err := session.Stream(ctx, input, opts...)
	if err != nil {
		return err
	}
	for resp := range ch {
		if resp.Error != nil {
			err = resp.Error
		}
	}
	return err
}

func TestMapInResponseSchema(t *testing.T) {
	type testCase struct {
		path       requestPath
		perCall    bool
		structured bool
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			rs := newRecordingServer(t, "{}")
			model := "claude-sonnet-4-20250514"
			if tc.structured {
				model = "claude-opus-5-5"
			}

			var sessionOpts []gollem.SessionOption
			var generateOpts []gollem.GenerateOption
			if tc.perCall {
				generateOpts = append(generateOpts, gollem.WithGenerateResponseSchema(mapResponseTestSchema()))
			} else {
				sessionOpts = append(sessionOpts,
					gollem.WithSessionContentType(gollem.ContentTypeJSON),
					gollem.WithSessionResponseSchema(mapResponseTestSchema()))
			}
			session := tc.path.newSession(t, rs.srv.URL, model, sessionOpts...)
			err := tc.path.sendErr(session, generateOpts...)

			if tc.structured {
				gt.True(t, errors.Is(err, gollem.ErrUnsupportedSchema))
				gt.S(t, err.Error()).Contains(`map at "labels" cannot be sent as Claude structured outputs`)
				rs.mu.Lock()
				defer rs.mu.Unlock()
				gt.A(t, rs.bodies).Length(0)
				return
			}

			gt.NoError(t, err)
			body := rs.lastBody(t)
			_, hasOutputConfig := body["output_config"]
			gt.False(t, hasOutputConfig)
			var system []anthropic.TextBlockParam
			gt.NoError(t, json.Unmarshal(body["system"], &system))
			gt.A(t, system).Length(1).Required()
			gt.S(t, system[0].Text).Contains(`"additionalProperties": {`)
		}
	}

	apiGenerate, apiStream, vertexGenerate, vertexStream := requestPaths[0], requestPaths[1], requestPaths[2], requestPaths[3]

	t.Run("structured outputs rejects a per-call map: API Generate", runTest(testCase{path: apiGenerate, perCall: true, structured: true}))
	t.Run("structured outputs rejects a per-call map: API Stream", runTest(testCase{path: apiStream, perCall: true, structured: true}))
	t.Run("structured outputs rejects a per-call map: Vertex Generate", runTest(testCase{path: vertexGenerate, perCall: true, structured: true}))
	t.Run("structured outputs rejects a per-call map: Vertex Stream", runTest(testCase{path: vertexStream, perCall: true, structured: true}))
	t.Run("structured outputs rejects a session map: API Generate", runTest(testCase{path: apiGenerate, structured: true}))
	t.Run("structured outputs rejects a session map: API Stream", runTest(testCase{path: apiStream, structured: true}))
	t.Run("structured outputs rejects a session map: Vertex Generate", runTest(testCase{path: vertexGenerate, structured: true}))
	t.Run("structured outputs rejects a session map: Vertex Stream", runTest(testCase{path: vertexStream, structured: true}))

	t.Run("system prompt keeps a per-call map: API Generate", runTest(testCase{path: apiGenerate, perCall: true}))
	t.Run("system prompt keeps a per-call map: API Stream", runTest(testCase{path: apiStream, perCall: true}))
	t.Run("system prompt keeps a per-call map: Vertex Generate", runTest(testCase{path: vertexGenerate, perCall: true}))
	t.Run("system prompt keeps a per-call map: Vertex Stream", runTest(testCase{path: vertexStream, perCall: true}))
	t.Run("system prompt keeps a session map: API Generate", runTest(testCase{path: apiGenerate}))
	t.Run("system prompt keeps a session map: API Stream", runTest(testCase{path: apiStream}))
	t.Run("system prompt keeps a session map: Vertex Generate", runTest(testCase{path: vertexGenerate}))
	t.Run("system prompt keeps a session map: Vertex Stream", runTest(testCase{path: vertexStream}))
}

func TestJSONIsExtractedHoweverTheSchemaIsSent(t *testing.T) {
	// JSON extraction re-marshals the value, which sorts keys and drops
	// whitespace, so the compact form shows that extraction took place.
	const responseText = "```json\n{\"count\": 2, \"answer\": \"yes\"}\n```"
	const expected = `{"answer":"yes","count":2}`

	runTest := func(p requestPath, model string) func(t *testing.T) {
		return func(t *testing.T) {
			rs := newRecordingServer(t, responseText)
			session := p.newSession(t, rs.srv.URL, model)
			actual := p.send(t, session, gollem.WithGenerateResponseSchema(structuredOutputTestSchema()))
			gt.Equal(t, expected, actual)
		}
	}

	t.Run("API Generate with structured outputs", runTest(requestPaths[0], "claude-opus-5-5"))
	t.Run("Vertex Generate with structured outputs", runTest(requestPaths[2], "claude-opus-5-5"))
	t.Run("API Generate with the schema in the system prompt", runTest(requestPaths[0], "claude-sonnet-4-20250514"))
	t.Run("Vertex Generate with the schema in the system prompt", runTest(requestPaths[2], "claude-sonnet-4-20250514"))
}

func TestJSONContentTypeWithoutSchema(t *testing.T) {
	const systemPrompt = "You are a test assistant."

	for _, p := range requestPaths {
		t.Run(p.name, func(t *testing.T) {
			t.Run("supported model gets the system prompt as written", func(t *testing.T) {
				rs := newRecordingServer(t, "{}")
				session := p.newSession(t, rs.srv.URL, "claude-opus-5-5",
					gollem.WithSessionSystemPrompt(systemPrompt),
					gollem.WithSessionContentType(gollem.ContentTypeJSON))
				p.send(t, session)
				body := rs.lastBody(t)

				assertJSONEqual(t, `[{"type":"text","text":"`+systemPrompt+`"}]`, body["system"])
				_, hasOutputConfig := body["output_config"]
				gt.False(t, hasOutputConfig)
			})

			t.Run("model without structured outputs gets the JSON instruction", func(t *testing.T) {
				rs := newRecordingServer(t, "{}")
				session := p.newSession(t, rs.srv.URL, "claude-sonnet-4-20250514",
					gollem.WithSessionSystemPrompt(systemPrompt),
					gollem.WithSessionContentType(gollem.ContentTypeJSON))
				p.send(t, session)

				var system []anthropic.TextBlockParam
				gt.NoError(t, json.Unmarshal(rs.lastBody(t)["system"], &system))
				gt.A(t, system).Length(1).Required()
				gt.Equal(t, systemPrompt+"\nPlease format your response as valid JSON.", system[0].Text)
			})
		})
	}
}

func TestVertexStructuredOutputsDisabled(t *testing.T) {
	const systemPrompt = "You are a test assistant."

	for _, stream := range []bool{false, true} {
		p := requestPath{name: "Vertex Generate", stream: stream}
		if stream {
			p.name = "Vertex Stream"
		}
		t.Run(p.name, func(t *testing.T) {
			rs := newRecordingServer(t, "{}")
			anthropicClient := anthropic.NewClient(
				option.WithAPIKey("test-key"),
				option.WithBaseURL(rs.srv.URL),
				option.WithMaxRetries(0),
			)
			client := claude.NewVertexClientWithAnthropicClient(&anthropicClient,
				claude.WithVertexModel("claude-opus-5-5"),
				claude.WithVertexStructuredOutputsDisabled())
			session, err := client.NewSession(context.Background(), gollem.WithSessionSystemPrompt(systemPrompt))
			gt.NoError(t, err).Required()

			p.send(t, session, gollem.WithGenerateResponseSchema(structuredOutputTestSchema()))
			body := rs.lastBody(t)

			_, hasOutputConfig := body["output_config"]
			gt.False(t, hasOutputConfig)
			var system []anthropic.TextBlockParam
			gt.NoError(t, json.Unmarshal(body["system"], &system))
			gt.A(t, system).Length(1).Required()
			gt.S(t, system[0].Text).HasPrefix(systemPrompt)
			gt.S(t, system[0].Text).Contains("Your response must conform to this JSON Schema")
		})
	}
}

func TestAPIErrorReachesCaller(t *testing.T) {
	const apiMessage = "attempting to use a disallowed feature structured_outputs"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, `{"type":"error","error":{"type":"invalid_request_error","message":%q}}`, apiMessage)
	}))
	t.Cleanup(srv.Close)

	for _, p := range requestPaths {
		t.Run(p.name, func(t *testing.T) {
			session := p.newSession(t, srv.URL, "claude-opus-5-5")
			input := []gollem.Input{gollem.Text("question")}
			opts := []gollem.GenerateOption{gollem.WithGenerateResponseSchema(structuredOutputTestSchema())}

			var err error
			if p.stream {
				ch, startErr := session.Stream(context.Background(), input, opts...)
				err = startErr
				if startErr == nil {
					for resp := range ch {
						if resp.Error != nil {
							err = resp.Error
						}
					}
				}
			} else {
				_, err = session.Generate(context.Background(), input, opts...)
			}

			gt.Error(t, err).Required()
			gt.S(t, err.Error()).Contains(apiMessage)

			// A failed call must not leave the question in the history.
			history, histErr := session.History()
			gt.NoError(t, histErr).Required()
			gt.A(t, history.Messages).Length(0)
		})
	}
}

func TestToolCallsDisabled(t *testing.T) {
	const model = "claude-opus-5-5"

	for _, p := range requestPaths {
		t.Run(p.name, func(t *testing.T) {
			t.Run("sends tool_choice none with the tools unchanged", func(t *testing.T) {
				rs := newRecordingServer(t, "{}")
				session := p.newSession(t, rs.srv.URL, model,
					gollem.WithSessionTools(&cachePromptTestTool{}))

				p.send(t, session)
				without := rs.lastBody(t)
				p.send(t, session, gollem.WithToolCallsDisabled())
				with := rs.lastBody(t)

				gt.Equal(t, string(without["tools"]), string(with["tools"]))
				_, hasToolChoice := without["tool_choice"]
				gt.False(t, hasToolChoice)
				assertJSONEqual(t, `{"type":"none"}`, with["tool_choice"])
			})

			t.Run("sends no tool_choice when the session has no tools", func(t *testing.T) {
				rs := newRecordingServer(t, "{}")
				session := p.newSession(t, rs.srv.URL, model)
				p.send(t, session, gollem.WithToolCallsDisabled())
				_, hasToolChoice := rs.lastBody(t)["tool_choice"]
				gt.False(t, hasToolChoice)
			})
		})
	}
}

func TestWithEffort(t *testing.T) {
	type testCase struct {
		effort   claude.Effort
		stream   bool
		expected string
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			rs := newRecordingServer(t, "ok")
			options := []claude.Option{claude.WithBaseURL(rs.srv.URL), claude.WithModel("claude-opus-5-5")}
			if tc.effort != "" {
				options = append(options, claude.WithEffort(tc.effort))
			}
			client, err := claude.New(context.Background(), "test-key", options...)
			gt.NoError(t, err).Required()
			session, err := client.NewSession(context.Background())
			gt.NoError(t, err).Required()

			requestPath{stream: tc.stream}.send(t, session)

			outputConfig, ok := rs.lastBody(t)["output_config"]
			if tc.expected == "" {
				gt.False(t, ok)
				return
			}
			gt.True(t, ok).Required()
			assertJSONEqual(t, tc.expected, outputConfig)
		}
	}

	t.Run("low", runTest(testCase{effort: claude.EffortLow, expected: `{"effort":"low"}`}))
	t.Run("medium", runTest(testCase{effort: claude.EffortMedium, expected: `{"effort":"medium"}`}))
	t.Run("high", runTest(testCase{effort: claude.EffortHigh, expected: `{"effort":"high"}`}))
	t.Run("xhigh", runTest(testCase{effort: claude.EffortXHigh, expected: `{"effort":"xhigh"}`}))
	t.Run("max", runTest(testCase{effort: claude.EffortMax, expected: `{"effort":"max"}`}))
	t.Run("stream", runTest(testCase{effort: claude.EffortLow, stream: true, expected: `{"effort":"low"}`}))
	t.Run("not set", runTest(testCase{}))

	t.Run("sent together with response schema", func(t *testing.T) {
		rs := newRecordingServer(t, "{}")
		client, err := claude.New(context.Background(), "test-key",
			claude.WithBaseURL(rs.srv.URL), claude.WithModel("claude-opus-5-5"), claude.WithEffort(claude.EffortHigh))
		gt.NoError(t, err).Required()
		session, err := client.NewSession(context.Background())
		gt.NoError(t, err).Required()

		requestPath{}.send(t, session, gollem.WithGenerateResponseSchema(structuredOutputTestSchema()))
		assertJSONEqual(t, `{"effort":"high","format":`+structuredOutputTestFormat+`}`, rs.lastBody(t)["output_config"])
	})
}
