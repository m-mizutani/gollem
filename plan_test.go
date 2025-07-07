package gollem_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
)

// Test tools for plan mode testing
type testSearchTool struct{}

func (t *testSearchTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "search",
		Description: "Search for information on the internet",
		Parameters: map[string]*gollem.Parameter{
			"query": {
				Type:        gollem.TypeString,
				Description: "Search query",
			},
		},
		Required: []string{"query"},
	}
}

func (t *testSearchTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	query, ok := args["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query must be a string")
	}

	return map[string]any{
		"results": fmt.Sprintf("Search results for: %s", query),
		"count":   3,
	}, nil
}

// Mock LLM client for unit tests
type mockLLMClient struct {
	responses []string
	callCount int
}

func (m *mockLLMClient) NewSession(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
	return &mockSession{
		client: m,
	}, nil
}

func (m *mockLLMClient) GenerateEmbedding(ctx context.Context, dimension int, input []string) ([][]float64, error) {
	return nil, fmt.Errorf("not implemented")
}

type mockSession struct {
	client *mockLLMClient
}

func (m *mockSession) GenerateContent(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
	if m.client.callCount >= len(m.client.responses) {
		return &gollem.Response{
			Texts: []string{"Default response"},
		}, nil
	}

	response := m.client.responses[m.client.callCount]
	m.client.callCount++

	return &gollem.Response{
		Texts: []string{response},
	}, nil
}

func (m *mockSession) GenerateStream(ctx context.Context, input ...gollem.Input) (<-chan *gollem.Response, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSession) History() *gollem.History {
	return nil
}

// Unit tests

func TestPlanCreation(t *testing.T) {
	mockClient := &mockLLMClient{
		responses: []string{
			`{"steps": [{"description": "First step", "intent": "Do first task"}, {"description": "Second step", "intent": "Do second task"}]}`,
		},
	}

	agent := gollem.New(mockClient, gollem.WithTools(&testSearchTool{}))

	plan, err := agent.Plan(context.Background(), "Test task")
	gt.NoError(t, err)
	gt.NotNil(t, plan)
}

func TestPlanSerialization(t *testing.T) {
	mockClient := &mockLLMClient{
		responses: []string{
			`{"steps": [{"description": "Test step", "intent": "Test intent"}]}`,
		},
	}

	agent := gollem.New(mockClient, gollem.WithTools(&testSearchTool{}))

	plan, err := agent.Plan(context.Background(), "Test task")
	gt.NoError(t, err)

	// Serialize
	data, err := plan.Serialize()
	gt.NoError(t, err)
	gt.True(t, len(data) > 0)

	// Deserialize
	deserializedPlan, err := agent.DeserializePlan(data)
	gt.NoError(t, err)
	gt.NotNil(t, deserializedPlan)
}

func TestPlanHooks(t *testing.T) {
	mockClient := &mockLLMClient{
		responses: []string{
			`{"steps": [{"description": "Test step", "intent": "Test intent"}]}`,
			"Step execution response",
			`{"should_continue": false, "response": "Task completed"}`,
		},
	}

	var hooksCalled []string

	agent := gollem.New(mockClient, gollem.WithTools(&testSearchTool{}))

	plan, err := agent.Plan(context.Background(), "Test task",
		gollem.WithPlanCreatedHook(func(ctx context.Context, plan *gollem.Plan) error {
			hooksCalled = append(hooksCalled, "created")
			return nil
		}),
		gollem.WithPlanCompletedHook(func(ctx context.Context, plan *gollem.Plan, result string) error {
			hooksCalled = append(hooksCalled, "completed")
			return nil
		}),
	)
	gt.NoError(t, err)

	_, err = plan.Execute(context.Background())
	gt.NoError(t, err)

	gt.Equal(t, []string{"created", "completed"}, hooksCalled)
}

