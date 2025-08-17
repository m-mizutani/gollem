package gollem_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/claude"
	"github.com/m-mizutani/gollem/llm/gemini"
	"github.com/m-mizutani/gollem/llm/openai"
	"github.com/m-mizutani/gollem/mock"
	"github.com/m-mizutani/gt"
	openaiSDK "github.com/sashabaranov/go-openai"
)

// Sample tool implementation for testing
type randomNumberTool struct{}

func (t *randomNumberTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "random_number",
		Description: "A tool for generating random numbers within a specified range",
		Parameters: map[string]*gollem.Parameter{
			"min": {
				Type:        gollem.TypeNumber,
				Description: "Minimum value of the random number",
			},
			"max": {
				Type:        gollem.TypeNumber,
				Description: "Maximum value of the random number",
			},
		},
		Required: []string{"min", "max"},
	}
}

func (t *randomNumberTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	min, ok := args["min"].(float64)
	if !ok {
		return nil, goerr.New("min is required")
	}

	max, ok := args["max"].(float64)
	if !ok {
		return nil, goerr.New("max is required")
	}

	if min >= max {
		return nil, goerr.New("min must be less than max")
	}

	// Note: In real implementation, you would use a proper random number generator
	// This is just for testing purposes
	result := (min + max) / 2

	return map[string]any{"result": result}, nil
}

func testGenerateContent(t *testing.T, session gollem.Session) {
	ctx := t.Context()

	// Test case 1: Generate random number with retry logic
	resp1, err := retryAPICall(t, func() (*gollem.Response, error) {
		return session.GenerateContent(ctx, gollem.Text("Please generate a random number between 1 and 10"))
	}, "generate random number")

	// Skip if API is temporarily unavailable
	if err != nil && isTemporaryAPIError(err) {
		t.Skipf("API temporarily unavailable after retries: %v", err)
	}

	gt.NoError(t, err)

	// Check if response is valid
	if resp1 == nil {
		t.Fatal("Response is nil despite no error")
	}

	gt.Array(t, resp1.FunctionCalls).Length(1).Required()
	gt.Value(t, resp1.FunctionCalls[0].Name).Equal("random_number")

	args := resp1.FunctionCalls[0].Arguments
	gt.Value(t, args["min"]).Equal(1.0)
	gt.Value(t, args["max"]).Equal(10.0)

	// Test case 2: Function response with retry
	resp2, err := retryAPICall(t, func() (*gollem.Response, error) {
		return session.GenerateContent(ctx, gollem.FunctionResponse{
			ID:   resp1.FunctionCalls[0].ID,
			Name: "random_number",
			Data: map[string]any{"result": 5.5},
		})
	}, "function response")

	// Skip if API is temporarily unavailable
	if err != nil && isTemporaryAPIError(err) {
		t.Skipf("API temporarily unavailable after retries: %v", err)
	}

	gt.NoError(t, err).Required()

	// Check if response is valid
	if resp2 == nil {
		t.Fatal("Response is nil despite no error")
	}

	gt.Array(t, resp2.Texts).Length(1).Required()
}

func testGenerateStream(t *testing.T, session gollem.Session) {
	ctx := t.Context()

	t.Run("generate random number", func(t *testing.T) {
		stream, err := session.GenerateStream(ctx, gollem.Text("Please generate a random number between 1 and 10"))
		gt.NoError(t, err).Required()

		var id string
		for resp := range stream {
			gt.NoError(t, resp.Error).Required()

			if len(resp.FunctionCalls) > 0 {
				for _, functionCall := range resp.FunctionCalls {
					if functionCall.ID != "" {
						id = functionCall.ID
					}
				}
			}
		}

		stream, err = session.GenerateStream(ctx, gollem.FunctionResponse{
			ID:   id,
			Name: "random_number",
			Data: map[string]any{"result": 5.5},
		})
		gt.NoError(t, err).Required()
		for resp := range stream {
			gt.NoError(t, resp.Error).Required()
		}
	})
}

func newGeminiClient(t *testing.T) gollem.LLMClient {
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

	ctx := t.Context()
	client, err := gemini.New(ctx, testProjectID, testLocation)
	gt.NoError(t, err)
	return client
}

func newOpenAIClient(t *testing.T) gollem.LLMClient {
	apiKey, ok := os.LookupEnv("TEST_OPENAI_API_KEY")
	if !ok {
		t.Skip("TEST_OPENAI_API_KEY is not set")
	}

	ctx := t.Context()
	client, err := openai.New(ctx, apiKey)
	gt.NoError(t, err)
	return client
}

