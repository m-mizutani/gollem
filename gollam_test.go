package gollam_test

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

	"github.com/m-mizutani/gollam"
	"github.com/m-mizutani/gollam/llm/claude"
	"github.com/m-mizutani/gollam/llm/gemini"
	"github.com/m-mizutani/gollam/llm/gpt"
	"github.com/m-mizutani/gollam/mock"
	"github.com/m-mizutani/gt"
)

// RandomNumberTool is a tool that generates a random number within a specified range
type RandomNumberTool struct{}

func (t *RandomNumberTool) Spec() gollam.ToolSpec {
	return gollam.ToolSpec{
		Name:        "random_number",
		Description: "Generates a random number within a specified range",
		Parameters: map[string]*gollam.Parameter{
			"min": {
				Type:        gollam.TypeNumber,
				Description: "Minimum value of the range",
			},
			"max": {
				Type:        gollam.TypeNumber,
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

func TestGollamWithTool(t *testing.T) {
	respModes := []gollam.ResponseMode{
		gollam.ResponseModeBlocking,
		gollam.ResponseModeStreaming,
	}

	testFn := func(t *testing.T, newClient func(t *testing.T) (gollam.LLMClient, error)) {
		for _, respMode := range respModes {
			t.Run(fmt.Sprintf("ResponseMode=%s", respMode), func(t *testing.T) {
				client, err := newClient(t)
				gt.NoError(t, err)

				toolCalled := false
				s := gollam.New(client,
					gollam.WithTools(&RandomNumberTool{}),
					gollam.WithToolRequestHook(func(ctx context.Context, tool gollam.FunctionCall) error {
						toolCalled = true
						gt.Equal(t, tool.Name, "random_number")
						return nil
					}),
					gollam.WithResponseMode(respMode),
				)

				_, err = s.Prompt(t.Context(), "Generate a random number between 1 and 100.")
				gt.NoError(t, err)
				gt.True(t, toolCalled)
			})
		}
	}

	t.Run("GPT", func(t *testing.T) {
		apiKey, ok := os.LookupEnv("TEST_OPENAI_API_KEY")
		if !ok {
			t.Skip("TEST_OPENAI_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollam.LLMClient, error) {
			return gpt.New(context.Background(), apiKey)
		})
	})

	t.Run("Claude", func(t *testing.T) {
		apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
		if !ok {
			t.Skip("TEST_CLAUDE_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollam.LLMClient, error) {
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
		testFn(t, func(t *testing.T) (gollam.LLMClient, error) {
			return gemini.New(context.Background(), projectID, location)
		})
	})
}

func TestGollamWithHooks(t *testing.T) {
	mockClient := &mock.LLMClientMock{
		NewSessionFunc: func(ctx context.Context, options ...gollam.SessionOption) (gollam.Session, error) {
			mockSession := &mock.SessionMock{
				GenerateContentFunc: func(ctx context.Context, input ...gollam.Input) (*gollam.Response, error) {
					// Check if the input is a function response
					if len(input) > 0 {
						if _, ok := input[0].(gollam.FunctionResponse); ok {
							// Return empty response to stop the loop
							return &gollam.Response{}, nil
						}
					}

					// Return response with function call
					return &gollam.Response{
						Texts: []string{"test response"},
						FunctionCalls: []*gollam.FunctionCall{
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
		s := gollam.New(mockClient,
			gollam.WithTools(&RandomNumberTool{}),
			gollam.WithToolRequestHook(func(ctx context.Context, tool gollam.FunctionCall) error {
				toolRequestCalled = true
				gt.Equal(t, tool.Name, "test_tool")
				gt.Equal(t, tool.Arguments["arg1"], "value1")
				return nil
			}),
			gollam.WithLoopLimit(1),
		)

		_, err := s.Prompt(t.Context(), "test message")
		gt.NoError(t, err)
		gt.True(t, toolRequestCalled)
	})

	t.Run("ToolResponseHook", func(t *testing.T) {
		// Create a tool that returns a test result
		testTool := &mockTool{
			spec: gollam.ToolSpec{
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
		s := gollam.New(mockClient,
			gollam.WithTools(testTool),
			gollam.WithToolResponseHook(func(ctx context.Context, tool gollam.FunctionCall, response map[string]any) error {
				toolResponseCalled = true
				gt.Equal(t, tool.Name, "test_tool")
				gt.Equal(t, response["result"], "test_result")
				return nil
			}),
			gollam.WithLoopLimit(1),
		)

		_, err := s.Prompt(t.Context(), "test message")
		gt.NoError(t, err)
		gt.True(t, toolResponseCalled)
	})

	t.Run("ToolErrorHook", func(t *testing.T) {
		// Create a tool that always returns an error
		errorTool := &mockTool{
			spec: gollam.ToolSpec{
				Name:        "test_tool", // Match the name in the mock response
				Description: "A tool that always returns an error",
			},
			run: func(ctx context.Context, args map[string]any) (map[string]any, error) {
				return nil, fmt.Errorf("test error")
			},
		}

		toolErrorCalled := false
		s := gollam.New(mockClient,
			gollam.WithTools(errorTool),
			gollam.WithToolErrorHook(func(ctx context.Context, err error, tool gollam.FunctionCall) error {
				toolErrorCalled = true
				gt.Equal(t, err.Error(), "test error")
				gt.Equal(t, tool.Name, "test_tool")
				return nil
			}),
			gollam.WithLoopLimit(1),
		)

		_, err := s.Prompt(t.Context(), "test message")
		gt.NoError(t, err)
		gt.True(t, toolErrorCalled)
	})

	t.Run("MessageHook", func(t *testing.T) {
		messageHookCalled := false
		s := gollam.New(mockClient,
			gollam.WithMessageHook(func(ctx context.Context, msg string) error {
				messageHookCalled = true
				gt.Equal(t, msg, "test response")
				return nil
			}),
			gollam.WithLoopLimit(1),
		)

		_, err := s.Prompt(t.Context(), "test message")
		gt.NoError(t, err)
		gt.True(t, messageHookCalled)
	})
}

// mockTool is a mock implementation of gollam.Tool
type mockTool struct {
	spec gollam.ToolSpec
	run  func(ctx context.Context, args map[string]any) (map[string]any, error)
}

func (t *mockTool) Spec() gollam.ToolSpec {
	return t.spec
}

func (t *mockTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return t.run(ctx, args)
}

// newMockClient creates a new LLMClientMock with the given GenerateContentFunc
func newMockClient(generateContentFunc func(ctx context.Context, input ...gollam.Input) (*gollam.Response, error)) *mock.LLMClientMock {
	return &mock.LLMClientMock{
		NewSessionFunc: func(ctx context.Context, options ...gollam.SessionOption) (gollam.Session, error) {
			mockSession := &mock.SessionMock{
				GenerateContentFunc: generateContentFunc,
			}
			return mockSession, nil
		},
	}
}

func TestGollamWithOptions(t *testing.T) {
	t.Run("WithLoopLimit", func(t *testing.T) {
		loopCount := 0
		mockClient := newMockClient(func(ctx context.Context, input ...gollam.Input) (*gollam.Response, error) {
			loopCount++
			return &gollam.Response{
				Texts: []string{"test response"},
				FunctionCalls: []*gollam.FunctionCall{
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
			SpecFunc: func() gollam.ToolSpec {
				return gollam.ToolSpec{
					Name:        "test_tool",
					Description: "A test tool",
				}
			},
			RunFunc: func(ctx context.Context, args map[string]any) (map[string]any, error) {
				return map[string]any{"result": "test"}, nil
			},
		}

		s := gollam.New(mockClient, gollam.WithLoopLimit(10), gollam.WithTools(tool))
		_, err := s.Prompt(t.Context(), "test message")
		gt.Error(t, err)
		gt.True(t, errors.Is(err, gollam.ErrLoopLimitExceeded))
		gt.Equal(t, loopCount, 11) // 10回のループ + 1回の制限チェック
	})

	t.Run("WithRetryLimit", func(t *testing.T) {
		retryCount := 0
		mockClient := newMockClient(func(ctx context.Context, input ...gollam.Input) (*gollam.Response, error) {
			// If input is a function response with error, continue the loop
			if len(input) > 0 {
				if resp, ok := input[0].(gollam.FunctionResponse); ok {
					if resp.Error != nil {
						return &gollam.Response{
							Texts: []string{"retrying..."},
							FunctionCalls: []*gollam.FunctionCall{
								{
									Name: "test_tool",
									Arguments: map[string]any{
										"arg1": "value1",
									},
								},
							},
						}, nil
					}
					return &gollam.Response{
						Texts: []string{"success"},
					}, nil
				}
			}

			return &gollam.Response{
				Texts: []string{"test response"},
				FunctionCalls: []*gollam.FunctionCall{
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
			SpecFunc: func() gollam.ToolSpec {
				return gollam.ToolSpec{
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

		s := gollam.New(mockClient, gollam.WithRetryLimit(5), gollam.WithLoopLimit(5), gollam.WithTools(tool))
		_, err := s.Prompt(t.Context(), "test message")
		gt.NoError(t, err)
		gt.Equal(t, retryCount, 3)
	})

	t.Run("WithInitPrompt", func(t *testing.T) {
		mockClient := newMockClient(func(ctx context.Context, input ...gollam.Input) (*gollam.Response, error) {
			// Check if the first input is initial prompt
			if len(input) > 1 {
				if text, ok := input[0].(gollam.Text); ok {
					gt.Equal(t, string(text), "initial prompt")
				}
				if text, ok := input[1].(gollam.Text); ok {
					gt.Equal(t, string(text), "test message")
				}
			}
			return &gollam.Response{
				Texts: []string{"test response"},
			}, nil
		})

		s := gollam.New(mockClient, gollam.WithInitPrompt("initial prompt"))
		_, err := s.Prompt(t.Context(), "test message")
		gt.NoError(t, err)
	})

	t.Run("WithSystemPrompt", func(t *testing.T) {
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollam.SessionOption) (gollam.Session, error) {
				cfg := gollam.NewSessionConfig(options...)
				gt.Equal(t, cfg.SystemPrompt(), "system prompt")
				mockSession := &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollam.Input) (*gollam.Response, error) {
						return &gollam.Response{
							Texts: []string{"test response"},
						}, nil
					},
				}
				return mockSession, nil
			},
		}

		s := gollam.New(mockClient, gollam.WithSystemPrompt("system prompt"))
		_, err := s.Prompt(t.Context(), "test message")
		gt.NoError(t, err)
	})

	t.Run("WithTools", func(t *testing.T) {
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollam.SessionOption) (gollam.Session, error) {
				cfg := gollam.NewSessionConfig(options...)
				tools := cfg.Tools()
				gt.Equal(t, len(tools), 1)
				gt.Equal(t, tools[0].Spec().Name, "test_tool")
				gt.Equal(t, tools[0].Spec().Description, "A test tool")

				mockSession := &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollam.Input) (*gollam.Response, error) {
						return &gollam.Response{
							Texts: []string{"test response"},
						}, nil
					},
				}
				return mockSession, nil
			},
		}

		tool := &mockTool{
			spec: gollam.ToolSpec{
				Name:        "test_tool",
				Description: "A test tool",
			},
			run: func(ctx context.Context, args map[string]any) (map[string]any, error) {
				return map[string]any{"result": "test"}, nil
			},
		}
		s := gollam.New(mockClient, gollam.WithTools(tool), gollam.WithLoopLimit(1))
		_, err := s.Prompt(t.Context(), "test message")
		gt.NoError(t, err)
	})

	t.Run("WithToolSets", func(t *testing.T) {
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollam.SessionOption) (gollam.Session, error) {
				cfg := gollam.NewSessionConfig(options...)
				tools := cfg.Tools()
				gt.Equal(t, len(tools), 1) // ToolSets are converted to Tools in SessionConfig

				mockSession := &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollam.Input) (*gollam.Response, error) {
						return &gollam.Response{
							Texts: []string{"test response"},
						}, nil
					},
				}
				return mockSession, nil
			},
		}

		toolSet := &mockToolSet{
			specs: []gollam.ToolSpec{
				{
					Name:        "test_tool",
					Description: "A test tool",
				},
			},
			run: func(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
				return map[string]any{"result": "test"}, nil
			},
		}
		s := gollam.New(mockClient, gollam.WithToolSets(toolSet), gollam.WithLoopLimit(1))
		_, err := s.Prompt(t.Context(), "test message")
		gt.NoError(t, err)
	})

	t.Run("WithResponseMode", func(t *testing.T) {
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollam.SessionOption) (gollam.Session, error) {
				mockSession := &mock.SessionMock{
					GenerateStreamFunc: func(ctx context.Context, input ...gollam.Input) (<-chan *gollam.Response, error) {
						ch := make(chan *gollam.Response)
						go func() {
							defer close(ch)
							ch <- &gollam.Response{
								Texts: []string{"test response 1"},
							}
							ch <- &gollam.Response{
								Texts: []string{"test response 2"},
							}
							ch <- &gollam.Response{
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
		s := gollam.New(mockClient,
			gollam.WithResponseMode(gollam.ResponseModeStreaming),
			gollam.WithMessageHook(func(ctx context.Context, msg string) error {
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

		mockClient := newMockClient(func(ctx context.Context, input ...gollam.Input) (*gollam.Response, error) {
			return &gollam.Response{
				Texts: []string{"test response"},
			}, nil
		})

		s := gollam.New(mockClient, gollam.WithLogger(logger))
		_, err := s.Prompt(t.Context(), "test message")
		gt.NoError(t, err)

		logContent := logOutput.String()
		gt.True(t, len(logContent) > 0)
	})

	t.Run("CombineOptions", func(t *testing.T) {
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollam.SessionOption) (gollam.Session, error) {
				cfg := gollam.NewSessionConfig(options...)
				gt.Equal(t, cfg.SystemPrompt(), "system prompt")
				gt.Equal(t, len(cfg.Tools()), 1)
				gt.Equal(t, cfg.Tools()[0].Spec().Name, "test_tool")

				mockSession := &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollam.Input) (*gollam.Response, error) {
						// Check if the first input is initial prompt
						if len(input) > 1 {
							if text, ok := input[0].(gollam.Text); ok {
								gt.Equal(t, string(text), "initial prompt")
							}
							if text, ok := input[1].(gollam.Text); ok {
								gt.Equal(t, string(text), "test message")
							}
						}
						return &gollam.Response{
							Texts: []string{"test response"},
						}, nil
					},
				}
				return mockSession, nil
			},
		}

		tool := &mockTool{
			spec: gollam.ToolSpec{
				Name:        "test_tool",
				Description: "A test tool",
			},
			run: func(ctx context.Context, args map[string]any) (map[string]any, error) {
				return map[string]any{"result": "test"}, nil
			},
		}
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))

		s := gollam.New(mockClient,
			gollam.WithLoopLimit(1),
			gollam.WithRetryLimit(5),
			gollam.WithInitPrompt("initial prompt"),
			gollam.WithSystemPrompt("system prompt"),
			gollam.WithTools(tool),
			gollam.WithResponseMode(gollam.ResponseModeBlocking),
			gollam.WithLogger(logger),
		)
		_, err := s.Prompt(t.Context(), "test message")
		gt.NoError(t, err)
	})
}

// mockToolSet is a mock implementation of gollam.ToolSet
type mockToolSet struct {
	specs []gollam.ToolSpec
	run   func(ctx context.Context, name string, args map[string]any) (map[string]any, error)
}

func (t *mockToolSet) Specs(ctx context.Context) ([]gollam.ToolSpec, error) {
	return t.specs, nil
}

func (t *mockToolSet) Run(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
	return t.run(ctx, name, args)
}
