package gollem_test

import (
	"context"
	"errors"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/mock"
	"github.com/m-mizutani/gt"
)

// Helper to create a mock agent that returns a specific response
func newMockAgent(response string) *gollem.Agent {
	callCount := 0
	mockClient := &mock.LLMClientMock{
		NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
			return &mock.SessionMock{
				GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
					callCount++
					return &gollem.Response{
						Texts: []string{response},
					}, nil
				},
			}, nil
		},
	}
	return gollem.New(mockClient)
}

func TestNewSubAgent(t *testing.T) {
	t.Run("create subagent with default mode", func(t *testing.T) {
		childAgent := newMockAgent("test response")
		subagent := gollem.NewSubAgent("test_agent", "A test subagent", childAgent)

		gt.NotNil(t, subagent)

		spec := subagent.Spec()
		gt.Equal(t, "test_agent", spec.Name)
		gt.Equal(t, "A test subagent", spec.Description)
	})

	t.Run("create subagent with template mode", func(t *testing.T) {
		childAgent := newMockAgent("test response")

		prompt, err := gollem.NewPromptTemplate(
			"Analyze {{.code}} with focus on {{.focus}}",
			map[string]*gollem.Parameter{
				"code":  {Type: gollem.TypeString, Description: "Code to analyze", Required: true},
				"focus": {Type: gollem.TypeString, Description: "Focus area", Required: true},
			},
		)
		gt.NoError(t, err)

		subagent := gollem.NewSubAgent(
			"analyzer",
			"Analyzes code",
			childAgent,
			gollem.WithPromptTemplate(prompt),
		)

		gt.NotNil(t, subagent)

		spec := subagent.Spec()
		gt.Equal(t, "analyzer", spec.Name)
		gt.Equal(t, "Analyzes code", spec.Description)
	})
}

func TestSubAgentSpec_DefaultMode(t *testing.T) {
	childAgent := newMockAgent("response")
	subagent := gollem.NewSubAgent("my_agent", "My description", childAgent)

	spec := subagent.Spec()

	gt.Equal(t, "my_agent", spec.Name)
	gt.Equal(t, "My description", spec.Description)
	gt.Equal(t, 1, len(spec.Parameters))

	queryParam, exists := spec.Parameters["query"]
	gt.True(t, exists)
	gt.Equal(t, gollem.TypeString, queryParam.Type)
	gt.True(t, queryParam.Required)
	gt.Equal(t, "Natural language query to send to the subagent", queryParam.Description)
}

func TestSubAgentSpec_TemplateMode(t *testing.T) {
	childAgent := newMockAgent("response")

	prompt, err := gollem.NewPromptTemplate(
		"Analyze the following code focusing on {{.focus}}:\n\n{{.code}}",
		map[string]*gollem.Parameter{
			"code":  {Type: gollem.TypeString, Description: "Code to analyze", Required: true},
			"focus": {Type: gollem.TypeString, Description: "Focus area (security, performance, etc.)", Required: true},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	subagent := gollem.NewSubAgent(
		"code_analyzer",
		"Analyzes code with specified focus area",
		childAgent,
		gollem.WithPromptTemplate(prompt),
	)

	spec := subagent.Spec()

	gt.Equal(t, "code_analyzer", spec.Name)
	gt.Equal(t, "Analyzes code with specified focus area", spec.Description)
	gt.Equal(t, 2, len(spec.Parameters))

	codeParam, exists := spec.Parameters["code"]
	gt.True(t, exists)
	gt.Equal(t, gollem.TypeString, codeParam.Type)
	gt.True(t, codeParam.Required)
	gt.Equal(t, "Code to analyze", codeParam.Description)

	focusParam, exists := spec.Parameters["focus"]
	gt.True(t, exists)
	gt.Equal(t, gollem.TypeString, focusParam.Type)
	gt.True(t, focusParam.Required)
	gt.Equal(t, "Focus area (security, performance, etc.)", focusParam.Description)
}

func TestSubAgentRun_DefaultMode(t *testing.T) {
	t.Run("successful execution", func(t *testing.T) {
		var capturedInput gollem.Input
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						if len(input) > 0 {
							capturedInput = input[0]
						}
						return &gollem.Response{
							Texts: []string{"Processed: success"},
						}, nil
					},
				}, nil
			},
		}
		childAgent := gollem.New(mockClient)
		subagent := gollem.NewSubAgent("processor", "Processes queries", childAgent)

		result, err := subagent.Run(context.Background(), map[string]any{
			"query": "Process this text",
		})

		gt.NoError(t, err)
		gt.NotNil(t, result)
		gt.Equal(t, "success", result["status"])
		gt.Equal(t, "Processed: success", result["response"])

		// Verify the query was passed to the child agent
		text, ok := capturedInput.(gollem.Text)
		gt.True(t, ok)
		gt.Equal(t, gollem.Text("Process this text"), text)
	})

	t.Run("missing required query parameter returns error", func(t *testing.T) {
		childAgent := newMockAgent("response")
		subagent := gollem.NewSubAgent("processor", "Processes queries", childAgent)

		// query is a required parameter, so missing it returns an error
		result, err := subagent.Run(context.Background(), map[string]any{})

		gt.Error(t, err)
		gt.Nil(t, result)
	})

	t.Run("non-string query parameter returns type error", func(t *testing.T) {
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						return &gollem.Response{
							Texts: []string{"Processed"},
						}, nil
					},
				}, nil
			},
		}
		childAgent := gollem.New(mockClient)
		subagent := gollem.NewSubAgent("processor", "Processes queries", childAgent)

		// Non-string value for string parameter returns type error
		result, err := subagent.Run(context.Background(), map[string]any{
			"query": 12345,
		})

		gt.Error(t, err)
		gt.Nil(t, result)
	})
}

