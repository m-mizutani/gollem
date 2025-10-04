package planexec_test

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/claude"
	"github.com/m-mizutani/gollem/llm/gemini"
	"github.com/m-mizutani/gollem/llm/openai"
	"github.com/m-mizutani/gollem/mock"
	"github.com/m-mizutani/gollem/strategy/planexec"
	"github.com/m-mizutani/gt"
)

// testHooks is a test implementation of PlanExecuteHooks
type testHooks struct {
	onCreated func(ctx context.Context, plan *planexec.Plan) error
	onUpdated func(ctx context.Context, plan *planexec.Plan) error
}

func (h *testHooks) OnCreated(ctx context.Context, plan *planexec.Plan) error {
	if h.onCreated != nil {
		return h.onCreated(ctx, plan)
	}
	return nil
}

func (h *testHooks) OnUpdated(ctx context.Context, plan *planexec.Plan) error {
	if h.onUpdated != nil {
		return h.onUpdated(ctx, plan)
	}
	return nil
}

// testTool is a simple implementation of gollem.Tool for testing
type testTool struct {
	name        string
	description string
	parameters  map[string]*gollem.Parameter
	required    []string
	runFunc     func(ctx context.Context, args map[string]any) (map[string]any, error)
}

func (t *testTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        t.name,
		Description: t.description,
		Parameters:  t.parameters,
		Required:    t.required,
	}
}