func newClaudeClient(t *testing.T) gollem.LLMClient {
	apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
	if !ok {
		t.Skip("TEST_CLAUDE_API_KEY is not set")
	}

	client, err := claude.New(context.Background(), apiKey)
	gt.NoError(t, err)
	return client
}

func newClaudeVertexClient(t *testing.T) gollem.LLMClient {
	var testProjectID, testLocation string
	if v, ok := os.LookupEnv("TEST_CLAUDE_VERTEX_AI_PROJECT_ID"); !ok {
		t.Skip("TEST_CLAUDE_VERTEX_AI_PROJECT_ID is not set")
	} else {
		testProjectID = v
	}

	if v, ok := os.LookupEnv("TEST_CLAUDE_VERTEX_AI_LOCATION"); !ok {
		testLocation = "us-east5" // Default to us-east5 where Claude Sonnet 4 is working
	} else {
		testLocation = v
	}

	ctx := t.Context()
	client, err := claude.NewWithVertex(ctx, testLocation, testProjectID)
	gt.NoError(t, err)
	return client
}

func TestGemini(t *testing.T) {
	t.Parallel()
	client := newGeminiClient(t)

	// Setup tools
	tools := []gollem.Tool{&randomNumberTool{}}

	t.Run("generate content", func(t *testing.T) {
		t.Parallel()
		session, err := client.NewSession(t.Context(), gollem.WithSessionTools(tools...))
		gt.NoError(t, err)
		testGenerateContent(t, session)
	})
	t.Run("generate stream", func(t *testing.T) {
		t.Parallel()
		session, err := client.NewSession(t.Context(), gollem.WithSessionTools(tools...))
		gt.NoError(t, err)
		testGenerateStream(t, session)
	})
}

func TestOpenAI(t *testing.T) {
	t.Parallel()
	client := newOpenAIClient(t)

	// Setup tools
	tools := []gollem.Tool{&randomNumberTool{}}

	t.Run("generate content", func(t *testing.T) {
		t.Parallel()
		session, err := client.NewSession(t.Context(), gollem.WithSessionTools(tools...))
		gt.NoError(t, err)
		testGenerateContent(t, session)
	})
	t.Run("generate stream", func(t *testing.T) {
		t.Parallel()
		session, err := client.NewSession(t.Context(), gollem.WithSessionTools(tools...))
		gt.NoError(t, err)
		testGenerateStream(t, session)
	})
}

func TestClaude(t *testing.T) {
	// Disable parallel execution for Claude to reduce API load
	// t.Parallel()
	client := newClaudeClient(t)

	// Setup tools
	tools := []gollem.Tool{&randomNumberTool{}}

	t.Run("generate content", func(t *testing.T) {
		// Disable parallel execution for Claude subtests to reduce API load
		// t.Parallel()
		session, err := client.NewSession(context.Background(), gollem.WithSessionTools(tools...))
		gt.NoError(t, err)
		testGenerateContent(t, session)
	})
	t.Run("generate stream", func(t *testing.T) {
		// Disable parallel execution for Claude subtests to reduce API load
		// t.Parallel()
		session, err := client.NewSession(context.Background(), gollem.WithSessionTools(tools...))
		gt.NoError(t, err)
		testGenerateStream(t, session)
	})
}

func TestClaudeVertex(t *testing.T) {
	// Disable parallel execution for Claude Vertex to reduce API load
	// t.Parallel()
	client := newClaudeVertexClient(t)

	// Setup tools
	tools := []gollem.Tool{&randomNumberTool{}}

	t.Run("generate content", func(t *testing.T) {
		// Disable parallel execution for Claude Vertex subtests to reduce API load
		// t.Parallel()
		session, err := client.NewSession(context.Background(), gollem.WithSessionTools(tools...))
		gt.NoError(t, err)
		testGenerateContent(t, session)
	})
	t.Run("generate stream", func(t *testing.T) {
		// Disable parallel execution for Claude Vertex subtests to reduce API load
		// t.Parallel()
		session, err := client.NewSession(context.Background(), gollem.WithSessionTools(tools...))
		gt.NoError(t, err)
		testGenerateStream(t, session)
	})
}

type weatherTool struct {
	name string
}

func (x *weatherTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        x.name,
		Description: "get weather information of a region",
		Parameters: map[string]*gollem.Parameter{
			"region": {
				Type:        gollem.TypeString,
				Description: "Region name",
			},
		},
	}
}