func TestSubAgentRun_TemplateMode(t *testing.T) {
	t.Run("successful template execution", func(t *testing.T) {
		var capturedInput gollem.Input
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						if len(input) > 0 {
							capturedInput = input[0]
						}
						return &gollem.Response{
							Texts: []string{"Analysis complete"},
						}, nil
					},
				}, nil
			},
		}
		childAgent := gollem.New(mockClient)

		prompt, err := gollem.NewPromptTemplate(
			"Analyze: {{.code}}, Focus: {{.focus}}",
			map[string]*gollem.Parameter{
				"code":  {Type: gollem.TypeString, Description: "Code"},
				"focus": {Type: gollem.TypeString, Description: "Focus"},
			},
		)
		gt.NoError(t, err)

		subagent := gollem.NewSubAgent(
			"analyzer",
			"Code analyzer",
			childAgent,
			gollem.WithPromptTemplate(prompt),
		)

		result, err := subagent.Run(context.Background(), map[string]any{
			"code":  "func main() {}",
			"focus": "security",
		})

		gt.NoError(t, err)
		gt.NotNil(t, result)
		gt.Equal(t, "success", result["status"])
		gt.Equal(t, "Analysis complete", result["response"])

		// Verify the template was rendered correctly
		text, ok := capturedInput.(gollem.Text)
		gt.True(t, ok)
		gt.Equal(t, gollem.Text("Analyze: func main() {}, Focus: security"), text)
	})

	t.Run("template with missing variable uses zero value", func(t *testing.T) {
		var capturedInput gollem.Input
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						if len(input) > 0 {
							capturedInput = input[0]
						}
						return &gollem.Response{
							Texts: []string{"done"},
						}, nil
					},
				}, nil
			},
		}
		childAgent := gollem.New(mockClient)

		prompt, err := gollem.NewPromptTemplate(
			"Value: {{.optional}}",
			map[string]*gollem.Parameter{
				"optional": {Type: gollem.TypeString, Description: "Optional value"},
			},
		)
		gt.NoError(t, err)

		subagent := gollem.NewSubAgent(
			"test",
			"Test",
			childAgent,
			gollem.WithPromptTemplate(prompt),
		)

		result, err := subagent.Run(context.Background(), map[string]any{})

		// missingkey=zero replaces missing variables with empty string
		gt.NoError(t, err)
		gt.NotNil(t, result)

		text, ok := capturedInput.(gollem.Text)
		gt.True(t, ok)
		gt.Equal(t, gollem.Text("Value: "), text)
	})
}

