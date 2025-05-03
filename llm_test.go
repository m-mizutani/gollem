package gollem_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/claude"
	"github.com/m-mizutani/gollem/llm/gemini"
	"github.com/m-mizutani/gollem/llm/openai"
	"github.com/m-mizutani/gt"
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
	params, ok := args["params"].(map[string]any)
	if !ok {
		return nil, goerr.New("params is required")
	}

	min, ok := params["min"].(float64)
	if !ok {
		return nil, goerr.New("min is required")
	}

	max, ok := params["max"].(float64)
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

	// Test case 1: Generate random number
	resp1, err := session.GenerateContent(ctx, gollem.Text("Please generate a random number between 1 and 10"))
	gt.NoError(t, err)
	gt.Array(t, resp1.FunctionCalls).Length(1).Required()
	gt.Value(t, resp1.FunctionCalls[0].Name).Equal("random_number")

	args := resp1.FunctionCalls[0].Arguments
	gt.Value(t, args["min"]).Equal(1.0)
	gt.Value(t, args["max"]).Equal(10.0)

	resp2, err := session.GenerateContent(ctx, gollem.FunctionResponse{
		ID:   resp1.FunctionCalls[0].ID,
		Name: "random_number",
		Data: map[string]any{"result": 5.5},
	})
	gt.NoError(t, err).Required()
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

func TestGemini(t *testing.T) {
	client := newGeminiClient(t)

	// Setup tools
	tools := []gollem.Tool{&randomNumberTool{}}
	session, err := client.NewSession(t.Context(), gollem.WithSessionTools(tools...))
	gt.NoError(t, err)

	t.Run("generate content", func(t *testing.T) {
		testGenerateContent(t, session)
	})
	t.Run("generate stream", func(t *testing.T) {
		testGenerateStream(t, session)
	})
}

func TestOpenAI(t *testing.T) {
	client := newOpenAIClient(t)

	// Setup tools
	tools := []gollem.Tool{&randomNumberTool{}}
	session, err := client.NewSession(t.Context(), gollem.WithSessionTools(tools...))
	gt.NoError(t, err)

	t.Run("generate content", func(t *testing.T) {
		testGenerateContent(t, session)
	})
	t.Run("generate stream", func(t *testing.T) {
		testGenerateStream(t, session)
	})
}

func TestClaude(t *testing.T) {
	client := newClaudeClient(t)

	session, err := client.NewSession(context.Background(), gollem.WithSessionTools(&randomNumberTool{}))
	gt.NoError(t, err)

	t.Run("generate content", func(t *testing.T) {
		testGenerateContent(t, session)
	})
	t.Run("generate stream", func(t *testing.T) {
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
	if _, ok := os.LookupEnv("TEST_FLAG_TOOL_NAME_CONVENTION"); !ok {
		t.Skip("TEST_FLAG_TOOL_NAME_CONVENTION is not set")
	}

	testFunc := func(t *testing.T, client gollem.LLMClient) {
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
		ctx := t.Context()
		apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
		if !ok {
			t.Skip("TEST_CLAUDE_API_KEY is not set")
		}

		client, err := claude.New(ctx, apiKey)
		gt.NoError(t, err)
		testFunc(t, client)
	})
}

func TestSessionHistory(t *testing.T) {
	testFn := func(t *testing.T, client gollem.LLMClient) {
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
		client := newOpenAIClient(t)
		testFn(t, client)
	})

	t.Run("gemini", func(t *testing.T) {
		client := newGeminiClient(t)
		testFn(t, client)
	})

	t.Run("claude", func(t *testing.T) {
		client := newClaudeClient(t)
		testFn(t, client)
	})
}
