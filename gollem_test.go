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
	respModes := []gollem.ResponseMode{
		gollem.ResponseModeBlocking,
		gollem.ResponseModeStreaming,
	}

	testFn := func(t *testing.T, newClient func(t *testing.T) (gollem.LLMClient, error)) {
		for _, respMode := range respModes {
			t.Run(fmt.Sprintf("ResponseMode=%s", respMode), func(t *testing.T) {
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

				err = s.Execute(t.Context(), "Generate a random number between 1 and 100.")
				gt.NoError(t, err)
				gt.True(t, randomNumberToolCalled)
			})
		}
	}

	t.Run("OpenAI", func(t *testing.T) {
		apiKey, ok := os.LookupEnv("TEST_OPENAI_API_KEY")
		if !ok {
			t.Skip("TEST_OPENAI_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return openai.New(context.Background(), apiKey)
		})
	})

	t.Run("Claude", func(t *testing.T) {
		apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
		if !ok {
			t.Skip("TEST_CLAUDE_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return claude.New(context.Background(), apiKey)
		})
	})

	t.Run("Gemini", func(t *testing.T) {
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
								if strings.Contains(string(text), "What is the next action needed") {
									// Return respond_to_user call to end session
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
			// Check if input is DefaultFacilitator's proceed prompt and return respond_to_user call
			if len(input) > 0 {
				if text, ok := input[0].(gollem.Text); ok {
					if strings.Contains(string(text), "What is the next action needed") {
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
						// Check if it's DefaultFacilitator's proceed prompt and return respond_to_user call
						if len(input) > 1 {
							if text, ok := input[1].(gollem.Text); ok {
								if strings.Contains(string(text), "What is the next action needed") {
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

			// Check if input is DefaultFacilitator's proceed prompt and return respond_to_user call
			if len(input) > 0 {
				if text, ok := input[0].(gollem.Text); ok {
					if strings.Contains(string(text), "What is the next action needed") {
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
						// Check if this is DefaultFacilitator's proceed prompt
						if len(input) > 0 {
							if text, ok := input[0].(gollem.Text); ok {
								if strings.Contains(string(text), "What is the next action needed") {
									// Return respond_to_user call to end session
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

						// Handle function responses
						if len(input) > 0 {
							if funcResp, ok := input[0].(gollem.FunctionResponse); ok {
								if funcResp.Name == "respond_to_user" {
									return &gollem.Response{}, nil
								}
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
				// Should have test_tool and respond_to_user (DefaultFacilitator)
				gt.Equal(t, len(tools), 2)
				toolNames := make(map[string]bool)
				for _, tool := range tools {
					toolNames[tool.Spec().Name] = true
				}
				gt.True(t, toolNames["test_tool"])
				gt.True(t, toolNames["respond_to_user"])

				mockSession := &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						// Check if this is DefaultFacilitator's proceed prompt
						if len(input) > 0 {
							if text, ok := input[0].(gollem.Text); ok {
								if strings.Contains(string(text), "What is the next action needed") {
									// Return respond_to_user call to end session
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

						// Handle function responses
						if len(input) > 0 {
							if funcResp, ok := input[0].(gollem.FunctionResponse); ok {
								if funcResp.Name == "respond_to_user" {
									return &gollem.Response{}, nil
								}
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
				// Should have test_tool from ToolSet and respond_to_user (DefaultFacilitator)
				gt.Equal(t, len(tools), 2)

				mockSession := &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						// Check if this is DefaultFacilitator's proceed prompt
						if len(input) > 0 {
							if text, ok := input[0].(gollem.Text); ok {
								if strings.Contains(string(text), "What is the next action needed") {
									// Return respond_to_user call to end session
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

						// Handle function responses
						if len(input) > 0 {
							if funcResp, ok := input[0].(gollem.FunctionResponse); ok {
								if funcResp.Name == "respond_to_user" {
									return &gollem.Response{}, nil
								}
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
				mockSession := &mock.SessionMock{
					GenerateStreamFunc: func(ctx context.Context, input ...gollem.Input) (<-chan *gollem.Response, error) {
						ch := make(chan *gollem.Response)
						go func() {
							defer close(ch)
							// Check if this is a proceed prompt from DefaultFacilitator
							if len(input) > 0 {
								if text, ok := input[0].(gollem.Text); ok {
									if strings.Contains(string(text), "What is the next action needed") {
										// End session by calling respond_to_user
										ch <- &gollem.Response{
											Texts: []string{"Session completed"},
											FunctionCalls: []*gollem.FunctionCall{
												{
													Name:      "respond_to_user",
													Arguments: map[string]any{},
												},
											},
										}
										return
									}
								}
							}

							ch <- &gollem.Response{
								Texts: []string{"test response 1"},
							}
							ch <- &gollem.Response{
								Texts: []string{"test response 2"},
							}
							ch <- &gollem.Response{
								Texts: []string{"test response 3"},
							}
						}()
						return ch, nil
					},
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						// Handle function responses
						if len(input) > 0 {
							if funcResp, ok := input[0].(gollem.FunctionResponse); ok {
								if funcResp.Name == "respond_to_user" {
									return &gollem.Response{}, nil
								}
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
		// Should receive the 3 test responses plus "Session completed"
		gt.Equal(t, len(receivedMessages), 4)
		gt.Equal(t, receivedMessages[0], "test response 1")
		gt.Equal(t, receivedMessages[1], "test response 2")
		gt.Equal(t, receivedMessages[2], "test response 3")
		gt.Equal(t, receivedMessages[3], "Session completed")
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
				// Should have test_tool and respond_to_user (DefaultFacilitator)
				gt.Equal(t, len(cfg.Tools()), 2)
				toolNames := make(map[string]bool)
				for _, tool := range cfg.Tools() {
					toolNames[tool.Spec().Name] = true
				}
				gt.True(t, toolNames["test_tool"])
				gt.True(t, toolNames["respond_to_user"])

				mockSession := &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						// Check if this is DefaultFacilitator's proceed prompt
						if len(input) > 0 {
							if text, ok := input[0].(gollem.Text); ok {
								if strings.Contains(string(text), "What is the next action needed") {
									// Return respond_to_user call to end session
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

						// Handle function responses
						if len(input) > 0 {
							if funcResp, ok := input[0].(gollem.FunctionResponse); ok {
								if funcResp.Name == "respond_to_user" {
									return &gollem.Response{}, nil
								}
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

func TestErrExitConversation(t *testing.T) {
	type testCase struct {
		name string
		test func(t *testing.T)
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			tc.test(t)
		}
	}

	t.Run("single tool returns ErrExitConversation", runTest(testCase{
		name: "single tool returns ErrExitConversation",
		test: func(t *testing.T) {
			toolCalled := false
			mockClient := &mock.LLMClientMock{
				NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
					mockSession := &mock.SessionMock{
						GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
							return &gollem.Response{
								Texts: []string{"Calling exit tool"},
								FunctionCalls: []*gollem.FunctionCall{
									{
										Name:      "exit_tool",
										Arguments: map[string]any{},
									},
								},
							}, nil
						},
						HistoryFunc: func() *gollem.History {
							return &gollem.History{}
						},
					}
					return mockSession, nil
				},
			}

			exitTool := &mockTool{
				spec: gollem.ToolSpec{
					Name:        "exit_tool",
					Description: "Tool that exits conversation",
				},
				run: func(ctx context.Context, args map[string]any) (map[string]any, error) {
					toolCalled = true
					return nil, gollem.ErrExitConversation
				},
			}

			s := gollem.New(mockClient, gollem.WithTools(exitTool))
			err := s.Execute(t.Context(), "test prompt")

			gt.NoError(t, err)
			gt.True(t, toolCalled)
			history := s.Session().History()
			gt.NotNil(t, history)
		},
	}))

	t.Run("multiple tools with one returning ErrExitConversation", runTest(testCase{
		name: "multiple tools with one returning ErrExitConversation",
		test: func(t *testing.T) {
			tool1Called := false
			tool2Called := false
			mockClient := &mock.LLMClientMock{
				NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
					mockSession := &mock.SessionMock{
						GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
							return &gollem.Response{
								Texts: []string{"Calling multiple tools"},
								FunctionCalls: []*gollem.FunctionCall{
									{
										Name:      "normal_tool",
										Arguments: map[string]any{},
									},
									{
										Name:      "exit_tool",
										Arguments: map[string]any{},
									},
								},
							}, nil
						},
						HistoryFunc: func() *gollem.History {
							return &gollem.History{}
						},
					}
					return mockSession, nil
				},
			}

			normalTool := &mockTool{
				spec: gollem.ToolSpec{
					Name:        "normal_tool",
					Description: "Normal tool that succeeds",
				},
				run: func(ctx context.Context, args map[string]any) (map[string]any, error) {
					tool1Called = true
					return map[string]any{"result": "success"}, nil
				},
			}

			exitTool := &mockTool{
				spec: gollem.ToolSpec{
					Name:        "exit_tool",
					Description: "Tool that exits conversation",
				},
				run: func(ctx context.Context, args map[string]any) (map[string]any, error) {
					tool2Called = true
					return nil, gollem.ErrExitConversation
				},
			}

			s := gollem.New(mockClient, gollem.WithTools(normalTool, exitTool))
			err := s.Execute(t.Context(), "test prompt")

			gt.NoError(t, err)
			gt.True(t, tool1Called)
			gt.True(t, tool2Called)
			history := s.Session().History()
			gt.NotNil(t, history)
		},
	}))

	t.Run("ErrExitConversation vs normal error behavior", runTest(testCase{
		name: "ErrExitConversation vs normal error behavior",
		test: func(t *testing.T) {
			// Test normal error continues conversation
			normalErrorToolCalled := false
			mockClient1 := &mock.LLMClientMock{
				NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
					mockSession := &mock.SessionMock{
						GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
							// If input is error response, check and return respond_to_user
							if len(input) > 0 {
								if funcResp, ok := input[0].(gollem.FunctionResponse); ok && funcResp.Error != nil {
									return &gollem.Response{
										Texts: []string{"Error handled, completing task"},
										FunctionCalls: []*gollem.FunctionCall{
											{
												Name:      "respond_to_user",
												Arguments: map[string]any{},
											},
										},
									}, nil
								}
								if funcResp, ok := input[0].(gollem.FunctionResponse); ok && funcResp.Name == "respond_to_user" {
									return &gollem.Response{}, nil
								}
							}
							return &gollem.Response{
								Texts: []string{"Calling error tool"},
								FunctionCalls: []*gollem.FunctionCall{
									{
										Name:      "error_tool",
										Arguments: map[string]any{},
									},
								},
							}, nil
						},
						HistoryFunc: func() *gollem.History {
							return &gollem.History{}
						},
					}
					return mockSession, nil
				},
			}

			errorTool := &mockTool{
				spec: gollem.ToolSpec{
					Name:        "error_tool",
					Description: "Tool that returns normal error",
				},
				run: func(ctx context.Context, args map[string]any) (map[string]any, error) {
					normalErrorToolCalled = true
					return nil, errors.New("normal error")
				},
			}

			s1 := gollem.New(mockClient1, gollem.WithTools(errorTool))
			err1 := s1.Execute(t.Context(), "test prompt")

			gt.NoError(t, err1) // Normal error doesn't terminate session
			gt.True(t, normalErrorToolCalled)
			history1 := s1.Session().History()
			gt.NotNil(t, history1)

			// Test ErrExitConversation terminates immediately
			exitToolCalled := false
			mockClient2 := &mock.LLMClientMock{
				NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
					mockSession := &mock.SessionMock{
						GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
							return &gollem.Response{
								Texts: []string{"Calling exit tool"},
								FunctionCalls: []*gollem.FunctionCall{
									{
										Name:      "exit_tool",
										Arguments: map[string]any{},
									},
								},
							}, nil
						},
						HistoryFunc: func() *gollem.History {
							return &gollem.History{}
						},
					}
					return mockSession, nil
				},
			}

			exitTool := &mockTool{
				spec: gollem.ToolSpec{
					Name:        "exit_tool",
					Description: "Tool that exits conversation",
				},
				run: func(ctx context.Context, args map[string]any) (map[string]any, error) {
					exitToolCalled = true
					return nil, gollem.ErrExitConversation
				},
			}

			s2 := gollem.New(mockClient2, gollem.WithTools(exitTool))
			err2 := s2.Execute(t.Context(), "test prompt")

			gt.NoError(t, err2) // ErrExitConversation is treated as success
			gt.True(t, exitToolCalled)
			history2 := s2.Session().History()
			gt.NotNil(t, history2)
		},
	}))

	t.Run("ErrExitConversation with streaming mode", runTest(testCase{
		name: "ErrExitConversation with streaming mode",
		test: func(t *testing.T) {
			toolCalled := false
			mockClient := &mock.LLMClientMock{
				NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
					mockSession := &mock.SessionMock{
						GenerateStreamFunc: func(ctx context.Context, input ...gollem.Input) (<-chan *gollem.Response, error) {
							ch := make(chan *gollem.Response, 1)
							go func() {
								defer close(ch)
								ch <- &gollem.Response{
									Texts: []string{"Calling exit tool"},
									FunctionCalls: []*gollem.FunctionCall{
										{
											Name:      "exit_tool",
											Arguments: map[string]any{},
										},
									},
								}
							}()
							return ch, nil
						},
						HistoryFunc: func() *gollem.History {
							return &gollem.History{}
						},
					}
					return mockSession, nil
				},
			}

			exitTool := &mockTool{
				spec: gollem.ToolSpec{
					Name:        "exit_tool",
					Description: "Tool that exits conversation",
				},
				run: func(ctx context.Context, args map[string]any) (map[string]any, error) {
					toolCalled = true
					return nil, gollem.ErrExitConversation
				},
			}

			s := gollem.New(mockClient,
				gollem.WithTools(exitTool),
				gollem.WithResponseMode(gollem.ResponseModeStreaming),
			)
			err := s.Execute(t.Context(), "test prompt")

			gt.NoError(t, err)
			gt.True(t, toolCalled)
			history := s.Session().History()
			gt.NotNil(t, history)
		},
	}))
}