func TestSubAgentTemplateRendering(t *testing.T) {
	t.Run("complex template", func(t *testing.T) {
		var capturedInput gollem.Input
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						if len(input) > 0 {
							capturedInput = input[0]
						}
						return &gollem.Response{
							Texts: []string{"ok"},
						}, nil
					},
				}, nil
			},
		}
		childAgent := gollem.New(mockClient)

		prompt, err := gollem.NewPromptTemplate(
			`Review the code:
File: {{.filename}}
Language: {{.language}}

{{.code}}

Focus on: {{.focus}}`,
			map[string]*gollem.Parameter{
				"filename": {Type: gollem.TypeString, Description: "Filename"},
				"language": {Type: gollem.TypeString, Description: "Programming language"},
				"code":     {Type: gollem.TypeString, Description: "Code content"},
				"focus":    {Type: gollem.TypeString, Description: "Review focus"},
			},
		)
		gt.NoError(t, err)

		subagent := gollem.NewSubAgent(
			"reviewer",
			"Code reviewer",
			childAgent,
			gollem.WithPromptTemplate(prompt),
		)

		result, err := subagent.Run(context.Background(), map[string]any{
			"filename": "main.go",
			"language": "Go",
			"code":     "package main",
			"focus":    "best practices",
		})

		gt.NoError(t, err)
		gt.Equal(t, "success", result["status"])

		expected := `Review the code:
File: main.go
Language: Go

package main

Focus on: best practices`

		text, ok := capturedInput.(gollem.Text)
		gt.True(t, ok)
		gt.Equal(t, gollem.Text(expected), text)
	})

	t.Run("invalid template syntax returns error", func(t *testing.T) {
		// Invalid template syntax returns error from NewPromptTemplate
		_, err := gollem.NewPromptTemplate(
			"{{.invalid}", // invalid syntax
			map[string]*gollem.Parameter{
				"value": {Type: gollem.TypeString, Description: "Value"},
			},
		)

		gt.Error(t, err)
	})
}

func TestAgentWithSubAgent(t *testing.T) {
	t.Run("parent agent invokes subagent as tool", func(t *testing.T) {
		// Create child agent
		childClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						return &gollem.Response{
							Texts: []string{"Child agent response"},
						}, nil
					},
				}, nil
			},
		}
		childAgent := gollem.New(childClient)

		// Create subagent
		subagent := gollem.NewSubAgent("helper", "A helper subagent", childAgent)

		// Create parent agent that will call the subagent
		callCount := 0
		subagentCalled := false
		parentClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				// Verify subagent is in the tools list
				cfg := gollem.NewSessionConfig(options...)
				tools := cfg.Tools()
				hasHelper := false
				for _, tool := range tools {
					if tool.Spec().Name == "helper" {
						hasHelper = true
						break
					}
				}
				gt.True(t, hasHelper)

				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						callCount++

						if callCount == 1 {
							// First call: invoke subagent
							return &gollem.Response{
								Texts: []string{"I'll use the helper"},
								FunctionCalls: []*gollem.FunctionCall{
									{
										ID:   "call_helper_1",
										Name: "helper",
										Arguments: map[string]any{
											"query": "Help me with this task",
										},
									},
								},
							}, nil
						}

						// Second call: verify subagent response was received
						if len(input) > 0 {
							if funcResp, ok := input[0].(gollem.FunctionResponse); ok {
								if funcResp.Name == "helper" {
									subagentCalled = true
									// Check the response from subagent
									if resp, ok := funcResp.Data["response"]; ok {
										gt.Equal(t, "Child agent response", resp)
									}
									if status, ok := funcResp.Data["status"]; ok {
										gt.Equal(t, "success", status)
									}
								}
							}
						}

						return &gollem.Response{
							Texts: []string{"Task completed"},
						}, nil
					},
				}, nil
			},
		}

		parentAgent := gollem.New(
			parentClient,
			gollem.WithSubAgents(subagent),
			gollem.WithLoopLimit(5),
		)

		result, err := parentAgent.Execute(context.Background(), gollem.Text("Do something"))
		gt.NoError(t, err)
		gt.NotNil(t, result)
		gt.True(t, subagentCalled)
		gt.Equal(t, 2, callCount)
	})
}