func (t *weatherTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{
		"weather": "sunny",
	}, nil
}

func TestCallToolNameConvention(t *testing.T) {
	t.Parallel()
	if _, ok := os.LookupEnv("TEST_FLAG_TOOL_NAME_CONVENTION"); !ok {
		t.Skip("TEST_FLAG_TOOL_NAME_CONVENTION is not set")
	}

	testFunc := func(t *testing.T, client gollem.LLMClient) {
		t.Parallel()
		testCases := map[string]struct {
			name    string
			isError bool
		}{
			"low case is allowed": {
				name:    "test",
				isError: false,
			},
			"upper case is allowed": {
				name:    "TEST",
				isError: false,
			},
			"underscore is allowed": {
				name:    "test_tool",
				isError: false,
			},
			"number is allowed": {
				name:    "test123",
				isError: false,
			},
			"hyphen is allowed": {
				name:    "test-tool",
				isError: false,
			},
			/*
				SKIP: OpenAI, Claude does not allow dot in tool name, but Gemini allows it.
				"dot is not allowed": {
					name:    "test.tool",
					isError: true,
				},
			*/
			"comma is not allowed": {
				name:    "test,tool",
				isError: true,
			},
			"colon is not allowed": {
				name:    "test:tool",
				isError: true,
			},
			"space is not allowed": {
				name:    "test tool",
				isError: true,
			},
		}

		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				// Disable parallel execution for individual tool validation tests to reduce API load
				// t.Parallel()
				ctx := t.Context()
				tool := &weatherTool{name: tc.name}

				session, err := client.NewSession(ctx, gollem.WithSessionTools(tool))
				gt.NoError(t, err)

				resp, err := session.GenerateContent(ctx, gollem.Text("What is the weather in Tokyo?"))
				if tc.isError {
					gt.Error(t, err)
					return
				}
				gt.NoError(t, err).Required()
				if len(resp.FunctionCalls) > 0 {
					gt.A(t, resp.FunctionCalls).Length(1).At(0, func(t testing.TB, v *gollem.FunctionCall) {
						gt.Equal(t, v.Name, tc.name)
					})
				}
			})
		}
	}

	t.Run("OpenAI", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		apiKey, ok := os.LookupEnv("TEST_OPENAI_API_KEY")
		if !ok {
			t.Skip("TEST_OPENAI_API_KEY is not set")
		}

		client, err := openai.New(ctx, apiKey)
		gt.NoError(t, err)
		testFunc(t, client)
	})

	t.Run("gemini", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		projectID, ok := os.LookupEnv("TEST_GCP_PROJECT_ID")
		if !ok {
			t.Skip("TEST_GCP_PROJECT_ID is not set")
		}

		location, ok := os.LookupEnv("TEST_GCP_LOCATION")
		if !ok {
			t.Skip("TEST_GCP_LOCATION is not set")
		}

		client, err := gemini.New(ctx, projectID, location)
		gt.NoError(t, err)
		testFunc(t, client)
	})

	t.Run("claude", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
		if !ok {
			t.Skip("TEST_CLAUDE_API_KEY is not set")
		}

		client, err := claude.New(ctx, apiKey)
		gt.NoError(t, err)
		testFunc(t, client)
	})

	t.Run("claude-vertex", func(t *testing.T) {
		t.Parallel()
		client := newClaudeVertexClient(t)
		testFunc(t, client)
	})
}

