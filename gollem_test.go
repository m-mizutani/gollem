package gollem_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"log/slog"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/claude"
	"github.com/m-mizutani/gollem/llm/gemini"
	"github.com/m-mizutani/gollem/llm/openai"
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
	t.Parallel()
	respModes := []gollem.ResponseMode{
		gollem.ResponseModeBlocking,
		gollem.ResponseModeStreaming,
	}

	testFn := func(t *testing.T, newClient func(t *testing.T) (gollem.LLMClient, error)) {
		for _, respMode := range respModes {
			t.Run(fmt.Sprintf("ResponseMode=%s", respMode), func(t *testing.T) {
				// Disable parallel execution for individual response modes to reduce API load
				// t.Parallel()
				client, err := newClient(t)
				gt.NoError(t, err)

				randomNumberToolCalled := false
				s := gollem.New(client,
					gollem.WithTools(&RandomNumberTool{}),
					gollem.WithToolRequestHook(func(ctx context.Context, tool gollem.FunctionCall) error {
						if tool.Name == "random_number" {
							randomNumberToolCalled = true
						}
						// Allow both random_number and respond_to_user tool calls
						gt.True(t, tool.Name == "random_number" || tool.Name == "respond_to_user")
						return nil
					}),
					gollem.WithResponseMode(respMode),
				)

				// Execute with retry logic for API errors
				maxRetries := 3
				for i := 0; i < maxRetries; i++ {
					err = s.Execute(t.Context(), "Generate a random number between 1 and 100.")
					if err == nil {
						break
					}

					// Check if it's a temporary API error
					if strings.Contains(err.Error(), "overloaded") || strings.Contains(err.Error(), "rate limit") || strings.Contains(err.Error(), "529") {
						t.Logf("API error (attempt %d/%d): %v", i+1, maxRetries, err)
						time.Sleep(time.Duration(i+1) * time.Second)
						continue
					}

					// If it's not a temporary error, break
					break
				}

				// Skip if API is temporarily unavailable
				if err != nil && (strings.Contains(err.Error(), "overloaded") || strings.Contains(err.Error(), "rate limit") || strings.Contains(err.Error(), "529")) {
					t.Skipf("API temporarily unavailable after %d retries: %v", maxRetries, err)
				}

				gt.NoError(t, err)
				gt.True(t, randomNumberToolCalled)
			})
		}
	}

	t.Run("OpenAI", func(t *testing.T) {
		t.Parallel()
		apiKey, ok := os.LookupEnv("TEST_OPENAI_API_KEY")
		if !ok {
			t.Skip("TEST_OPENAI_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return openai.New(context.Background(), apiKey)
		})
	})

	t.Run("Claude", func(t *testing.T) {
		// Disable parallel execution for Claude to reduce API load
		// t.Parallel()
		apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
		if !ok {
			t.Skip("TEST_CLAUDE_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return claude.New(context.Background(), apiKey)
		})
	})

	t.Run("Gemini", func(t *testing.T) {
		t.Parallel()
		projectID, ok := os.LookupEnv("TEST_GCP_PROJECT_ID")
		if !ok {
			t.Skip("TEST_GCP_PROJECT_ID is not set")
		}
		location, ok := os.LookupEnv("TEST_GCP_LOCATION")
		if !ok {
			t.Skip("TEST_GCP_LOCATION is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return gemini.New(context.Background(), projectID, location)
		})
	})
}