func TestNestedSubAgents(t *testing.T) {
	t.Run("subagent can have its own subagent", func(t *testing.T) {
		// Create grandchild agent
		grandchildClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						return &gollem.Response{
							Texts: []string{"Grandchild response"},
						}, nil
					},
				}, nil
			},
		}
		grandchildAgent := gollem.New(grandchildClient)
		grandchildSubagent := gollem.NewSubAgent("grandchild", "Grandchild helper", grandchildAgent)

		// Create child agent with grandchild as subagent
		childCallCount := 0
		childClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						childCallCount++
						if childCallCount == 1 {
							// Call grandchild
							return &gollem.Response{
								FunctionCalls: []*gollem.FunctionCall{
									{
										ID:   "call_gc",
										Name: "grandchild",
										Arguments: map[string]any{
											"query": "Help from grandchild",
										},
									},
								},
							}, nil
						}
						return &gollem.Response{
							Texts: []string{"Child response with grandchild data"},
						}, nil
					},
				}, nil
			},
		}
		childAgent := gollem.New(childClient, gollem.WithSubAgents(grandchildSubagent), gollem.WithLoopLimit(5))
		childSubagent := gollem.NewSubAgent("child", "Child helper", childAgent)

		// Create parent agent with child as subagent
		parentCallCount := 0
		parentClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						parentCallCount++
						if parentCallCount == 1 {
							return &gollem.Response{
								FunctionCalls: []*gollem.FunctionCall{
									{
										ID:   "call_child",
										Name: "child",
										Arguments: map[string]any{
											"query": "Help from child",
										},
									},
								},
							}, nil
						}
						return &gollem.Response{
							Texts: []string{"Parent completed"},
						}, nil
					},
				}, nil
			},
		}

		parentAgent := gollem.New(
			parentClient,
			gollem.WithSubAgents(childSubagent),
			gollem.WithLoopLimit(5),
		)

		result, err := parentAgent.Execute(context.Background(), gollem.Text("Start"))
		gt.NoError(t, err)
		gt.NotNil(t, result)
		gt.Equal(t, "Parent completed", result.String())
	})
}

func TestWithSubAgentsOption(t *testing.T) {
	t.Run("multiple subagents", func(t *testing.T) {
		agent1 := newMockAgent("response1")
		agent2 := newMockAgent("response2")

		subagent1 := gollem.NewSubAgent("agent1", "First agent", agent1)
		subagent2 := gollem.NewSubAgent("agent2", "Second agent", agent2)

		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				cfg := gollem.NewSessionConfig(options...)
				tools := cfg.Tools()

				// Should have both subagents
				gt.Equal(t, 2, len(tools))

				toolNames := make(map[string]bool)
				for _, tool := range tools {
					toolNames[tool.Spec().Name] = true
				}
				gt.True(t, toolNames["agent1"])
				gt.True(t, toolNames["agent2"])

				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						return &gollem.Response{
							Texts: []string{"done"},
						}, nil
					},
				}, nil
			},
		}

		parentAgent := gollem.New(mockClient, gollem.WithSubAgents(subagent1, subagent2))
		_, err := parentAgent.Execute(context.Background(), gollem.Text("test"))
		gt.NoError(t, err)
	})

	t.Run("subagent with regular tools", func(t *testing.T) {
		childAgent := newMockAgent("child response")
		subagent := gollem.NewSubAgent("helper", "Helper agent", childAgent)

		regularTool := &mock.ToolMock{
			SpecFunc: func() gollem.ToolSpec {
				return gollem.ToolSpec{
					Name:        "regular_tool",
					Description: "A regular tool",
				}
			},
			RunFunc: func(ctx context.Context, args map[string]any) (map[string]any, error) {
				return map[string]any{"result": "ok"}, nil
			},
		}

		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				cfg := gollem.NewSessionConfig(options...)
				tools := cfg.Tools()

				// Should have both subagent and regular tool
				gt.Equal(t, 2, len(tools))

				toolNames := make(map[string]bool)
				for _, tool := range tools {
					toolNames[tool.Spec().Name] = true
				}
				gt.True(t, toolNames["helper"])
				gt.True(t, toolNames["regular_tool"])

				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						return &gollem.Response{
							Texts: []string{"done"},
						}, nil
					},
				}, nil
			},
		}

		parentAgent := gollem.New(
			mockClient,
			gollem.WithTools(regularTool),
			gollem.WithSubAgents(subagent),
		)
		_, err := parentAgent.Execute(context.Background(), gollem.Text("test"))
		gt.NoError(t, err)
	})
}