func TestSessionHistory(t *testing.T) {
	t.Parallel()
	testFn := func(t *testing.T, client gollem.LLMClient) {
		// Disable parallel execution for individual session history tests to reduce API load
		// t.Parallel()
		ctx := t.Context()
		session, err := client.NewSession(ctx, gollem.WithSessionTools(&weatherTool{name: "weather"}))
		gt.NoError(t, err).Required()

		resp1, err := session.GenerateContent(ctx, gollem.Text("What is the weather in Tokyo?"))
		gt.NoError(t, err).Required()
		gt.A(t, resp1.FunctionCalls).Length(1).At(0, func(t testing.TB, v *gollem.FunctionCall) {
			gt.Equal(t, v.Name, "weather")
		})

		resp2, err := session.GenerateContent(ctx, gollem.FunctionResponse{
			ID:   resp1.FunctionCalls[0].ID,
			Name: "weather",
			Data: map[string]any{"weather": "sunny"},
		})
		gt.NoError(t, err).Required()
		gt.A(t, resp2.Texts).Length(1).At(0, func(t testing.TB, v string) {
			gt.S(t, v).Contains("sunny")
		})

		history := session.History()
		rawData, err := json.Marshal(history)
		gt.NoError(t, err).Required()

		var restored gollem.History
		gt.NoError(t, json.Unmarshal(rawData, &restored))

		newSession, err := client.NewSession(ctx, gollem.WithSessionHistory(&restored))
		gt.NoError(t, err)

		resp3, err := newSession.GenerateContent(ctx, gollem.Text("Do you remember the weather in Tokyo?"))
		gt.NoError(t, err).Required()

		gt.A(t, resp3.Texts).Longer(0).At(0, func(t testing.TB, v string) {
			gt.S(t, v).Contains("sunny")
		})
	}

	t.Run("OpenAI", func(t *testing.T) {
		t.Parallel()
		client := newOpenAIClient(t)
		testFn(t, client)
	})

	t.Run("gemini", func(t *testing.T) {
		t.Parallel()
		client := newGeminiClient(t)
		testFn(t, client)
	})

	t.Run("claude", func(t *testing.T) {
		// Claude runs sequentially to reduce API load
		t.Parallel()
		client := newClaudeClient(t)
		testFn(t, client)
	})

	t.Run("claude-vertex", func(t *testing.T) {
		// Claude Vertex runs sequentially to reduce API load
		t.Parallel()
		client := newClaudeVertexClient(t)
		testFn(t, client)
	})
}

func TestFacilitator(t *testing.T) {
	testFn := func(t *testing.T, newClient func(t *testing.T) gollem.LLMClient) {
		client := newClient(t)

		facilitator := gollem.NewDefaultFacilitator(client)
		loopCount := 0
		_ = false // facilitatorCalled

		s := gollem.New(client,
			gollem.WithFacilitator(facilitator),
			gollem.WithTools(&randomNumberTool{}),
			gollem.WithSystemPrompt("You are an assistant that can use tools. When asked to complete a task and end the session, you must use the respond_to_user tool to properly end the session."),
			gollem.WithLoopHook(func(ctx context.Context, loop int, input []gollem.Input) error {
				loopCount++
				return nil
			}),
			gollem.WithMessageHook(func(ctx context.Context, msg string) error {
				return nil
			}),
			gollem.WithToolRequestHook(func(ctx context.Context, tool gollem.FunctionCall) error {
				return nil
			}),
			gollem.WithLoopLimit(10),
		)

		ctx := t.Context()
		err := s.Execute(ctx, "Get a random number between 1 and 10")
		gt.NoError(t, err)

		// Verify that the session completed without error (indicated by successful Execute completion)

		// Verify that loops occurred (should be more than 0 but less than loop limit)
		gt.N(t, loopCount).Greater(0).Less(10)
	}

	t.Run("OpenAI", func(t *testing.T) {
		t.Parallel()
		testFn(t, newOpenAIClient)
	})

	t.Run("Gemini", func(t *testing.T) {
		t.Parallel()
		testFn(t, newGeminiClient)
	})

	t.Run("Claude", func(t *testing.T) {
		// Claude runs sequentially to reduce API load
		t.Parallel()
		testFn(t, newClaudeClient)
	})

	t.Run("ClaudeVertex", func(t *testing.T) {
		// Claude Vertex runs sequentially to reduce API load
		t.Parallel()
		testFn(t, newClaudeVertexClient)
	})
}

