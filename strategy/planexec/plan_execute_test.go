package planexec_test

import (
	"context"
	"os"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/claude"
	"github.com/m-mizutani/gollem/llm/gemini"
	"github.com/m-mizutani/gollem/llm/openai"
	"github.com/m-mizutani/gollem/mock"
	"github.com/m-mizutani/gollem/strategy/planexec"
	"github.com/m-mizutani/gt"
)

func TestBasicPlanExecution(t *testing.T) {
	// Helper function to create mock client for direct response
	createDirectResponseMock := func(response string) *mock.LLMClientMock {
		return &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						jsonResponse := `{
							"needs_plan": false,
							"direct_response": "` + response + `"
						}`
						return &gollem.Response{
							Texts: []string{jsonResponse},
						}, nil
					},
					HistoryFunc: func() (*gollem.History, error) {
						return &gollem.History{}, nil
					},
				}, nil
			},
			CountTokensFunc: func(ctx context.Context, history *gollem.History) (int, error) {
				return 100, nil
			},
			IsCompatibleHistoryFunc: func(ctx context.Context, history *gollem.History) error {
				return nil
			},
		}
	}

	// Helper function to create mock client for plan execution
	createPlanExecutionMock := func() *mock.LLMClientMock {
		callCount := 0
		return &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						callCount++
						switch callCount {
						case 1:
							// First call: return a plan
							return &gollem.Response{
								Texts: []string{`{
									"needs_plan": true,
									"goal": "Calculate 10 + 5",
									"tasks": [{"description": "Add 10 and 5", "state": "pending"}]
								}`},
							}, nil
						case 2:
							// Second call: task execution
							return &gollem.Response{
								Texts: []string{"The result is 15"},
							}, nil
						default:
							// Third call: reflection to complete
							return &gollem.Response{
								Texts: []string{`{"should_continue": false, "goal_achieved": true}`},
							}, nil
						}
					},
					HistoryFunc: func() (*gollem.History, error) {
						return &gollem.History{}, nil
					},
				}, nil
			},
			CountTokensFunc: func(ctx context.Context, history *gollem.History) (int, error) {
				return 100, nil
			},
			IsCompatibleHistoryFunc: func(ctx context.Context, history *gollem.History) error {
				return nil
			},
		}
	}

	t.Run("Direct response without plan using Agent", func(t *testing.T) {
		ctx := context.Background()
		mockClient := createDirectResponseMock("The answer is 4")

		// Create strategy with mock client
		strategy := planexec.NewPlanExecuteStrategy(
			planexec.WithLLMClient(mockClient),
		)

		// Create agent and test
		agent := gollem.New(mockClient, gollem.WithStrategy(strategy))
		resp, err := agent.Execute(ctx, gollem.Text("What is 2 + 2?"))
		gt.NoError(t, err)
		gt.V(t, resp).NotNil()
		gt.V(t, resp.Texts[0]).Equal("The answer is 4")
	})

	t.Run("Simple plan with single task using Agent", func(t *testing.T) {
		ctx := context.Background()
		mockClient := createPlanExecutionMock()

		// Create strategy with mock client
		strategy := planexec.NewPlanExecuteStrategy(
			planexec.WithLLMClient(mockClient),
		)

		// Create agent and test
		agent := gollem.New(mockClient, gollem.WithStrategy(strategy))
		resp, err := agent.Execute(ctx, gollem.Text("Calculate 10 + 5"))
		gt.NoError(t, err)
		gt.V(t, resp).NotNil()
		gt.True(t, len(resp.Texts) > 0)
	})
}

// Test with real LLM providers using Agent.Execute
func TestPlanExecuteWithLLMs(t *testing.T) {
	// Helper function for testing with Agent.Execute
	testWithAgent := func(client gollem.LLMClient) func(t *testing.T) {
		return func(t *testing.T) {
			ctx := context.Background()

			// Create strategy
			strategy := planexec.NewPlanExecuteStrategy(
				planexec.WithLLMClient(client),
			)

			// Create agent with the strategy
			agent := gollem.New(client, gollem.WithStrategy(strategy))

			// Test direct response
			t.Run("DirectResponse", func(t *testing.T) {
				response, err := agent.Execute(ctx, gollem.Text("What is 2 + 2?"))
				gt.NoError(t, err)
				gt.V(t, response).NotNil()
				gt.True(t, len(response.Texts) > 0)
			})

			// Test multiple task execution
			t.Run("MultipleTaskExecution", func(t *testing.T) {
				response, err := agent.Execute(ctx,
					gollem.Text("List three primary colors and explain why they are called primary"))
				gt.NoError(t, err)
				gt.V(t, response).NotNil()
				gt.True(t, len(response.Texts) > 0)
			})
		}
	}

	// Test with OpenAI
	t.Run("OpenAI", func(t *testing.T) {
		apiKey := os.Getenv("TEST_OPENAI_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		if apiKey == "" {
			t.Skip("TEST_OPENAI_API_KEY or OPENAI_API_KEY is not set")
		}

		client, err := openai.New(context.Background(), apiKey)
		gt.NoError(t, err)

		testWithAgent(client)(t)
	})

	// Test with Claude
	t.Run("Claude", func(t *testing.T) {
		apiKey := os.Getenv("TEST_CLAUDE_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		if apiKey == "" {
			t.Skip("TEST_CLAUDE_API_KEY or ANTHROPIC_API_KEY is not set")
		}

		client, err := claude.New(context.Background(), apiKey)
		gt.NoError(t, err)

		testWithAgent(client)(t)
	})

	// Test with Gemini
	t.Run("Gemini", func(t *testing.T) {
		projectID := os.Getenv("TEST_GCP_PROJECT_ID")
		if projectID == "" {
			projectID = os.Getenv("GEMINI_PROJECT_ID")
		}
		location := os.Getenv("TEST_GCP_LOCATION")
		if location == "" {
			location = os.Getenv("GEMINI_LOCATION")
		}

		if projectID == "" || location == "" {
			t.Skip("Required Gemini env vars not set")
		}

		client, err := gemini.New(context.Background(), projectID, location)
		gt.NoError(t, err)

		testWithAgent(client)(t)
	})
}