func TestPromptTemplateRender(t *testing.T) {
	t.Run("render simple template", func(t *testing.T) {
		pt, err := gollem.NewPromptTemplate(
			"Hello, {{.name}}!",
			map[string]*gollem.Parameter{
				"name": {Type: gollem.TypeString, Description: "Name"},
			},
		)
		gt.NoError(t, err)

		result, err := pt.Render(map[string]any{"name": "World"})
		gt.NoError(t, err)
		gt.Equal(t, "Hello, World!", result)
	})

	t.Run("render complex template", func(t *testing.T) {
		pt, err := gollem.NewPromptTemplate(
			"Analyze {{.code}} with focus on {{.focus}}",
			map[string]*gollem.Parameter{
				"code":  {Type: gollem.TypeString, Description: "Code"},
				"focus": {Type: gollem.TypeString, Description: "Focus"},
			},
		)
		gt.NoError(t, err)

		result, err := pt.Render(map[string]any{
			"code":  "func main() {}",
			"focus": "security",
		})
		gt.NoError(t, err)
		gt.Equal(t, "Analyze func main() {} with focus on security", result)
	})

	t.Run("render with missing variable uses zero value", func(t *testing.T) {
		pt, err := gollem.NewPromptTemplate(
			"Hello, {{.name}}!",
			map[string]*gollem.Parameter{
				"name": {Type: gollem.TypeString, Description: "Name"},
			},
		)
		gt.NoError(t, err)

		// missingkey=zero replaces missing variables with empty string
		result, err := pt.Render(map[string]any{})
		gt.NoError(t, err)
		gt.Equal(t, "Hello, !", result)
	})
}

func TestPromptTemplateParameters(t *testing.T) {
	params := map[string]*gollem.Parameter{
		"code":  {Type: gollem.TypeString, Description: "Code to analyze", Required: true},
		"focus": {Type: gollem.TypeString, Description: "Focus area"},
	}

	pt, err := gollem.NewPromptTemplate("{{.code}} {{.focus}}", params)
	gt.NoError(t, err)

	got := pt.Parameters()
	gt.Equal(t, 2, len(got))
	gt.Equal(t, "Code to analyze", got["code"].Description)
	gt.True(t, got["code"].Required)
	gt.Equal(t, "Focus area", got["focus"].Description)
}

func TestDefaultPromptTemplate(t *testing.T) {
	t.Run("returns valid template", func(t *testing.T) {
		pt := gollem.DefaultPromptTemplate()
		gt.NotNil(t, pt)

		params := pt.Parameters()
		gt.Equal(t, 1, len(params))

		queryParam, exists := params["query"]
		gt.True(t, exists)
		gt.Equal(t, gollem.TypeString, queryParam.Type)
		gt.True(t, queryParam.Required)
	})

	t.Run("render with query", func(t *testing.T) {
		pt := gollem.DefaultPromptTemplate()

		result, err := pt.Render(map[string]any{"query": "Hello, World!"})
		gt.NoError(t, err)
		gt.Equal(t, "Hello, World!", result)
	})

	t.Run("render without required query returns error", func(t *testing.T) {
		pt := gollem.DefaultPromptTemplate()

		// query is required, so missing it returns an error
		_, err := pt.Render(map[string]any{})
		gt.Error(t, err)
	})
}