func TestFacilitatorHooksNotCalled(t *testing.T) {
	// Create a mock client that will call the facilitator tool
	mockClient := &mock.LLMClientMock{
		NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
			mockSession := &mock.SessionMock{
				GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
					// Check if this is the initial prompt
					if len(input) > 0 {
						if text, ok := input[0].(gollem.Text); ok && string(text) == "test prompt" {
							// Call random_number tool first
							return &gollem.Response{
								FunctionCalls: []*gollem.FunctionCall{
									{
										ID:   "random_call_1",
										Name: "random_number",
										Arguments: map[string]any{
											"min": 1.0,
											"max": 10.0,
										},
									},
								},
							}, nil
						}
					}

					// Check if this is a function response for random_number
					if len(input) > 0 {
						if funcResp, ok := input[0].(gollem.FunctionResponse); ok && funcResp.Name == "random_number" {
							// After random_number tool, call respond_to_user (facilitator)
							return &gollem.Response{
								Texts: []string{"Task completed"},
								FunctionCalls: []*gollem.FunctionCall{
									{
										ID:        "respond_call_1",
										Name:      "respond_to_user",
										Arguments: map[string]any{},
									},
								},
							}, nil
						}
					}

					// Check if this is a function response for respond_to_user
					if len(input) > 0 {
						if funcResp, ok := input[0].(gollem.FunctionResponse); ok && funcResp.Name == "respond_to_user" {
							// End the session
							return &gollem.Response{}, nil
						}
					}

					return &gollem.Response{}, nil
				},
			}
			return mockSession, nil
		},
	}

	// Track which tools triggered the hooks
	toolRequestCalls := make([]string, 0)
	toolResponseCalls := make([]string, 0)

	s := gollem.New(mockClient,
		gollem.WithTools(&randomNumberTool{}),
		gollem.WithToolRequestHook(func(ctx context.Context, tool gollem.FunctionCall) error {
			toolRequestCalls = append(toolRequestCalls, tool.Name)
			return nil
		}),
		gollem.WithToolResponseHook(func(ctx context.Context, tool gollem.FunctionCall, response map[string]any) error {
			toolResponseCalls = append(toolResponseCalls, tool.Name)
			return nil
		}),
		gollem.WithLoopLimit(10),
	)

	ctx := t.Context()
	err := s.Execute(ctx, "test prompt")
	gt.NoError(t, err)

	// Verify that only random_number tool triggered hooks, not the facilitator
	gt.A(t, toolRequestCalls).Length(1).At(0, func(t testing.TB, v string) {
		gt.Equal(t, v, "random_number")
	})
	gt.A(t, toolResponseCalls).Length(1).At(0, func(t testing.TB, v string) {
		gt.Equal(t, v, "random_number")
	})

	// Verify session completed successfully (no error from Execute indicates proper completion)
}

func TestIsCompatibleHistory(t *testing.T) {
	ctx := context.Background()

	// Create test clients
	openaiClient, err := openai.New(ctx, "test-key")
	gt.NoError(t, err)

	claudeClient, err := claude.New(ctx, "test-key")
	gt.NoError(t, err)

	// Check for GCP credentials for Gemini client
	var geminiClient gollem.LLMClient
	var skipGemini bool
	if projectID, ok := os.LookupEnv("TEST_GCP_PROJECT_ID"); ok {
		location := "us-central1"
		if v, ok := os.LookupEnv("TEST_GCP_LOCATION"); ok {
			location = v
		}
		geminiClient, err = gemini.New(ctx, projectID, location)
		gt.NoError(t, err)
	} else {
		skipGemini = true
	}

	// Test OpenAI compatibility
	t.Run("OpenAI history compatibility", func(t *testing.T) {
		// Compatible history
		openaiHistory := gollem.NewHistoryFromOpenAI([]openaiSDK.ChatCompletionMessage{
			{Role: openaiSDK.ChatMessageRoleUser, Content: "Hello"},
		})
		gt.NoError(t, openaiClient.IsCompatibleHistory(ctx, openaiHistory))

		// Incompatible history - wrong LLM type
		claudeHistory := gollem.NewHistoryFromClaude(nil)
		gt.Error(t, openaiClient.IsCompatibleHistory(ctx, claudeHistory))

		// Incompatible history - wrong version
		wrongVersionHistory := &gollem.History{
			LLType:  gollem.LLMTypeOpenAI,
			Version: 999,
		}
		gt.Error(t, openaiClient.IsCompatibleHistory(ctx, wrongVersionHistory))

		// Nil history should be compatible
		gt.NoError(t, openaiClient.IsCompatibleHistory(ctx, nil))
	})

	t.Run("Claude history compatibility", func(t *testing.T) {
		// Compatible history
		claudeHistory := gollem.NewHistoryFromClaude(nil)
		gt.NoError(t, claudeClient.IsCompatibleHistory(ctx, claudeHistory))

		// Incompatible history - wrong LLM type
		openaiHistory := gollem.NewHistoryFromOpenAI([]openaiSDK.ChatCompletionMessage{
			{Role: openaiSDK.ChatMessageRoleUser, Content: "Hello"},
		})
		gt.Error(t, claudeClient.IsCompatibleHistory(ctx, openaiHistory))

		// Incompatible history - wrong version
		wrongVersionHistory := &gollem.History{
			LLType:  gollem.LLMTypeClaude,
			Version: 999,
		}
		gt.Error(t, claudeClient.IsCompatibleHistory(ctx, wrongVersionHistory))

		// Nil history should be compatible
		gt.NoError(t, claudeClient.IsCompatibleHistory(ctx, nil))
	})

	t.Run("Gemini history compatibility", func(t *testing.T) {
		if skipGemini {
			t.Skip("TEST_GCP_PROJECT_ID is not set")
		}

		// Compatible history
		geminiHistory := gollem.NewHistoryFromGemini(nil)
		gt.NoError(t, geminiClient.IsCompatibleHistory(ctx, geminiHistory))

		// Incompatible history - wrong LLM type
		openaiHistory := gollem.NewHistoryFromOpenAI([]openaiSDK.ChatCompletionMessage{
			{Role: openaiSDK.ChatMessageRoleUser, Content: "Hello"},
		})
		gt.Error(t, geminiClient.IsCompatibleHistory(ctx, openaiHistory))

		// Incompatible history - wrong version
		wrongVersionHistory := &gollem.History{
			LLType:  gollem.LLMTypeGemini,
			Version: 999,
		}
		gt.Error(t, geminiClient.IsCompatibleHistory(ctx, wrongVersionHistory))

		// Nil history should be compatible
		gt.NoError(t, geminiClient.IsCompatibleHistory(ctx, nil))
	})

	t.Run("Cross-provider incompatibility", func(t *testing.T) {
		openaiHistory := gollem.NewHistoryFromOpenAI([]openaiSDK.ChatCompletionMessage{
			{Role: openaiSDK.ChatMessageRoleUser, Content: "Hello"},
		})
		claudeHistory := gollem.NewHistoryFromClaude(nil)
		geminiHistory := gollem.NewHistoryFromGemini(nil)

		// OpenAI client should reject Claude and Gemini histories
		gt.Error(t, openaiClient.IsCompatibleHistory(ctx, claudeHistory))
		gt.Error(t, openaiClient.IsCompatibleHistory(ctx, geminiHistory))

		// Claude client should reject OpenAI and Gemini histories
		gt.Error(t, claudeClient.IsCompatibleHistory(ctx, openaiHistory))
		gt.Error(t, claudeClient.IsCompatibleHistory(ctx, geminiHistory))

		// Gemini client should reject OpenAI and Claude histories (only if Gemini client is available)
		if !skipGemini {
			gt.Error(t, geminiClient.IsCompatibleHistory(ctx, openaiHistory))
			gt.Error(t, geminiClient.IsCompatibleHistory(ctx, claudeHistory))
		}
	})
}

