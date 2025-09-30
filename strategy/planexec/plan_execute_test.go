package planexec_test

import (
	"context"
	"os"
	"strings"
	"sync/atomic"
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

	t.Run("Plan with single task execution", func(t *testing.T) {
		ctx := context.Background()
		mockClient := createPlanExecutionMock()

		// Track task execution
		var planCreatedCalled int32
		var createdPlan *planexec.Plan

		hooks := planexec.PlanExecuteHooks{
			OnPlanCreated: func(ctx context.Context, plan *planexec.Plan) error {
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

		hooks := planexec.PlanExecuteHooks{
			OnPlanCreated: func(ctx context.Context, plan *planexec.Plan) error {
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
		hooks := planexec.PlanExecuteHooks{
			OnPlanCreated: func(ctx context.Context, plan *planexec.Plan) error {
				atomic.AddInt32(&planCreatedCalled, 1)
				createdPlan = plan
				return nil
			},
			OnPlanUpdated: func(ctx context.Context, plan *planexec.Plan) error {
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
							// Reflection with plan update
							return &gollem.Response{
								Texts: []string{`{
									"should_continue": true,
									"goal_achieved": false,
									"reason": "Need additional validation step",
									"plan_updates": {
										"new_tasks": ["Validate data"],
										"remove_task_ids": []
									}
								}`},
							}, nil
						case 4:
							// Execute second task
							return &gollem.Response{
								Texts: []string{"Data transformed"},
							}, nil
						case 5:
							// Reflection to continue
							return &gollem.Response{
								Texts: []string{`{"should_continue": true, "goal_achieved": false}`},
							}, nil
						case 6:
							// Execute validation task
							return &gollem.Response{
								Texts: []string{"Data validated"},
							}, nil
						default:
							// Complete due to max iterations (testing limit)
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

		// Create strategy with all options
		strategy := planexec.New(mockClient,
			planexec.WithHooks(hooks),
			planexec.WithMiddleware([]gollem.ContentBlockMiddleware{mockMiddleware}),
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
	// Helper function for testing with Agent.Execute
	testWithAgent := func(client gollem.LLMClient) func(t *testing.T) {
		return func(t *testing.T) {
			ctx := context.Background()

			// Test simple calculation (likely direct response)
			t.Run("SimpleCalculation", func(t *testing.T) {
				// Track hooks
				var planCreatedCalled int32
				var createdPlan *planexec.Plan

				hooks := planexec.PlanExecuteHooks{
					OnPlanCreated: func(ctx context.Context, plan *planexec.Plan) error {
						atomic.AddInt32(&planCreatedCalled, 1)
						createdPlan = plan
						return nil
					},
				}

				// Create strategy with hooks
				strategy := planexec.New(client, planexec.WithHooks(hooks))

				// Create agent with the strategy
				agent := gollem.New(client, gollem.WithStrategy(strategy))

				response, err := agent.Execute(ctx, gollem.Text("What is 2 + 2?"))
				gt.NoError(t, err)
				gt.V(t, response).NotNil()
				gt.True(t, len(response.Texts) > 0)

				// Verify the response contains "4"
				responseText := strings.Join(response.Texts, " ")
				gt.S(t, responseText).Contains("4")

				// Simple questions should create a plan (but with 0 tasks for direct response)
				gt.V(t, atomic.LoadInt32(&planCreatedCalled)).Equal(int32(1))
				gt.V(t, createdPlan).NotNil()
				gt.V(t, len(createdPlan.Tasks)).Equal(0) // Direct response should have no tasks
			})

			// Test task creation and execution with reflection
			t.Run("TaskExecutionWithReflection", func(t *testing.T) {
				// Track hooks and middleware
				var planCreatedCalled int32
				var createdPlan *planexec.Plan
				var sessionCreationCount int32
				var reflectionInputs []string

				hooks := planexec.PlanExecuteHooks{
					OnPlanCreated: func(ctx context.Context, plan *planexec.Plan) error {
						atomic.AddInt32(&planCreatedCalled, 1)
						createdPlan = plan
						return nil
					},
				}

				// Create middleware to track LLM calls and detect reflection
				mockMiddleware := func(next gollem.ContentBlockHandler) gollem.ContentBlockHandler {
					return func(ctx context.Context, req *gollem.ContentRequest) (*gollem.ContentResponse, error) {
						atomic.AddInt32(&sessionCreationCount, 1)

						// Check if this is a reflection call by looking for reflection-related keywords
						for _, input := range req.Inputs {
							if text, ok := input.(gollem.Text); ok {
								inputStr := strings.ToLower(string(text))
								if strings.Contains(inputStr, "goal_achieved") ||
								   strings.Contains(inputStr, "should_continue") ||
								   strings.Contains(inputStr, "completed tasks") {
									reflectionInputs = append(reflectionInputs, inputStr)
								}
							}
						}

						return next(ctx, req)
					}
				}

				// Create strategy with hooks and middleware
				strategy := planexec.New(client,
					planexec.WithHooks(hooks),
					planexec.WithMiddleware([]gollem.ContentBlockMiddleware{mockMiddleware}),
				)

				// Create agent with the strategy
				agent := gollem.New(client, gollem.WithStrategy(strategy))

				// Use a prompt that forces task creation
				response, err := agent.Execute(ctx,
					gollem.Text("Please follow these steps exactly: Step 1: Calculate 2+2. Step 2: Calculate 3+3. Step 3: Add the results from step 1 and step 2."))
				gt.NoError(t, err)
				gt.V(t, response).NotNil()
				gt.True(t, len(response.Texts) > 0)

				// Verify the response contains expected content
				responseText := strings.ToLower(strings.Join(response.Texts, " "))
				gt.S(t, responseText).ContainsAny("4", "6", "10", "four", "six", "ten")

				// Verify plan was created
				gt.V(t, atomic.LoadInt32(&planCreatedCalled)).Equal(int32(1))
				gt.V(t, createdPlan).NotNil()

				// Verify multiple sessions were created (minimum: plan + execution + reflection)
				// If tasks were created, we expect at least 3 sessions:
				// 1. Initial plan creation
				// 2. At least one task execution
				// 3. At least one reflection after task
				sessionCount := atomic.LoadInt32(&sessionCreationCount)
				gt.True(t, sessionCount >= 3)

				// Verify reflection was called (check if we captured reflection inputs)
				gt.True(t, len(reflectionInputs) > 0)
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