func (t *testTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return t.runFunc(ctx, args)
}

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

	t.Run("Plan with single task execution", func(t *testing.T) {
		ctx := context.Background()
		mockClient := createPlanExecutionMock()

		// Track task execution
		var planCreatedCalled int32
		var createdPlan *planexec.Plan

		hooks := &testHooks{
			onCreated: func(ctx context.Context, plan *planexec.Plan) error {
				atomic.AddInt32(&planCreatedCalled, 1)
				createdPlan = plan
				return nil
			},
		}

		// Create strategy with hooks
		strategy := planexec.New(mockClient, planexec.WithHooks(hooks))

		// Create agent and test
		agent := gollem.New(mockClient, gollem.WithStrategy(strategy))
		resp, err := agent.Execute(ctx, gollem.Text("Calculate 10 + 5"))
		gt.NoError(t, err)
		gt.V(t, resp).NotNil()
		gt.True(t, len(resp.Texts) > 0)

		// Verify plan was created with tasks
		gt.V(t, atomic.LoadInt32(&planCreatedCalled)).Equal(int32(1))
		gt.V(t, createdPlan).NotNil()
		gt.V(t, len(createdPlan.Tasks)).Equal(1)
		gt.S(t, createdPlan.Tasks[0].Description).Contains("Add 10 and 5")
	})

	t.Run("Direct response without tasks", func(t *testing.T) {
		ctx := context.Background()
		mockClient := createDirectResponseMock("The answer is 4")

		// Track plan creation
		var planCreatedCalled int32
		var createdPlan *planexec.Plan

		hooks := &testHooks{
			onCreated: func(ctx context.Context, plan *planexec.Plan) error {
				atomic.AddInt32(&planCreatedCalled, 1)
				createdPlan = plan
				return nil
			},
		}

		// Create strategy with hooks
		strategy := planexec.New(mockClient, planexec.WithHooks(hooks))

		// Create agent and test
		agent := gollem.New(mockClient, gollem.WithStrategy(strategy))
		resp, err := agent.Execute(ctx, gollem.Text("What is 2 + 2?"))
		gt.NoError(t, err)
		gt.V(t, resp).NotNil()
		gt.V(t, resp.Texts[0]).Equal("The answer is 4")

		// Verify plan was created but with no tasks (direct response)
		gt.V(t, atomic.LoadInt32(&planCreatedCalled)).Equal(int32(1))
		gt.V(t, createdPlan).NotNil()
		gt.V(t, len(createdPlan.Tasks)).Equal(0)
		gt.V(t, createdPlan.DirectResponse).Equal("The answer is 4")
	})

	t.Run("Comprehensive test with Hooks and Middleware", func(t *testing.T) {
		ctx := context.Background()

		// Track hook calls
		var planCreatedCalled int32
		var planUpdatedCalled int32
		var createdPlan *planexec.Plan
		var updatedPlan *planexec.Plan

		// Track middleware calls
		var middlewareApplied int32

		// Create hooks
		hooks := &testHooks{
			onCreated: func(ctx context.Context, plan *planexec.Plan) error {
				atomic.AddInt32(&planCreatedCalled, 1)
				createdPlan = plan
				return nil
			},
			onUpdated: func(ctx context.Context, plan *planexec.Plan) error {
				atomic.AddInt32(&planUpdatedCalled, 1)
				updatedPlan = plan
				return nil
			},
		}

		// Create middleware
		mockMiddleware := func(next gollem.ContentBlockHandler) gollem.ContentBlockHandler {
			return func(ctx context.Context, req *gollem.ContentRequest) (*gollem.ContentResponse, error) {
				return next(ctx, req)
			}
		}

		// Create mock client that tests plan updates
		callCount := 0
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				// Apply options to check middleware is passed
				cfg := &gollem.SessionConfig{}
				for _, opt := range options {
					opt(cfg)
				}

				// Check if middleware was applied to the session
				if len(cfg.ContentBlockMiddlewares()) > 0 {
					atomic.AddInt32(&middlewareApplied, 1)
				}

				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						callCount++
						switch callCount {
						case 1:
							// First call: return a plan with 2 tasks
							return &gollem.Response{
								Texts: []string{`{
									"needs_plan": true,
									"goal": "Process data",
									"tasks": [
										{"description": "Load data"},
										{"description": "Transform data"}
									]
								}`},
							}, nil
						case 2:
							// Execute first task
							return &gollem.Response{
								Texts: []string{"Data loaded successfully"},
							}, nil
						case 3:
							// Reflection with plan update - add new task
							return &gollem.Response{
								Texts: []string{`{
									"new_tasks": ["Validate data"],
									"updated_tasks": [],
									"reason": "Need additional validation step"
								}`},
							}, nil
						case 4:
							// Execute second task
							return &gollem.Response{
								Texts: []string{"Data transformed"},
							}, nil
						case 5:
							// Reflection - no updates needed
							return &gollem.Response{
								Texts: []string{`{
									"new_tasks": [],
									"updated_tasks": [],
									"reason": "All tasks on track"
								}`},
							}, nil
						case 6:
							// Execute validation task
							return &gollem.Response{
								Texts: []string{"Data validated"},
							}, nil
						case 7:
							// Final reflection - no more tasks
							return &gollem.Response{
								Texts: []string{`{
									"new_tasks": [],
									"updated_tasks": [],
									"reason": "All tasks completed"
								}`},
							}, nil
						default:
							// Final conclusion
							return &gollem.Response{
								Texts: []string{"All data processing tasks completed successfully"},
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

		// Create strategy with all options
		strategy := planexec.New(mockClient,
			planexec.WithHooks(hooks),
			planexec.WithMiddleware(mockMiddleware),
		)

		// Create agent and test
		agent := gollem.New(mockClient, gollem.WithStrategy(strategy))
		resp, err := agent.Execute(ctx, gollem.Text("Process data"))
		gt.NoError(t, err)
		gt.V(t, resp).NotNil()

		// Verify all hooks were called
		gt.V(t, atomic.LoadInt32(&planCreatedCalled)).Equal(int32(1))
		gt.V(t, atomic.LoadInt32(&planUpdatedCalled)).Equal(int32(1)) // Should be called when new task is added
		gt.V(t, createdPlan).NotNil()
		gt.V(t, createdPlan.Goal).Equal("Process data")
		gt.V(t, updatedPlan).NotNil()
		gt.V(t, len(updatedPlan.Tasks)).Equal(3) // Original 2 tasks + 1 new task

		// Verify middleware was applied
		gt.True(t, atomic.LoadInt32(&middlewareApplied) > 0) // Should be applied at least once
	})
}

// Test with real LLM providers using Agent.Execute
func TestPlanExecuteWithLLMs(t *testing.T) {
	// Create test tools for the agent to use
	createTestTools := func() []gollem.Tool {
		// Tool to get weather information
		getWeather := &testTool{
			name:        "get_weather",
			description: "Get current weather for a city",
			parameters: map[string]*gollem.Parameter{
				"city": {
					Type:        gollem.TypeString,
					Description: "City name",
				},
			},
			required: []string{"city"},
			runFunc: func(ctx context.Context, args map[string]any) (map[string]any, error) {
				city, ok := args["city"].(string)
				if !ok || city == "" {
					return nil, goerr.New("missing or invalid 'city' parameter")
				}
				return map[string]any{
					"city":        city,
					"temperature": 22,
					"condition":   "sunny",
				}, nil
			},
		}

		// Tool to calculate distance
		calculateDistance := &testTool{
			name:        "calculate_distance",
			description: "Calculate distance between two cities in km",
			parameters: map[string]*gollem.Parameter{
				"from": {
					Type:        gollem.TypeString,
					Description: "Starting city",
				},
				"to": {
					Type:        gollem.TypeString,
					Description: "Destination city",
				},
			},
			required: []string{"from", "to"},
			runFunc: func(ctx context.Context, args map[string]any) (map[string]any, error) {
				from, ok := args["from"].(string)
				if !ok || from == "" {
					return nil, goerr.New("missing or invalid 'from' parameter")
				}
				to, ok := args["to"].(string)
				if !ok || to == "" {
					return nil, goerr.New("missing or invalid 'to' parameter")
				}
				// Mock distance calculation
				distance := len(from) + len(to)*10 // Simple mock calculation
				return map[string]any{
					"from":     from,
					"to":       to,
					"distance": distance,
				}, nil
			},
		}

		// Tool to search database
		searchDB := &testTool{
			name:        "search_database",
			description: "Search information in database",
			parameters: map[string]*gollem.Parameter{
				"query": {
					Type:        gollem.TypeString,
					Description: "Search query",
				},
			},
			required: []string{"query"},
			runFunc: func(ctx context.Context, args map[string]any) (map[string]any, error) {
				query, ok := args["query"].(string)
				if !ok || query == "" {
					return nil, goerr.New("missing or invalid 'query' parameter")
				}
				return map[string]any{
					"query":   query,
					"results": []string{"result1", "result2", "result3"},
					"count":   3,
				}, nil
			},
		}

		return []gollem.Tool{getWeather, calculateDistance, searchDB}
	}

	// Helper function for testing with Agent.Execute
	testWithAgent := func(client gollem.LLMClient, _ string) func(t *testing.T) {
		return func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
			ctx := ctxlog.With(context.Background(), logger)

			// Test with multiple tool calls
			var toolCallCount int32
			var toolNames []string

			// Track plan creation and updates
			var planCreatedCalled int32
			var createdPlan *planexec.Plan
			var taskCount int

			hooks := &testHooks{
				onCreated: func(ctx context.Context, plan *planexec.Plan) error {
					atomic.AddInt32(&planCreatedCalled, 1)
					createdPlan = plan
					taskCount = len(plan.Tasks)
					return nil
				},
			}

			// Create middleware to track tool calls
			toolTracker := func(next gollem.ContentBlockHandler) gollem.ContentBlockHandler {
				return func(ctx context.Context, req *gollem.ContentRequest) (*gollem.ContentResponse, error) {
					resp, err := next(ctx, req)
					if err != nil {
						return resp, err
					}

					// Track tool calls in response
					if resp != nil && resp.FunctionCalls != nil {
						for _, call := range resp.FunctionCalls {
							atomic.AddInt32(&toolCallCount, 1)
							toolNames = append(toolNames, call.Name)
						}
					}

					return resp, nil
				}
			}

			// Create strategy with hooks and middleware
			strategy := planexec.New(client,
				planexec.WithHooks(hooks),
				planexec.WithMiddleware(toolTracker),
			)

			// Create test tools
			tools := createTestTools()

			// Create agent with the strategy and tools
			agent := gollem.New(client,
				gollem.WithStrategy(strategy),
				gollem.WithTools(tools...),
				gollem.WithContentBlockMiddleware(toolTracker),
			)

			// Execute task that requires multiple tool calls
			response, err := agent.Execute(ctx,
				gollem.Text("Get the weather for Tokyo and calculate the distance from Tokyo to Osaka"))
			gt.NoError(t, err)
			gt.V(t, response).NotNil()
			if response == nil {
				return // Skip if API failed
			}
			gt.True(t, len(response.Texts) > 0)

			// Verify plan was created
			gt.V(t, atomic.LoadInt32(&planCreatedCalled)).Equal(int32(1))
			gt.V(t, createdPlan).NotNil()

			// Verify tasks were created (should have at least 1 task for multi-step operation)
			gt.True(t, taskCount > 0)

			// Verify tools were called (should be called at least 2 times for the 2 tools)
			finalToolCallCount := atomic.LoadInt32(&toolCallCount)
			gt.True(t, finalToolCallCount >= 2)

			// Verify expected tools were called
			toolNameStr := strings.Join(toolNames, ",")
			gt.S(t, toolNameStr).Contains("get_weather")
			gt.S(t, toolNameStr).Contains("calculate_distance")

			// Verify response contains results from tools
			responseText := strings.ToLower(strings.Join(response.Texts, " "))
			gt.S(t, responseText).ContainsAny("tokyo", "weather", "distance", "osaka")
		}
	}

	// Test with OpenAI
	t.Run("OpenAI", func(t *testing.T) {
		t.Parallel()
		apiKey := os.Getenv("TEST_OPENAI_API_KEY")
		if apiKey == "" {
			t.Skip("TEST_OPENAI_API_KEY or OPENAI_API_KEY is not set")
		}

		client, err := openai.New(context.Background(), apiKey)
		gt.NoError(t, err)

		testWithAgent(client, "OpenAI")(t)
	})

	// Test with Claude
	t.Run("Claude", func(t *testing.T) {
		t.Parallel()
		apiKey := os.Getenv("TEST_CLAUDE_API_KEY")
		if apiKey == "" {
			t.Skip("TEST_CLAUDE_API_KEY or ANTHROPIC_API_KEY is not set")
		}

		client, err := claude.New(context.Background(), apiKey)
		gt.NoError(t, err)

		testWithAgent(client, "Claude")(t)
	})

	// Test with Gemini
	t.Run("Gemini", func(t *testing.T) {
		t.Parallel()
		projectID := os.Getenv("TEST_GCP_PROJECT_ID")
		location := os.Getenv("TEST_GCP_LOCATION")

		if projectID == "" || location == "" {
			t.Skip("Required Gemini env vars not set")
		}

		client, err := gemini.New(context.Background(), projectID, location)
		gt.NoError(t, err)

		testWithAgent(client, "Gemini")(t)
	})
}