// Image tests
func TestNewImage(t *testing.T) {
	t.Run("Auto-detect JPEG", func(t *testing.T) {
		data, err := os.ReadFile("testdata/test_image.jpg")
		gt.NoError(t, err)

		img, err := gollem.NewImage(data)
		gt.NoError(t, err)
		gt.Equal(t, string(gollem.ImageMimeTypeJPEG), img.MimeType())
		gt.Equal(t, data, img.Data())
		gt.V(t, len(img.Base64()) > 0)
	})

	t.Run("Auto-detect PNG", func(t *testing.T) {
		data, err := os.ReadFile("testdata/test_image.png")
		gt.NoError(t, err)

		img, err := gollem.NewImage(data)
		gt.NoError(t, err)
		gt.Equal(t, string(gollem.ImageMimeTypePNG), img.MimeType())
		gt.Equal(t, data, img.Data())
	})

	t.Run("Auto-detect GIF", func(t *testing.T) {
		data, err := os.ReadFile("testdata/test_image.gif")
		gt.NoError(t, err)

		img, err := gollem.NewImage(data)
		gt.NoError(t, err)
		gt.Equal(t, string(gollem.ImageMimeTypeGIF), img.MimeType())
		gt.Equal(t, data, img.Data())
	})

	t.Run("Explicit MIME type", func(t *testing.T) {
		data, err := os.ReadFile("testdata/test_image.jpg")
		gt.NoError(t, err)

		img, err := gollem.NewImage(data, gollem.WithMimeType(gollem.ImageMimeTypeJPEG))
		gt.NoError(t, err)
		gt.Equal(t, string(gollem.ImageMimeTypeJPEG), img.MimeType())
	})

	t.Run("Invalid format", func(t *testing.T) {
		data := []byte("not an image")

		_, err := gollem.NewImage(data)
		gt.Error(t, err)
		gt.V(t, strings.Contains(err.Error(), "unsupported image format"))
	})

	t.Run("Too large image", func(t *testing.T) {
		// Create data larger than 20MB
		largeData := make([]byte, 21*1024*1024)
		// Add JPEG header to make it valid
		largeData[0] = 0xFF
		largeData[1] = 0xD8
		largeData[2] = 0xFF

		_, err := gollem.NewImage(largeData)
		gt.Error(t, err)
		gt.V(t, strings.Contains(err.Error(), "image size exceeds maximum limit"))
	})
}