func TestSubAgentWithSubAgentMiddleware(t *testing.T) {
	t.Run("middleware adds context to args", func(t *testing.T) {
		var capturedInput gollem.Input
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						if len(input) > 0 {
							capturedInput = input[0]
						}
						return &gollem.Response{
							Texts: []string{"done"},
						}, nil
					},
				}, nil
			},
		}
		childAgent := gollem.New(mockClient)

		prompt, err := gollem.NewPromptTemplate(
			"Query: {{.query}}, Time: {{.current_time}}, User: {{.user_name}}",
			map[string]*gollem.Parameter{
				"query": {Type: gollem.TypeString, Description: "Query", Required: true},
				// current_time and user_name are injected by middleware, not in LLM spec
			},
		)
		gt.NoError(t, err)

		subagent := gollem.NewSubAgent(
			"context_aware",
			"Context aware agent",
			childAgent,
			gollem.WithPromptTemplate(prompt),
			gollem.WithSubAgentMiddleware(func(next gollem.SubAgentHandler) gollem.SubAgentHandler {
				return func(ctx context.Context, args map[string]any) (map[string]any, error) {
					// Inject context that LLM doesn't provide
					args["current_time"] = "2024-01-01T12:00:00Z"
					args["user_name"] = "Alice"
					return next(ctx, args)
				}
			}),
		)

		result, err := subagent.Run(context.Background(), map[string]any{
			"query": "test query",
		})

		gt.NoError(t, err)
		gt.NotNil(t, result)
		gt.Equal(t, "success", result["status"])

		// Verify the injected context was used in the prompt
		text, ok := capturedInput.(gollem.Text)
		gt.True(t, ok)
		gt.Equal(t, gollem.Text("Query: test query, Time: 2024-01-01T12:00:00Z, User: Alice"), text)
	})

	t.Run("middleware can modify existing args", func(t *testing.T) {
		var capturedInput gollem.Input
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						if len(input) > 0 {
							capturedInput = input[0]
						}
						return &gollem.Response{
							Texts: []string{"done"},
						}, nil
					},
				}, nil
			},
		}
		childAgent := gollem.New(mockClient)

		prompt, err := gollem.NewPromptTemplate(
			"Password: {{.password}}",
			map[string]*gollem.Parameter{
				"password": {Type: gollem.TypeString, Description: "Password"},
			},
		)
		gt.NoError(t, err)

		subagent := gollem.NewSubAgent(
			"masked",
			"Masks sensitive data",
			childAgent,
			gollem.WithPromptTemplate(prompt),
			gollem.WithSubAgentMiddleware(func(next gollem.SubAgentHandler) gollem.SubAgentHandler {
				return func(ctx context.Context, args map[string]any) (map[string]any, error) {
					// Mask the password
					if _, ok := args["password"].(string); ok {
						args["password"] = "***"
					}
					return next(ctx, args)
				}
			}),
		)

		result, err := subagent.Run(context.Background(), map[string]any{
			"password": "secret123",
		})

		gt.NoError(t, err)
		gt.NotNil(t, result)

		// Verify the password was masked
		text, ok := capturedInput.(gollem.Text)
		gt.True(t, ok)
		gt.Equal(t, gollem.Text("Password: ***"), text)
	})
}

func TestSubAgentArgsMiddlewareChain(t *testing.T) {
	t.Run("multiple middlewares execute in order", func(t *testing.T) {
		var capturedInput gollem.Input
		var executionOrder []string

		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						if len(input) > 0 {
							capturedInput = input[0]
						}
						return &gollem.Response{
							Texts: []string{"done"},
						}, nil
					},
				}, nil
			},
		}
		childAgent := gollem.New(mockClient)

		prompt, err := gollem.NewPromptTemplate(
			"Value: {{.value}}",
			map[string]*gollem.Parameter{
				"value": {Type: gollem.TypeString, Description: "Value"},
			},
		)
		gt.NoError(t, err)

		subagent := gollem.NewSubAgent(
			"chained",
			"Chained middlewares",
			childAgent,
			gollem.WithPromptTemplate(prompt),
			// First middleware (executes first)
			gollem.WithSubAgentMiddleware(func(next gollem.SubAgentHandler) gollem.SubAgentHandler {
				return func(ctx context.Context, args map[string]any) (map[string]any, error) {
					executionOrder = append(executionOrder, "middleware1-before")
					args["value"] = args["value"].(string) + "-m1"
					result, err := next(ctx, args)
					executionOrder = append(executionOrder, "middleware1-after")
					return result, err
				}
			}),
			// Second middleware (executes second)
			gollem.WithSubAgentMiddleware(func(next gollem.SubAgentHandler) gollem.SubAgentHandler {
				return func(ctx context.Context, args map[string]any) (map[string]any, error) {
					executionOrder = append(executionOrder, "middleware2-before")
					args["value"] = args["value"].(string) + "-m2"
					result, err := next(ctx, args)
					executionOrder = append(executionOrder, "middleware2-after")
					return result, err
				}
			}),
		)

		result, err := subagent.Run(context.Background(), map[string]any{
			"value": "initial",
		})

		gt.NoError(t, err)
		gt.NotNil(t, result)

		// Verify middleware execution order
		gt.Equal(t, []string{
			"middleware1-before",
			"middleware2-before",
			"middleware2-after",
			"middleware1-after",
		}, executionOrder)

		// Verify both middlewares modified the value
		text, ok := capturedInput.(gollem.Text)
		gt.True(t, ok)
		gt.Equal(t, gollem.Text("Value: initial-m1-m2"), text)
	})
}

