package gollem_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"strings"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/mock"
	"github.com/m-mizutani/gt"
)

// RandomNumberTool is a tool that generates a random number within a specified range
type RandomNumberTool struct{}

func (t *RandomNumberTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "random_number",
		Description: "Generates a random number within a specified range",
		Parameters: map[string]*gollem.Parameter{
			"min": {
				Type:        gollem.TypeNumber,
				Description: "Minimum value of the range",
			},
			"max": {
				Type:        gollem.TypeNumber,
				Description: "Maximum value of the range",
			},
		},
		Required: []string{"min", "max"},
	}
}

func (t *RandomNumberTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	min := int(args["min"].(float64))
	max := int(args["max"].(float64))

	if min >= max {
		return nil, fmt.Errorf("min must be less than max")
	}

	randomNum := rand.Intn(max-min) + min
	return map[string]any{
		"number": randomNum,
	}, nil
}

func TestGollemWithTool(t *testing.T) {
	t.Run("tool execution", func(t *testing.T) {
		callCount := 0
		toolCalled := false

		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				mockSession := &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						callCount++

						// First call: return tool call
						if callCount == 1 {
							return &gollem.Response{
								Texts: []string{"I'll generate a random number for you."},
								FunctionCalls: []*gollem.FunctionCall{
									{
										ID:   "call_random_1",
										Name: "random_number",
										Arguments: map[string]any{
											"min": float64(1),
											"max": float64(100),
										},
									},
								},
							}, nil
						}

						// Second call: handle tool response
						if callCount == 2 {
							// Verify we received a function response
							if len(input) > 0 {
								if funcResp, ok := input[0].(gollem.FunctionResponse); ok {
									gt.Equal(t, "call_random_1", funcResp.ID)
									gt.Equal(t, "random_number", funcResp.Name)
									// Verify the response contains a number
									if result, ok := funcResp.Data["number"]; ok {
										var num int
										switch v := result.(type) {
										case int:
											num = v
										case float64:
											num = int(v)
										}
										if num >= 1 && num <= 100 {
											toolCalled = true
										}
									}
								}
							}
							// End the conversation
							return &gollem.Response{
								Texts: []string{"Tool execution completed"},
							}, nil
						}

						return &gollem.Response{
							Texts: []string{"unexpected call"},
						}, nil
					},
				}
				return mockSession, nil
			},
		}

		tool := &RandomNumberTool{}
		s := gollem.New(mockClient,
			gollem.WithTools(tool),
			gollem.WithLoopLimit(5),
		)

		err := s.Execute(t.Context(), gollem.Text("Generate a random number between 1 and 100."))
		gt.NoError(t, err)
		gt.True(t, toolCalled)
		gt.Equal(t, 2, callCount)
	})

	t.Run("tool middleware", func(t *testing.T) {
		middlewareCalled := false
		var capturedToolCalls []*gollem.FunctionCall

		// Tool middleware that captures tool calls
		toolMiddleware := func(next gollem.ToolHandler) gollem.ToolHandler {
			return func(ctx context.Context, req *gollem.ToolExecRequest) (*gollem.ToolExecResponse, error) {
				middlewareCalled = true
				capturedToolCalls = append(capturedToolCalls, req.Tool)

				// Call the next handler
				resp, err := next(ctx, req)
				if err != nil {
					return nil, err
				}

				// Modify the response
				if resp.Result != nil {
					if result, ok := resp.Result["number"]; ok {
						resp.Result["middleware_processed"] = true
						resp.Result["original_number"] = result
					}
				}

				return resp, nil
			}
		}

		callCount := 0
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				mockSession := &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						callCount++
						// First call: return tool call
						if callCount == 1 {
							return &gollem.Response{
								FunctionCalls: []*gollem.FunctionCall{
									{
										ID:   "test_call",
										Name: "random_number",
										Arguments: map[string]any{
											"min": float64(1),
											"max": float64(10),
										},
									},
								},
							}, nil
						}
						// Second call: end the conversation
						return &gollem.Response{
							Texts: []string{"Done"},
						}, nil
					},
				}
				return mockSession, nil
			},
		}

		tool := &RandomNumberTool{}
		s := gollem.New(mockClient,
			gollem.WithTools(tool),
			gollem.WithToolMiddleware(toolMiddleware),
			gollem.WithLoopLimit(5),
		)

		err := s.Execute(t.Context(), gollem.Text("test"))
		gt.NoError(t, err)
		gt.True(t, middlewareCalled)
		gt.Equal(t, 1, len(capturedToolCalls))
		gt.Equal(t, "random_number", capturedToolCalls[0].Name)
	})
}

