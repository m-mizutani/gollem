package gollem_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/openai"
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
	deserializedPlan, err := agent.NewPlanFromData(data)
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
	for b.Loop() {
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
	for b.Loop() {
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
	for b.Loop() {
		_, err := agent.NewPlanFromData(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Test tool for threat intelligence (OTX-like)
type threatIntelTool struct{}

func (t *threatIntelTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "otx.ipv4",
		Description: "Search for threat intelligence data about IPv4 addresses using OTX",
		Parameters: map[string]*gollem.Parameter{
			"ip": {
				Type:        gollem.TypeString,
				Description: "IPv4 address to investigate",
			},
		},
		Required: []string{"ip"},
	}
}

func (t *threatIntelTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	ip, ok := args["ip"].(string)
	if !ok {
		return nil, fmt.Errorf("ip must be a string")
	}
	return map[string]any{
		"ip":         ip,
		"reputation": "clean",
		"sources":    []string{"OTX"},
	}, nil
}

// Reproduce Warren's premature completion issue with real LLM
func TestPrematureCompletionIssueWithRealLLM(t *testing.T) {
	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	// Import the actual OpenAI client
	openaiClient, err := openai.New(context.Background(), openaiKey)
	if err != nil {
		t.Fatalf("Failed to create OpenAI client: %v", err)
	}

	threatTool := &threatIntelTool{}
	agent := gollem.New(openaiClient, gollem.WithTools(threatTool))

	// Track execution progress
	var executedTodos []string
	var completedTodos []string

	plan, err := agent.Plan(context.Background(), "Investigate IP address 192.0.2.1 for security threats",
		gollem.WithToDoStartHook(func(ctx context.Context, plan *gollem.Plan, todo gollem.PlanToDo) error {
			executedTodos = append(executedTodos, todo.ID)
			t.Logf("Started todo %s: %s", todo.ID, todo.Description)
			return nil
		}),
		gollem.WithToDoCompletedHook(func(ctx context.Context, plan *gollem.Plan, todo gollem.PlanToDo) error {
			completedTodos = append(completedTodos, todo.ID)
			t.Logf("Completed todo %s: %s", todo.ID, todo.Description)
			return nil
		}),
	)
	gt.NoError(t, err)
	gt.NotNil(t, plan)

	initialTodos := plan.GetToDos()
	t.Logf("Plan created with %d todos:", len(initialTodos))
	for i, todo := range initialTodos {
		t.Logf("  %d. %s - %s", i+1, todo.Description, todo.Intent)
	}

	result, err := plan.Execute(context.Background())
	gt.NoError(t, err)

	finalTodos := plan.GetToDos()
	t.Logf("\nExecution completed:")
	t.Logf("Total todos created: %d", len(initialTodos))
	t.Logf("Todos started: %d", len(executedTodos))
	t.Logf("Todos completed: %d", len(completedTodos))
	t.Logf("Final result: %s", result)

	// Log the final state of all todos
	for i, todo := range finalTodos {
		t.Logf("Todo %d (%s): %s - Status: %s", i+1, todo.ID, todo.Description, todo.Status)
		if todo.Result != nil {
			t.Logf("  Output: %s", todo.Result.Output)
			t.Logf("  Tool calls: %d", len(todo.Result.ToolCalls))
		}
	}

	// This test is mainly for observation - we want to see if:
	// 1. LLM creates multiple todos but only executes some
	// 2. LLM doesn't use available tools when it should
	// 3. Reflection decides to complete early due to perceived tool unavailability

	// Check if we have the premature completion issue
	if len(initialTodos) > 1 && len(completedTodos) < len(initialTodos) {
		t.Logf("WARNING: Potential premature completion detected!")
		t.Logf("  Plan had %d todos but only %d were completed", len(initialTodos), len(completedTodos))

		// Check if any completed todo used tools
		toolsUsed := false
		for _, todo := range finalTodos {
			if todo.Completed && todo.Result != nil && len(todo.Result.ToolCalls) > 0 {
				toolsUsed = true
				break
			}
		}

		if !toolsUsed {
			t.Logf("WARNING: No tools were used despite threat intelligence tool being available!")
		}
	}
}
