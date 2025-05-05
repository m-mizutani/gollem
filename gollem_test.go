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

				toolCalled := false
				s := gollem.New(client,
					gollem.WithTools(&RandomNumberTool{}),
					gollem.WithToolRequestHook(func(ctx context.Context, tool gollem.FunctionCall) error {
						toolCalled = true
						gt.Equal(t, tool.Name, "random_number")
						return nil
					}),
					gollem.WithResponseMode(respMode),
				)

				_, err = s.Prompt(t.Context(), "Generate a random number between 1 and 100.")
				gt.NoError(t, err)
				gt.True(t, toolCalled)
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
						if _, ok := input[0].(gollem.FunctionResponse); ok {
							// Return empty response to stop the loop
							return &gollem.Response{}, nil
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
				toolRequestCalled = true
				gt.Equal(t, tool.Name, "test_tool")
				gt.Equal(t, tool.Arguments["arg1"], "value1")
				return nil
			}),
			gollem.WithLoopLimit(1),
		)

		_, err := s.Prompt(t.Context(), "test message")
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
				toolResponseCalled = true
				gt.Equal(t, tool.Name, "test_tool")
				gt.Equal(t, response["result"], "test_result")
				return nil
			}),
			gollem.WithLoopLimit(1),
		)

		_, err := s.Prompt(t.Context(), "test message")
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
				toolErrorCalled = true
				gt.Equal(t, err.Error(), "test error")
				gt.Equal(t, tool.Name, "test_tool")
				return nil
			}),
			gollem.WithLoopLimit(1),
		)

		_, err := s.Prompt(t.Context(), "test message")
		gt.NoError(t, err)
		gt.True(t, toolErrorCalled)
	})

	t.Run("MessageHook", func(t *testing.T) {
		messageHookCalled := false
		s := gollem.New(mockClient,
			gollem.WithMessageHook(func(ctx context.Context, msg string) error {
				messageHookCalled = true
				gt.Equal(t, msg, "test response")
				return nil
			}),
			gollem.WithLoopLimit(1),
		)

		_, err := s.Prompt(t.Context(), "test message")
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
				GenerateContentFunc: generateContentFunc,
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
		_, err := s.Prompt(t.Context(), "test message")
		gt.Error(t, err)
		gt.True(t, errors.Is(err, gollem.ErrLoopLimitExceeded))
		gt.Equal(t, loopCount, 11) // 10回のループ + 1回の制限チェック
	})

	t.Run("WithRetryLimit", func(t *testing.T) {
		retryCount := 0
		mockClient := newMockClient(func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
			// If input is a function response with error, continue the loop
			if len(input) > 0 {
				if resp, ok := input[0].(gollem.FunctionResponse); ok {
					if resp.Error != nil {
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
		_, err := s.Prompt(t.Context(), "test message")
		gt.NoError(t, err)
		gt.Equal(t, retryCount, 3)
	})

	t.Run("WithInitPrompt", func(t *testing.T) {
		mockClient := newMockClient(func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
			// Check if the first input is initial prompt
			if len(input) > 1 {
				if text, ok := input[0].(gollem.Text); ok {
					gt.Equal(t, string(text), "initial prompt")
				}
				if text, ok := input[1].(gollem.Text); ok {
					gt.Equal(t, string(text), "test message")
				}
			}
			return &gollem.Response{
				Texts: []string{"test response"},
			}, nil
		})

		s := gollem.New(mockClient, gollem.WithInitPrompt("initial prompt"))
		_, err := s.Prompt(t.Context(), "test message")
		gt.NoError(t, err)
	})

	t.Run("WithSystemPrompt", func(t *testing.T) {
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				cfg := gollem.NewSessionConfig(options...)
				gt.Equal(t, cfg.SystemPrompt(), "system prompt")
				mockSession := &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						return &gollem.Response{
							Texts: []string{"test response"},
						}, nil
					},
				}
				return mockSession, nil
			},
		}

		s := gollem.New(mockClient, gollem.WithSystemPrompt("system prompt"))
		_, err := s.Prompt(t.Context(), "test message")
		gt.NoError(t, err)
	})

	t.Run("WithTools", func(t *testing.T) {
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				cfg := gollem.NewSessionConfig(options...)
				tools := cfg.Tools()
				gt.Equal(t, len(tools), 1)
				gt.Equal(t, tools[0].Spec().Name, "test_tool")
				gt.Equal(t, tools[0].Spec().Description, "A test tool")

				mockSession := &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						return &gollem.Response{
							Texts: []string{"test response"},
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
		s := gollem.New(mockClient, gollem.WithTools(tool), gollem.WithLoopLimit(1))
		_, err := s.Prompt(t.Context(), "test message")
		gt.NoError(t, err)
	})

	t.Run("WithToolSets", func(t *testing.T) {
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				cfg := gollem.NewSessionConfig(options...)
				tools := cfg.Tools()
				gt.Equal(t, len(tools), 1) // ToolSets are converted to Tools in SessionConfig

				mockSession := &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						return &gollem.Response{
							Texts: []string{"test response"},
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
		s := gollem.New(mockClient, gollem.WithToolSets(toolSet), gollem.WithLoopLimit(1))
		_, err := s.Prompt(t.Context(), "test message")
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
		_, err := s.Prompt(t.Context(), "test message")
		gt.NoError(t, err)
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
			return &gollem.Response{
				Texts: []string{"test response"},
			}, nil
		})

		s := gollem.New(mockClient, gollem.WithLogger(logger))
		_, err := s.Prompt(t.Context(), "test message")
		gt.NoError(t, err)

		logContent := logOutput.String()
		gt.True(t, len(logContent) > 0)
	})

	t.Run("CombineOptions", func(t *testing.T) {
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				cfg := gollem.NewSessionConfig(options...)
				gt.Equal(t, cfg.SystemPrompt(), "system prompt")
				gt.Equal(t, len(cfg.Tools()), 1)
				gt.Equal(t, cfg.Tools()[0].Spec().Name, "test_tool")

				mockSession := &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						// Check if the first input is initial prompt
						if len(input) > 1 {
							if text, ok := input[0].(gollem.Text); ok {
								gt.Equal(t, string(text), "initial prompt")
							}
							if text, ok := input[1].(gollem.Text); ok {
								gt.Equal(t, string(text), "test message")
							}
						}
						return &gollem.Response{
							Texts: []string{"test response"},
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
			gollem.WithLoopLimit(1),
			gollem.WithRetryLimit(5),
			gollem.WithInitPrompt("initial prompt"),
			gollem.WithSystemPrompt("system prompt"),
			gollem.WithTools(tool),
			gollem.WithResponseMode(gollem.ResponseModeBlocking),
			gollem.WithLogger(logger),
		)
		_, err := s.Prompt(t.Context(), "test message")
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