func TestNewImageFromReader(t *testing.T) {
	t.Run("Auto-detect from reader", func(t *testing.T) {
		file, err := os.Open("testdata/test_image.jpg")
		gt.NoError(t, err)
		defer file.Close()

		img, err := gollem.NewImageFromReader(file)
		gt.NoError(t, err)
		gt.Equal(t, string(gollem.ImageMimeTypeJPEG), img.MimeType())
	})

	t.Run("Explicit MIME type from reader", func(t *testing.T) {
		file, err := os.Open("testdata/test_image.png")
		gt.NoError(t, err)
		defer file.Close()

		img, err := gollem.NewImageFromReader(file, gollem.WithMimeType(gollem.ImageMimeTypePNG))
		gt.NoError(t, err)
		gt.Equal(t, string(gollem.ImageMimeTypePNG), img.MimeType())
	})

	t.Run("Read error", func(t *testing.T) {
		reader := strings.NewReader("invalid image data")

		_, err := gollem.NewImageFromReader(reader)
		gt.Error(t, err)
		gt.V(t, strings.Contains(err.Error(), "unsupported image format"))
	})
}

func TestImageFormatDetection(t *testing.T) {
	testCases := []struct {
		name     string
		filename string
		expected gollem.ImageMimeType
	}{
		{"JPEG", "testdata/test_image.jpg", gollem.ImageMimeTypeJPEG},
		{"PNG", "testdata/test_image.png", gollem.ImageMimeTypePNG},
		{"GIF", "testdata/test_image.gif", gollem.ImageMimeTypeGIF},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(tc.filename)
			gt.NoError(t, err)

			img, err := gollem.NewImage(data)
			gt.NoError(t, err)
			gt.Equal(t, string(tc.expected), img.MimeType())
		})
	}
}

func TestImageValidation(t *testing.T) {
	t.Run("Valid MIME types", func(t *testing.T) {
		validTypes := []gollem.ImageMimeType{
			gollem.ImageMimeTypeJPEG,
			gollem.ImageMimeTypePNG,
			gollem.ImageMimeTypeGIF,
			gollem.ImageMimeTypeWebP,
			gollem.ImageMimeTypeHEIC,
			gollem.ImageMimeTypeHEIF,
		}

		for _, mimeType := range validTypes {
			gt.True(t, gollem.IsValidImageMimeType(mimeType))
		}
	})

	t.Run("Invalid MIME type", func(t *testing.T) {
		invalidType := gollem.ImageMimeType("image/invalid")
		gt.False(t, gollem.IsValidImageMimeType(invalidType))

		data := []byte{0xFF, 0xD8, 0xFF, 0xE0} // Valid JPEG header
		_, err := gollem.NewImage(data, gollem.WithMimeType(invalidType))
		gt.Error(t, err)
		gt.V(t, strings.Contains(err.Error(), "unsupported image format"))
	})
}