func TestPlanAlreadyExecutedError(t *testing.T) {
	mockClient := &mockLLMClient{
		responses: []string{
			`{"steps": [{"description": "Test step", "intent": "Test intent"}]}`,
			"Step execution response",
			`{"should_continue": false, "response": "Task completed"}`,
		},
	}

	agent := gollem.New(mockClient, gollem.WithTools(&testSearchTool{}))

	plan, err := agent.Plan(context.Background(), "Test task")
	gt.NoError(t, err)

	// First execution should succeed
	_, err = plan.Execute(context.Background())
	gt.NoError(t, err)

	// Second execution should fail
	_, err = plan.Execute(context.Background())
	gt.Error(t, err)
	gt.Equal(t, gollem.ErrPlanAlreadyExecuted, err)
}

// Integration tests

func TestPlanModeIntegration(t *testing.T) {
	t.Skip("Integration tests require LLM API keys - run separately")
}

func TestMultiStepPlanExecution(t *testing.T) {
	t.Skip("Integration tests require LLM API keys - run separately")
}

func TestPlanWithHistory(t *testing.T) {
	t.Skip("Integration tests require LLM API keys - run separately")
}

func TestPlanErrorHandling(t *testing.T) {
	mockClient := &mockLLMClient{
		responses: []string{
			`invalid json response`,
		},
	}

	agent := gollem.New(mockClient, gollem.WithTools(&testSearchTool{}))

	_, err := agent.Plan(context.Background(), "Test task")
	gt.Error(t, err)
	gt.True(t, strings.Contains(err.Error(), "failed to parse plan"))
}

func TestPlanWithFacilitator(t *testing.T) {
	mockClient := &mockLLMClient{
		responses: []string{
			`{"steps": [{"description": "Test step", "intent": "Test intent"}]}`,
			"Step execution response",
			`{"should_continue": false, "response": "Task completed"}`,
		},
	}

	agent := gollem.New(mockClient, gollem.WithTools(&testSearchTool{}))

	plan, err := agent.Plan(context.Background(), "Test task")
	gt.NoError(t, err)

	result, err := plan.Execute(context.Background()) // Fixed: Execute not Run
	gt.NoError(t, err)
	gt.True(t, len(result) > 0)
}

func TestPlanWithCustomOptions(t *testing.T) {
	mockClient := &mockLLMClient{
		responses: []string{
			`{"steps": [{"description": "Test step", "intent": "Test intent"}]}`,
		},
	}

	agent := gollem.New(mockClient, gollem.WithTools(&testSearchTool{}))

	plan, err := agent.Plan(context.Background(), "Test task",
		gollem.WithMaxPlanSteps(5),
		gollem.WithReflectionEnabled(false),
		gollem.WithPlanSystemPrompt("Custom system prompt"))
	gt.NoError(t, err)
	gt.NotNil(t, plan)
}

// Benchmark tests

func BenchmarkPlanCreation(b *testing.B) {
	mockClient := &mockLLMClient{
		responses: []string{
			`{"steps": [{"description": "Test step", "intent": "Test intent"}]}`,
		},
	}

	agent := gollem.New(mockClient, gollem.WithTools(&testSearchTool{}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := agent.Plan(context.Background(), "Test task")
		if err != nil {
			b.Fatal(err)
		}
		// Reset mock client for next iteration
		mockClient.callCount = 0
	}
}

func BenchmarkPlanSerialization(b *testing.B) {
	mockClient := &mockLLMClient{
		responses: []string{
			`{"steps": [{"description": "Test step", "intent": "Test intent"}]}`,
		},
	}

	agent := gollem.New(mockClient, gollem.WithTools(&testSearchTool{}))
	plan, err := agent.Plan(context.Background(), "Test task")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := plan.Serialize()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPlanDeserialization(b *testing.B) {
	mockClient := &mockLLMClient{
		responses: []string{
			`{"steps": [{"description": "Test step", "intent": "Test intent"}]}`,
		},
	}

	agent := gollem.New(mockClient, gollem.WithTools(&testSearchTool{}))
	plan, err := agent.Plan(context.Background(), "Test task")
	if err != nil {
		b.Fatal(err)
	}

	data, err := plan.Serialize()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := agent.DeserializePlan(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