func TestSubAgentArgsMiddlewareError(t *testing.T) {
	t.Run("middleware error stops execution", func(t *testing.T) {
		agentCalled := false
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						agentCalled = true
						return &gollem.Response{
							Texts: []string{"done"},
						}, nil
					},
				}, nil
			},
		}
		childAgent := gollem.New(mockClient)

		subagent := gollem.NewSubAgent(
			"error_test",
			"Tests middleware errors",
			childAgent,
			gollem.WithSubAgentMiddleware(func(next gollem.SubAgentHandler) gollem.SubAgentHandler {
				return func(ctx context.Context, args map[string]any) (map[string]any, error) {
					// Return an error before calling next
					return nil, errors.New("middleware error")
				}
			}),
		)

		result, err := subagent.Run(context.Background(), map[string]any{
			"query": "test",
		})

		gt.Error(t, err)
		gt.Nil(t, result)
		gt.False(t, agentCalled)
		gt.S(t, err.Error()).Contains("middleware error")
	})

	t.Run("chained middleware error stops subsequent middlewares", func(t *testing.T) {
		middleware2Called := false

		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						return &gollem.Response{
							Texts: []string{"done"},
						}, nil
					},
				}, nil
			},
		}
		childAgent := gollem.New(mockClient)

		subagent := gollem.NewSubAgent(
			"error_chain",
			"Tests error in chain",
			childAgent,
			// First middleware returns error
			gollem.WithSubAgentMiddleware(func(next gollem.SubAgentHandler) gollem.SubAgentHandler {
				return func(ctx context.Context, args map[string]any) (map[string]any, error) {
					return nil, errors.New("first middleware error")
				}
			}),
			// Second middleware should not be called
			gollem.WithSubAgentMiddleware(func(next gollem.SubAgentHandler) gollem.SubAgentHandler {
				return func(ctx context.Context, args map[string]any) (map[string]any, error) {
					middleware2Called = true
					return next(ctx, args)
				}
			}),
		)

		result, err := subagent.Run(context.Background(), map[string]any{
			"query": "test",
		})

		gt.Error(t, err)
		gt.Nil(t, result)
		gt.False(t, middleware2Called)
	})
}

func TestSubAgentArgsMiddlewareNil(t *testing.T) {
	t.Run("no middleware preserves backward compatibility", func(t *testing.T) {
		var capturedInput gollem.Input
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						if len(input) > 0 {
							capturedInput = input[0]
						}
						return &gollem.Response{
							Texts: []string{"Processed"},
						}, nil
					},
				}, nil
			},
		}
		childAgent := gollem.New(mockClient)

		// Create subagent without middleware (backward compatible)
		subagent := gollem.NewSubAgent("processor", "Processes queries", childAgent)

		result, err := subagent.Run(context.Background(), map[string]any{
			"query": "Process this text",
		})

		gt.NoError(t, err)
		gt.NotNil(t, result)
		gt.Equal(t, "success", result["status"])
		gt.Equal(t, "Processed", result["response"])

		// Verify the query was passed to the child agent
		text, ok := capturedInput.(gollem.Text)
		gt.True(t, ok)
		gt.Equal(t, gollem.Text("Process this text"), text)
	})

	t.Run("template mode without middleware still works", func(t *testing.T) {
		var capturedInput gollem.Input
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						if len(input) > 0 {
							capturedInput = input[0]
						}
						return &gollem.Response{
							Texts: []string{"Analysis complete"},
						}, nil
					},
				}, nil
			},
		}
		childAgent := gollem.New(mockClient)

		prompt, err := gollem.NewPromptTemplate(
			"Code: {{.code}}, Focus: {{.focus}}",
			map[string]*gollem.Parameter{
				"code":  {Type: gollem.TypeString, Description: "Code"},
				"focus": {Type: gollem.TypeString, Description: "Focus"},
			},
		)
		gt.NoError(t, err)

		// Template mode without middleware
		subagent := gollem.NewSubAgent(
			"analyzer",
			"Code analyzer",
			childAgent,
			gollem.WithPromptTemplate(prompt),
		)

		result, err := subagent.Run(context.Background(), map[string]any{
			"code":  "func main() {}",
			"focus": "security",
		})

		gt.NoError(t, err)
		gt.NotNil(t, result)
		gt.Equal(t, "success", result["status"])

		text, ok := capturedInput.(gollem.Text)
		gt.True(t, ok)
		gt.Equal(t, gollem.Text("Code: func main() {}, Focus: security"), text)
	})
}