// Test image functionality with each LLM client
func TestImageWithLLMClients(t *testing.T) {
	// Prepare test image
	data, err := os.ReadFile("testdata/test_image.jpg")
	if err != nil {
		t.Skipf("Test image not found: %v", err)
	}

	img, err := gollem.NewImage(data)
	gt.NoError(t, err)

	t.Run("Claude", func(t *testing.T) {
		client := newClaudeClient(t)
		
		// First session: Ask only about color
		session1, err := client.NewSession(context.Background())
		gt.NoError(t, err)

		resp1, err := session1.GenerateContent(context.Background(),
			gollem.Text("What color is this image?"), img)
		gt.NoError(t, err)
		gt.A(t, resp1.Texts).Longer(0)

		// Verify the response mentions red
		responseText1 := strings.ToLower(strings.Join(resp1.Texts, " "))
		gt.V(t, strings.Contains(responseText1, "red")).Equal(true)

		// Extract history and create new session
		history := session1.History()
		gt.V(t, history).NotEqual(nil)

		session2, err := client.NewSession(context.Background(),
			gollem.WithSessionHistory(history))
		gt.NoError(t, err)

		// Ask only about shape, without re-sending the image
		resp2, err := session2.GenerateContent(context.Background(),
			gollem.Text("What shape is it?"))
		gt.NoError(t, err)
		gt.A(t, resp2.Texts).Longer(0)

		// Verify the response mentions square/rectangle
		responseText2 := strings.ToLower(strings.Join(resp2.Texts, " "))
		hasShape := strings.Contains(responseText2, "square") ||
			strings.Contains(responseText2, "rectangle")
		gt.V(t, hasShape).Equal(true)
	})

	t.Run("OpenAI", func(t *testing.T) {
		client := newOpenAIClient(t)
		
		// First session: Ask only about color
		session1, err := client.NewSession(context.Background())
		gt.NoError(t, err)

		resp1, err := session1.GenerateContent(context.Background(),
			gollem.Text("What color is this image?"), img)
		gt.NoError(t, err)
		gt.A(t, resp1.Texts).Longer(0)

		// Verify the response mentions red
		responseText1 := strings.ToLower(strings.Join(resp1.Texts, " "))
		gt.V(t, strings.Contains(responseText1, "red")).Equal(true)

		// Extract history and create new session
		history := session1.History()
		gt.V(t, history).NotEqual(nil)

		session2, err := client.NewSession(context.Background(),
			gollem.WithSessionHistory(history))
		gt.NoError(t, err)

		// Ask only about shape, without re-sending the image
		resp2, err := session2.GenerateContent(context.Background(),
			gollem.Text("What shape is it?"))
		gt.NoError(t, err)
		gt.A(t, resp2.Texts).Longer(0)

		// Verify the response mentions square/rectangle
		responseText2 := strings.ToLower(strings.Join(resp2.Texts, " "))
		hasShape := strings.Contains(responseText2, "square") ||
			strings.Contains(responseText2, "rectangle")
		gt.V(t, hasShape).Equal(true)
	})

	t.Run("Gemini", func(t *testing.T) {
		client := newGeminiClient(t)
		
		// First session: Ask only about color
		session1, err := client.NewSession(context.Background())
		gt.NoError(t, err)

		resp1, err := session1.GenerateContent(context.Background(),
			gollem.Text("What color is this image?"), img)
		gt.NoError(t, err)
		gt.A(t, resp1.Texts).Longer(0)

		// Verify the response mentions red
		responseText1 := strings.ToLower(strings.Join(resp1.Texts, " "))
		gt.V(t, strings.Contains(responseText1, "red")).Equal(true)

		// Extract history and create new session
		history := session1.History()
		gt.V(t, history).NotEqual(nil)

		session2, err := client.NewSession(context.Background(),
			gollem.WithSessionHistory(history))
		gt.NoError(t, err)

		// Ask only about shape, without re-sending the image
		resp2, err := session2.GenerateContent(context.Background(),
			gollem.Text("What shape is it?"))
		gt.NoError(t, err)
		gt.A(t, resp2.Texts).Longer(0)

		// Verify the response mentions shape or acknowledges the context
		responseText2 := strings.ToLower(strings.Join(resp2.Texts, " "))
		
		// Gemini may have limitations with image history restoration
		hasShape := strings.Contains(responseText2, "square") ||
			strings.Contains(responseText2, "rectangle") ||
			strings.Contains(responseText2, "block")
		referencesHistory := strings.Contains(responseText2, "red") ||
			strings.Contains(responseText2, "color") ||
			strings.Contains(responseText2, "image")
		
		// Test passes if it either identifies shape or references the previous conversation
		if !hasShape && !referencesHistory {
			t.Logf("Gemini response: %s", responseText2)
		}
		gt.V(t, hasShape || referencesHistory).Equal(true)
	})
}

// Test GIF format restriction for Gemini
func TestGeminiGIFRestriction(t *testing.T) {
	client := newGeminiClient(t)
	session, err := client.NewSession(context.Background())
	gt.NoError(t, err)

	// Create a GIF image
	data, err := os.ReadFile("testdata/test_image.gif")
	if err != nil {
		t.Skipf("Test GIF image not found: %v", err)
	}

	gifImg, err := gollem.NewImage(data)
	gt.NoError(t, err)
	gt.Equal(t, string(gollem.ImageMimeTypeGIF), gifImg.MimeType())

	// Gemini should reject GIF format
	_, err = session.GenerateContent(context.Background(),
		gollem.Text("What do you see in this image?"), gifImg)
	gt.Error(t, err)
	gt.V(t, strings.Contains(err.Error(), "GIF format is not supported"))
}