// mockTool is a mock implementation of gollem.Tool
type mockTool struct {
	spec gollem.ToolSpec
	run  func(ctx context.Context, args map[string]any) (map[string]any, error)
}

func (t *mockTool) Spec() gollem.ToolSpec {
	return t.spec
}

func (t *mockTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return t.run(ctx, args)
}

// newMockClient creates a new LLMClientMock with the given GenerateContentFunc
func newMockClient(generateContentFunc func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error)) *mock.LLMClientMock {
	return &mock.LLMClientMock{
		NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
			mockSession := &mock.SessionMock{
				GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
					response, err := generateContentFunc(ctx, input...)
					if err != nil {
						return nil, err
					}
					return response, nil
				},
			}
			return mockSession, nil
		},
	}
}

func TestGollemWithOptions(t *testing.T) {
	t.Run("WithLoopLimit", func(t *testing.T) {
		loopCount := 0
		mockClient := newMockClient(func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
			loopCount++
			return &gollem.Response{
				Texts: []string{"test response"},
				FunctionCalls: []*gollem.FunctionCall{
					{
						Name: "test_tool",
						Arguments: map[string]any{
							"arg1": "value1",
						},
					},
				},
			}, nil
		})

		tool := &mock.ToolMock{
			SpecFunc: func() gollem.ToolSpec {
				return gollem.ToolSpec{
					Name:        "test_tool",
					Description: "A test tool",
				}
			},
			RunFunc: func(ctx context.Context, args map[string]any) (map[string]any, error) {
				return map[string]any{"result": "test"}, nil
			},
		}

		s := gollem.New(mockClient, gollem.WithLoopLimit(10), gollem.WithTools(tool))
		err := s.Execute(t.Context(), gollem.Text("test message"))
		gt.Error(t, err)
		gt.True(t, errors.Is(err, gollem.ErrLoopLimitExceeded))
		gt.Equal(t, loopCount, 10)
	})

	t.Run("WithSystemPrompt", func(t *testing.T) {
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				cfg := gollem.NewSessionConfig(options...)
				gt.Equal(t, cfg.SystemPrompt(), "system prompt")
				mockSession := &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						// Return response based on input
						if len(input) > 0 {
							if text, ok := input[0].(gollem.Text); ok {
								if strings.Contains(string(text), "test message") {
									// Return response with tool call
									return &gollem.Response{
										FunctionCalls: []*gollem.FunctionCall{
											{
												ID:        "test_call_1",
												Name:      "test_tool",
												Arguments: map[string]any{},
											},
										},
									}, nil
								}
							}
						}

						// Handle function responses
						if len(input) > 0 {
							if _, ok := input[0].(gollem.FunctionResponse); ok {
								// Return response with no tool calls to end the loop
								return &gollem.Response{
									Texts: []string{"Task completed"},
								}, nil
							}
						}

						return &gollem.Response{
							Texts: []string{"test response"},
							FunctionCalls: []*gollem.FunctionCall{
								{
									Name:      "respond_to_user",
									Arguments: map[string]any{},
								},
							},
						}, nil
					},
				}
				return mockSession, nil
			},
		}

		s := gollem.New(mockClient, gollem.WithSystemPrompt("system prompt"))
		err := s.Execute(t.Context(), gollem.Text("test message"))
		gt.NoError(t, err)
	})

	t.Run("WithTools", func(t *testing.T) {
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				cfg := gollem.NewSessionConfig(options...)
				tools := cfg.Tools()
				// Should have test_tool only
				gt.Equal(t, len(tools), 1)
				toolNames := make(map[string]bool)
				for _, tool := range tools {
					toolNames[tool.Spec().Name] = true
				}
				gt.True(t, toolNames["test_tool"])

				mockSession := &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						// Return response based on input
						if len(input) > 0 {
							if text, ok := input[0].(gollem.Text); ok {
								if strings.Contains(string(text), "test message") {
									// Return response with tool call
									return &gollem.Response{
										FunctionCalls: []*gollem.FunctionCall{
											{
												ID:        "test_call_1",
												Name:      "test_tool",
												Arguments: map[string]any{},
											},
										},
									}, nil
								}
							}
						}

						// Handle function responses
						if len(input) > 0 {
							if _, ok := input[0].(gollem.FunctionResponse); ok {
								// Return response with no tool calls to end the loop
								return &gollem.Response{
									Texts: []string{"Task completed"},
								}, nil
							}
						}

						return &gollem.Response{
							Texts: []string{"test response"},
							FunctionCalls: []*gollem.FunctionCall{
								{
									Name:      "respond_to_user",
									Arguments: map[string]any{},
								},
							},
						}, nil
					},
				}
				return mockSession, nil
			},
		}

		tool := &mockTool{
			spec: gollem.ToolSpec{
				Name:        "test_tool",
				Description: "A test tool",
			},
			run: func(ctx context.Context, args map[string]any) (map[string]any, error) {
				return map[string]any{"result": "test"}, nil
			},
		}
		s := gollem.New(mockClient, gollem.WithTools(tool), gollem.WithLoopLimit(5))
		err := s.Execute(t.Context(), gollem.Text("test message"))
		gt.NoError(t, err)
	})

	t.Run("WithToolSets", func(t *testing.T) {
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				cfg := gollem.NewSessionConfig(options...)
				tools := cfg.Tools()
				// Should have test_tool from ToolSet only
				gt.Equal(t, len(tools), 1)

				mockSession := &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						// Return response based on input
						if len(input) > 0 {
							if text, ok := input[0].(gollem.Text); ok {
								if strings.Contains(string(text), "test message") {
									// Return response with tool call
									return &gollem.Response{
										FunctionCalls: []*gollem.FunctionCall{
											{
												ID:        "test_call_1",
												Name:      "test_tool",
												Arguments: map[string]any{},
											},
										},
									}, nil
								}
							}
						}

						// Handle function responses
						if len(input) > 0 {
							if _, ok := input[0].(gollem.FunctionResponse); ok {
								// Return response with no tool calls to end the loop
								return &gollem.Response{
									Texts: []string{"Task completed"},
								}, nil
							}
						}

						return &gollem.Response{
							Texts: []string{"test response"},
							FunctionCalls: []*gollem.FunctionCall{
								{
									Name:      "respond_to_user",
									Arguments: map[string]any{},
								},
							},
						}, nil
					},
				}
				return mockSession, nil
			},
		}

		toolSet := &mockToolSet{
			specs: []gollem.ToolSpec{
				{
					Name:        "test_tool",
					Description: "A test tool",
				},
			},
			run: func(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
				return map[string]any{"result": "test"}, nil
			},
		}
		s := gollem.New(mockClient, gollem.WithToolSets(toolSet), gollem.WithLoopLimit(5))
		err := s.Execute(t.Context(), gollem.Text("test message"))
		gt.NoError(t, err)
	})

	t.Run("WithResponseMode", func(t *testing.T) {
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				// Check session options to determine if this is for streaming
				cfg := gollem.NewSessionConfig(options...)
				isStreamingSession := cfg.SystemPrompt() == "" // Main session has no specific system prompt for streaming

				mockSession := &mock.SessionMock{
					GenerateStreamFunc: func(ctx context.Context, input ...gollem.Input) (<-chan *gollem.Response, error) {
						ch := make(chan *gollem.Response)
						go func() {
							defer close(ch)
							// Only handle streaming for the main session
							if isStreamingSession {
								ch <- &gollem.Response{
									Texts: []string{"test response 1"},
								}
								ch <- &gollem.Response{
									Texts: []string{"test response 2"},
								}
								ch <- &gollem.Response{
									Texts: []string{"test response 3"},
								}
							}
						}()
						return ch, nil
					},
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						// Return response based on input
						if len(input) > 0 {
							if text, ok := input[0].(gollem.Text); ok {
								if strings.Contains(string(text), "test message") {
									// Return response with tool call
									return &gollem.Response{
										FunctionCalls: []*gollem.FunctionCall{
											{
												ID:        "test_call_1",
												Name:      "test_tool",
												Arguments: map[string]any{},
											},
										},
									}, nil
								}
							}
						}
						// Handle function responses
						if len(input) > 0 {
							if _, ok := input[0].(gollem.FunctionResponse); ok {
								// Return response with no tool calls to end the loop
								return &gollem.Response{
									Texts: []string{"Task completed"},
								}, nil
							}
						}
						return &gollem.Response{}, nil
					},
				}
				return mockSession, nil
			},
		}

		s := gollem.New(mockClient,
			gollem.WithResponseMode(gollem.ResponseModeStreaming),
		)
		err := s.Execute(t.Context(), gollem.Text("test message"))
		gt.NoError(t, err)
		// Test completes successfully with streaming mode
	})

	t.Run("WithLogger", func(t *testing.T) {
		var logOutput strings.Builder
		logger := slog.New(slog.NewTextHandler(&logOutput, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))

		mockClient := newMockClient(func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
			return &gollem.Response{
				Texts: []string{"test response"},
			}, nil
		})

		s := gollem.New(mockClient, gollem.WithLogger(logger), gollem.WithLoopLimit(5))
		err := s.Execute(t.Context(), gollem.Text("test message"))
		gt.NoError(t, err)

		logContent := logOutput.String()
		gt.True(t, len(logContent) > 0)
	})

	t.Run("CombineOptions", func(t *testing.T) {
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				cfg := gollem.NewSessionConfig(options...)
				gt.Equal(t, cfg.SystemPrompt(), "system prompt")
				// Should have test_tool only
				gt.Equal(t, len(cfg.Tools()), 1)
				toolNames := make(map[string]bool)
				for _, tool := range cfg.Tools() {
					toolNames[tool.Spec().Name] = true
				}
				gt.True(t, toolNames["test_tool"])

				mockSession := &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						// Return response based on input
						if len(input) > 0 {
							if text, ok := input[0].(gollem.Text); ok {
								if strings.Contains(string(text), "test message") {
									// Return response with tool call
									return &gollem.Response{
										FunctionCalls: []*gollem.FunctionCall{
											{
												ID:        "test_call_1",
												Name:      "test_tool",
												Arguments: map[string]any{},
											},
										},
									}, nil
								}
							}
						}

						// Handle function responses
						if len(input) > 0 {
							if _, ok := input[0].(gollem.FunctionResponse); ok {
								// Return response with no tool calls to end the loop
								return &gollem.Response{
									Texts: []string{"Task completed"},
								}, nil
							}
						}

						return &gollem.Response{
							Texts: []string{"test response"},
							FunctionCalls: []*gollem.FunctionCall{
								{
									Name:      "respond_to_user",
									Arguments: map[string]any{},
								},
							},
						}, nil
					},
				}
				return mockSession, nil
			},
		}

		tool := &mockTool{
			spec: gollem.ToolSpec{
				Name:        "test_tool",
				Description: "A test tool",
			},
			run: func(ctx context.Context, args map[string]any) (map[string]any, error) {
				return map[string]any{"result": "test"}, nil
			},
		}
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))

		s := gollem.New(mockClient,
			gollem.WithLoopLimit(5),
			gollem.WithSystemPrompt("system prompt"),
			gollem.WithTools(tool),
			gollem.WithResponseMode(gollem.ResponseModeBlocking),
			gollem.WithLogger(logger),
		)
		err := s.Execute(t.Context(), gollem.Text("test message"))
		gt.NoError(t, err)
	})

	t.Run("WithToolMiddleware", func(t *testing.T) {
		t.Parallel()

		// Create tool middleware that tracks execution
		middlewareExecuted := false
		testMiddleware := func(next gollem.ToolHandler) gollem.ToolHandler {
			return func(ctx context.Context, req *gollem.ToolExecRequest) (*gollem.ToolExecResponse, error) {
				middlewareExecuted = true
				return next(ctx, req)
			}
		}

		// Create agent with tool middleware
		agent := gollem.New(nil,
			gollem.WithToolMiddleware(testMiddleware),
		)

		gt.NotNil(t, agent)
		// Middleware configuration verification - agent accepts tool middleware
		_ = middlewareExecuted // Reserved for future execution testing
	})
}

// mockToolSet is a mock implementation of gollem.ToolSet
type mockToolSet struct {
	specs []gollem.ToolSpec
	run   func(ctx context.Context, name string, args map[string]any) (map[string]any, error)
}

func (t *mockToolSet) Specs(ctx context.Context) ([]gollem.ToolSpec, error) {
	return t.specs, nil
}

func (t *mockToolSet) Run(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
	return t.run(ctx, name, args)
}