func TestGollemWithHooks(t *testing.T) {
	t.Parallel()
	mockClient := &mock.LLMClientMock{
		NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
			mockSession := &mock.SessionMock{
				GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
					// Check if the input is a function response
					if len(input) > 0 {
						if funcResp, ok := input[0].(gollem.FunctionResponse); ok {
							// If it's a respond_to_user tool response, end the session
							if funcResp.Name == "respond_to_user" {
								return &gollem.Response{}, nil
							}
							// For other tool responses, call respond_to_user to end the session
							return &gollem.Response{
								Texts: []string{"Task completed."},
								FunctionCalls: []*gollem.FunctionCall{
									{
										Name:      "respond_to_user",
										Arguments: map[string]any{},
									},
								},
							}, nil
						}
					}

					// Check if input is DefaultFacilitator's proceed prompt
					if len(input) > 0 {
						if text, ok := input[0].(gollem.Text); ok {
							if strings.Contains(string(text), "What is the next action needed") {
								// Return respond_to_user tool call to end session
								return &gollem.Response{
									Texts: []string{"I will complete the task now."},
									FunctionCalls: []*gollem.FunctionCall{
										{
											Name:      "respond_to_user",
											Arguments: map[string]any{},
										},
									},
								}, nil
							}
						}
					}

					// Return response with function call
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
				},
			}
			return mockSession, nil
		},
	}

	t.Run("ToolRequestHook", func(t *testing.T) {
		t.Parallel()
		toolRequestCalled := false
		s := gollem.New(mockClient,
			gollem.WithTools(&RandomNumberTool{}),
			gollem.WithToolRequestHook(func(ctx context.Context, tool gollem.FunctionCall) error {
				if tool.Name == "test_tool" {
					toolRequestCalled = true
					gt.Equal(t, tool.Name, "test_tool")
					gt.Equal(t, tool.Arguments["arg1"], "value1")
				}
				return nil
			}),
			gollem.WithLoopLimit(5),
		)

		err := s.Execute(t.Context(), "test message")
		gt.NoError(t, err)
		gt.True(t, toolRequestCalled)
	})

	t.Run("ToolResponseHook", func(t *testing.T) {
		t.Parallel()
		// Create a tool that returns a test result
		testTool := &mockTool{
			spec: gollem.ToolSpec{
				Name:        "test_tool",
				Description: "A test tool",
			},
			run: func(ctx context.Context, args map[string]any) (map[string]any, error) {
				return map[string]any{
					"result": "test_result",
				}, nil
			},
		}

		toolResponseCalled := false
		s := gollem.New(mockClient,
			gollem.WithTools(testTool),
			gollem.WithToolResponseHook(func(ctx context.Context, tool gollem.FunctionCall, response map[string]any) error {
				if tool.Name == "test_tool" {
					toolResponseCalled = true
					gt.Equal(t, tool.Name, "test_tool")
					gt.Equal(t, response["result"], "test_result")
				}
				return nil
			}),
			gollem.WithLoopLimit(5),
		)

		err := s.Execute(t.Context(), "test message")
		gt.NoError(t, err)
		gt.True(t, toolResponseCalled)
	})

	t.Run("ToolErrorHook", func(t *testing.T) {
		t.Parallel()
		// Create a tool that always returns an error
		errorTool := &mockTool{
			spec: gollem.ToolSpec{
				Name:        "test_tool", // Match the name in the mock response
				Description: "A tool that always returns an error",
			},
			run: func(ctx context.Context, args map[string]any) (map[string]any, error) {
				return nil, fmt.Errorf("test error")
			},
		}

		toolErrorCalled := false
		s := gollem.New(mockClient,
			gollem.WithTools(errorTool),
			gollem.WithToolErrorHook(func(ctx context.Context, err error, tool gollem.FunctionCall) error {
				if tool.Name == "test_tool" {
					toolErrorCalled = true
					gt.Equal(t, err.Error(), "test error")
					gt.Equal(t, tool.Name, "test_tool")
				}
				return nil
			}),
			gollem.WithLoopLimit(5),
		)

		err := s.Execute(t.Context(), "test message")
		gt.NoError(t, err)
		gt.True(t, toolErrorCalled)
	})

	t.Run("MessageHook", func(t *testing.T) {
		t.Parallel()
		messageHookCalled := false
		s := gollem.New(mockClient,
			gollem.WithMessageHook(func(ctx context.Context, msg string) error {
				if msg == "test response" {
					messageHookCalled = true
					gt.Equal(t, msg, "test response")
				}
				return nil
			}),
			gollem.WithLoopLimit(5),
		)

		err := s.Execute(t.Context(), "test message")
		gt.NoError(t, err)
		gt.True(t, messageHookCalled)
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

					// If the custom function returns only text responses (no function calls),
					// we need to call respond_to_user to end the session properly with DefaultFacilitator
					if len(response.FunctionCalls) == 0 {
						// Check if this is already a proceed prompt from DefaultFacilitator
						if len(input) > 0 {
							if text, ok := input[0].(gollem.Text); ok {
								if strings.Contains(string(text), "Choose your next action or complete") {
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

						// Check if this is a function response, if so end the session
						if len(input) > 0 {
							if funcResp, ok := input[0].(gollem.FunctionResponse); ok {
								if funcResp.Name == "respond_to_user" {
									return &gollem.Response{}, nil
								}
							}
						}
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
			// Check if input is DefaultFacilitator's proceed prompt and return JSON response
			if len(input) > 0 {
				if text, ok := input[0].(gollem.Text); ok {
					if strings.Contains(string(text), "Choose your next action or complete") {
						return &gollem.Response{
							Texts: []string{`{"action": "complete", "reason": "Task completed successfully", "completion": "All tasks finished"}`},
						}, nil
					}
				}
			}
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
		err := s.Execute(t.Context(), "test message")
		gt.Error(t, err)
		gt.True(t, errors.Is(err, gollem.ErrLoopLimitExceeded))
		gt.Equal(t, loopCount, 10)
	})

	t.Run("WithRetryLimit", func(t *testing.T) {
		retryCount := 0
		mockClient := newMockClient(func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
			// If input is a function response with error, continue the loop
			if len(input) > 0 {
				if resp, ok := input[0].(gollem.FunctionResponse); ok {
					if resp.Error != nil {
						// Check if it's DefaultFacilitator's proceed prompt and return JSON response
						if len(input) > 1 {
							if text, ok := input[1].(gollem.Text); ok {
								if strings.Contains(string(text), "Choose your next action or complete") {
									return &gollem.Response{
										Texts: []string{`{"action": "complete", "reason": "Task completed successfully", "completion": "All tasks finished"}`},
									}, nil
								}
							}
						}
						return &gollem.Response{
							Texts: []string{"retrying..."},
							FunctionCalls: []*gollem.FunctionCall{
								{
									Name: "test_tool",
									Arguments: map[string]any{
										"arg1": "value1",
									},
								},
							},
						}, nil
					}
					return &gollem.Response{
						Texts: []string{"success"},
					}, nil
				}
			}

			// Check if input is DefaultFacilitator's proceed prompt and return JSON response
			if len(input) > 0 {
				if text, ok := input[0].(gollem.Text); ok {
					if strings.Contains(string(text), "Choose your next action or complete") {
						return &gollem.Response{
							Texts: []string{`{"action": "complete", "reason": "Task completed successfully", "completion": "All tasks finished"}`},
						}, nil
					}
				}
			}

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
				retryCount++
				if retryCount <= 2 {
					return nil, fmt.Errorf("test error")
				}
				return map[string]any{"result": "test"}, nil
			},
		}

		s := gollem.New(mockClient, gollem.WithRetryLimit(5), gollem.WithLoopLimit(5), gollem.WithTools(tool))
		err := s.Execute(t.Context(), "test message")
		gt.NoError(t, err)
		gt.Equal(t, retryCount, 3)
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
		err := s.Execute(t.Context(), "test message")
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
		err := s.Execute(t.Context(), "test message")
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
		err := s.Execute(t.Context(), "test message")
		gt.NoError(t, err)
	})

	t.Run("WithResponseMode", func(t *testing.T) {
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				// Check session options to determine if this is for streaming or DefaultFacilitator
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

		receivedMessages := []string{}
		s := gollem.New(mockClient,
			gollem.WithResponseMode(gollem.ResponseModeStreaming),
			gollem.WithMessageHook(func(ctx context.Context, msg string) error {
				receivedMessages = append(receivedMessages, msg)
				return nil
			}),
		)
		err := s.Execute(t.Context(), "test message")
		gt.NoError(t, err)
		// Should receive only the 3 test responses from streaming (Facilitator's JSON response doesn't go through MessageHook)
		gt.Equal(t, len(receivedMessages), 3)
		gt.Equal(t, receivedMessages[0], "test response 1")
		gt.Equal(t, receivedMessages[1], "test response 2")
		gt.Equal(t, receivedMessages[2], "test response 3")
	})

	t.Run("WithLogger", func(t *testing.T) {
		var logOutput strings.Builder
		logger := slog.New(slog.NewTextHandler(&logOutput, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))

		mockClient := newMockClient(func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
			// Check if input is DefaultFacilitator's proceed prompt and return JSON response
			if len(input) > 0 {
				if text, ok := input[0].(gollem.Text); ok {
					if strings.Contains(string(text), "Choose your next action or complete") {
						return &gollem.Response{
							Texts: []string{`{"action": "complete", "reason": "Task completed successfully", "completion": "All tasks finished"}`},
						}, nil
					}
				}
			}
			return &gollem.Response{
				Texts: []string{"test response"},
			}, nil
		})

		s := gollem.New(mockClient, gollem.WithLogger(logger))
		err := s.Execute(t.Context(), "test message")
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
			gollem.WithRetryLimit(5),
			gollem.WithSystemPrompt("system prompt"),
			gollem.WithTools(tool),
			gollem.WithResponseMode(gollem.ResponseModeBlocking),
			gollem.WithLogger(logger),
		)
		err := s.Execute(t.Context(), "test message")
		gt.NoError(t, err)
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

// TestIsTokenLimitError_RealLLMs tests if isTokenLimitError correctly detects token limit errors from real LLMs
func TestIsTokenLimitError_RealLLMs(t *testing.T) {
	// Skip if TEST_TOKEN_LIMIT_ERROR is not set
	if os.Getenv("TEST_TOKEN_LIMIT_ERROR") == "" {
		t.Skip("TEST_TOKEN_LIMIT_ERROR is not set")
	}

	// Helper function to generate a huge message
	generateHugeMessage := func(targetTokens int) string {
		// ~4 characters per token for English text
		targetChars := targetTokens * 4
		pattern := "This is a test message designed to exceed token limits. It contains various words and sentences to simulate real content. "

		var sb strings.Builder
		sb.Grow(targetChars)

		repeats := targetChars / len(pattern)
		if repeats < 1 {
			repeats = 1
		}

		for i := 0; i < repeats; i++ {
			sb.WriteString(pattern)
			// Add variation to avoid compression
			sb.WriteString(strings.Repeat("x", i%10))
			sb.WriteString(" ")
		}

		return sb.String()
	}

	t.Run("OpenAI", func(t *testing.T) {
		apiKey := os.Getenv("TEST_OPENAI_API_KEY")
		if apiKey == "" {
			t.Skip("TEST_OPENAI_API_KEY is not set")
		}

		ctx := context.Background()

		// Create OpenAI client with a model that has smaller context window
		client, err := openai.New(ctx, apiKey, openai.WithModel("gpt-3.5-turbo"))
		gt.NoError(t, err)

		// Create agent without compression
		agent := gollem.New(client)

		// Send a message that exceeds the token limit
		// GPT-3.5-turbo has 16k context window
		// Try with increasing sizes to ensure we hit the limit
		tokenSizes := []int{20000, 50000, 100000, 200000}
		var execErr error

		for _, size := range tokenSizes {
			hugeMessage := generateHugeMessage(size)
			execErr = agent.Execute(ctx, hugeMessage)
			if execErr != nil {
				break
			}
		}

		// Ensure we got an error
		if execErr == nil {
			t.Fatal("Expected token limit error but got none even with very large messages")
		}

		// Log the error for debugging

		// Verify that isTokenLimitError correctly identifies the error
		isTokenError := gollem.IsTokenLimitError(execErr)

		// isTokenLimitError should return true for token limit errors
		gt.True(t, isTokenError)
	})

	t.Run("Claude", func(t *testing.T) {
		apiKey := os.Getenv("TEST_CLAUDE_API_KEY")
		if apiKey == "" {
			t.Skip("TEST_CLAUDE_API_KEY is not set")
		}

		ctx := context.Background()

		// Create Claude client
		client, err := claude.New(ctx, apiKey, claude.WithModel("claude-3-haiku-20240307"))
		gt.NoError(t, err)

		// Create agent without compression
		agent := gollem.New(client)

		// Send a message that exceeds the token limit
		// Claude 3 Haiku has 200k context window
		// Try with increasing sizes to ensure we hit the limit
		tokenSizes := []int{250000, 500000, 1000000}
		var execErr error

		for _, size := range tokenSizes {
			hugeMessage := generateHugeMessage(size)
			execErr = agent.Execute(ctx, hugeMessage)
			if execErr != nil {
				break
			}
		}

		// Ensure we got an error
		if execErr == nil {
			t.Fatal("Expected token limit error but got none even with very large messages")
		}

		// Log the error for debugging

		// Verify that isTokenLimitError correctly identifies the error
		isTokenError := gollem.IsTokenLimitError(execErr)

		// isTokenLimitError should return true for token limit errors
		gt.True(t, isTokenError)
	})

	t.Run("Gemini", func(t *testing.T) {
		projectID := os.Getenv("TEST_GCP_PROJECT_ID")
		location := os.Getenv("TEST_GCP_LOCATION")
		if projectID == "" || location == "" {
			t.Skip("TEST_GCP_PROJECT_ID or TEST_GCP_LOCATION is not set")
		}

		ctx := context.Background()

		// Create Gemini client with 2.0-flash or newer
		client, err := gemini.New(ctx, projectID, location, gemini.WithModel("gemini-2.0-flash"))
		if err != nil {
			t.Error("Failed to create Gemini client")
		}

		// Create agent without compression
		agent := gollem.New(client)

		// Send a message that might exceed the token limit
		// Gemini 1.5 Flash has 1M token context window, so we start smaller
		hugeMessage := generateHugeMessage(50000) // ~50k tokens

		err = agent.Execute(ctx, hugeMessage)
		if err == nil {
			// If 50k didn't trigger an error, try larger
			hugeMessage = generateHugeMessage(1500000) // 1.5M tokens
			err = agent.Execute(ctx, hugeMessage)
		}

		if err != nil {
			// Log the error for debugging

			// Verify that isTokenLimitError correctly identifies the error
			isTokenError := gollem.IsTokenLimitError(err)

			// isTokenLimitError should return true for token limit errors
			gt.True(t, isTokenError)
		} else {
			t.Skip("Gemini did not return a token limit error with test message sizes")
		}
	})
}
