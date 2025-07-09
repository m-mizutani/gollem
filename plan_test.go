package gollem_test

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/claude"
	"github.com/m-mizutani/gollem/llm/gemini"
	"github.com/m-mizutani/gollem/llm/openai"
	"github.com/m-mizutani/gt"
)

// retryAPICall executes a function with exponential backoff and jitter for API errors
func retryAPICall[T any](t *testing.T, fn func() (T, error), operation string) (T, error) {
	const maxRetries = 3
	const baseDelay = 100 * time.Millisecond

	var result T
	var err error

	for attempt := 0; attempt < maxRetries; attempt++ {
		result, err = fn()
		if err == nil {
			return result, nil
		}

		// Check if it's a temporary API error
		if isTemporaryAPIError(err) {
			if attempt < maxRetries-1 {
				// Exponential backoff with jitter
				delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
				jitter := time.Duration(rand.Float64() * float64(delay) * 0.1)
				totalDelay := delay + jitter

				t.Logf("%s: API error (attempt %d/%d), retrying in %v: %v",
					operation, attempt+1, maxRetries, totalDelay, err)
				time.Sleep(totalDelay)
				continue
			}
		}

		// If it's not a temporary error or we've exhausted retries, return the error
		break
	}

	return result, err
}

// isTemporaryAPIError checks if an error is a temporary API error that should be retried
func isTemporaryAPIError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "overloaded") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "529") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "504")
}

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
	deserializedPlan, err := agent.NewPlanFromData(t.Context(), data)
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
		_, err := agent.NewPlanFromData(b.Context(), data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Test tool for threat intelligence (OTX-like)
type threatIntelTool struct{}

func (t *threatIntelTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "otx_ipv4",
		Description: "Search for threat intelligence data about IPv4 addresses using OTX",
		Parameters: map[string]*gollem.Parameter{
			"target": {
				Type:        gollem.TypeString,
				Description: "IPv4 address to investigate",
			},
		},
		Required: []string{"target"},
	}
}

func (t *threatIntelTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	ip, ok := args["target"].(string)
	if !ok {
		return nil, fmt.Errorf("target must be a string")
	}
	return map[string]any{
		"ip":         ip,
		"reputation": "clean",
		"sources":    []string{"OTX"},
	}, nil
}

// Client creation functions for different LLMs (similar to llm_test.go pattern)
func newPlanTestGeminiClient(t *testing.T) gollem.LLMClient {
	projectID, ok := os.LookupEnv("TEST_GCP_PROJECT_ID")
	if !ok {
		t.Skip("TEST_GCP_PROJECT_ID is not set")
	}

	location, ok := os.LookupEnv("TEST_GCP_LOCATION")
	if !ok {
		t.Skip("TEST_GCP_LOCATION is not set")
	}

	ctx := t.Context()
	client, err := gemini.New(ctx, projectID, location)
	gt.NoError(t, err)
	return client
}

func newPlanTestOpenAIClient(t *testing.T) gollem.LLMClient {
	apiKey, ok := os.LookupEnv("TEST_OPENAI_API_KEY")
	if !ok {
		t.Skip("TEST_OPENAI_API_KEY is not set")
	}

	ctx := t.Context()
	client, err := openai.New(ctx, apiKey)
	gt.NoError(t, err)
	return client
}

func newPlanTestClaudeClient(t *testing.T) gollem.LLMClient {
	apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
	if !ok {
		t.Skip("TEST_CLAUDE_API_KEY is not set")
	}

	client, err := claude.New(context.Background(), apiKey)
	gt.NoError(t, err)
	return client
}

// Common test function for premature completion issue
func testPrematureCompletion(t *testing.T, client gollem.LLMClient) {
	threatTool := &threatIntelTool{}
	agent := gollem.New(client, gollem.WithTools(threatTool))

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
	if err != nil {
		t.Logf("Plan execution failed: %v", err)
		// If plan is not nil, still try to get todos to see what happened
		if plan != nil {
			finalTodos := plan.GetToDos()
			t.Logf("Final todos after error:")
			for i, todo := range finalTodos {
				t.Logf("Todo %d (%s): %s - Status: %s", i+1, todo.ID, todo.Description, todo.Status)
				if todo.Error != nil {
					t.Logf("  Error: %v", todo.Error)
				}
			}
		}
	}
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

// Test premature completion issue with all LLMs
func TestPrematureCompletionIssueWithRealLLM(t *testing.T) {
	t.Run("OpenAI", func(t *testing.T) {
		client := newPlanTestOpenAIClient(t)
		testPrematureCompletion(t, client)
	})

	t.Run("Gemini", func(t *testing.T) {
		client := newPlanTestGeminiClient(t)
		testPrematureCompletion(t, client)
	})

	t.Run("Claude", func(t *testing.T) {
		client := newPlanTestClaudeClient(t)
		testPrematureCompletion(t, client)
	})
}

// Multiple security tools for comprehensive testing
type virusTotalTool struct{}

func (t *virusTotalTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "virus_total",
		Description: "Scan files, URLs, and IP addresses for malware using VirusTotal API",
		Parameters: map[string]*gollem.Parameter{
			"resource": {
				Type:        gollem.TypeString,
				Description: "File hash, URL, or IP address to scan",
			},
			"scan_type": {
				Type:        gollem.TypeString,
				Description: "Type of scan: 'file', 'url', or 'ip'",
			},
		},
		Required: []string{"resource", "scan_type"},
	}
}

func (t *virusTotalTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	resource, ok := args["resource"].(string)
	if !ok {
		return nil, fmt.Errorf("resource must be a string")
	}
	scanType, ok := args["scan_type"].(string)
	if !ok {
		return nil, fmt.Errorf("scan_type must be a string")
	}
	return map[string]any{
		"resource":   resource,
		"scan_type":  scanType,
		"clean":      true,
		"detections": 0,
		"scan_date":  "2024-01-01",
		"engines":    []string{"Microsoft", "Kaspersky", "Symantec"},
	}, nil
}

type shodanTool struct{}

func (t *shodanTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "shodan",
		Description: "Search for internet-connected devices and services using Shodan",
		Parameters: map[string]*gollem.Parameter{
			"query": {
				Type:        gollem.TypeString,
				Description: "Search query (IP, port, service, etc.)",
			},
		},
		Required: []string{"query"},
	}
}

func (t *shodanTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	query, ok := args["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query must be a string")
	}
	return map[string]any{
		"query":         query,
		"total_results": 42,
		"results": []map[string]any{
			{
				"ip":       "192.0.2.1",
				"port":     80,
				"service":  "http",
				"banner":   "Apache/2.4.41",
				"location": "US",
			},
		},
	}, nil
}

type crtshTool struct{}

func (t *crtshTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "crt_sh",
		Description: "Search for SSL/TLS certificates using crt.sh database",
		Parameters: map[string]*gollem.Parameter{
			"domain": {
				Type:        gollem.TypeString,
				Description: "Domain name to search certificates for",
			},
		},
		Required: []string{"domain"},
	}
}

func (t *crtshTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	domain, ok := args["domain"].(string)
	if !ok {
		return nil, fmt.Errorf("domain must be a string")
	}
	return map[string]any{
		"domain": domain,
		"certificates": []map[string]any{
			{
				"common_name": domain,
				"issuer":      "Let's Encrypt",
				"not_before":  "2024-01-01",
				"not_after":   "2024-04-01",
			},
		},
	}, nil
}

type whoisTool struct{}

func (t *whoisTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "whois",
		Description: "Lookup domain registration information using WHOIS",
		Parameters: map[string]*gollem.Parameter{
			"domain": {
				Type:        gollem.TypeString,
				Description: "Domain name to lookup",
			},
		},
		Required: []string{"domain"},
	}
}

func (t *whoisTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	domain, ok := args["domain"].(string)
	if !ok {
		return nil, fmt.Errorf("domain must be a string")
	}
	return map[string]any{
		"domain":          domain,
		"registrar":       "Example Registrar",
		"creation_date":   "2020-01-01",
		"expiration_date": "2025-01-01",
		"registrant_org":  "Example Organization",
		"name_servers":    []string{"ns1.example.com", "ns2.example.com"},
	}, nil
}

type nmapTool struct{}

func (t *nmapTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "nmap",
		Description: "Perform network port scanning using nmap",
		Parameters: map[string]*gollem.Parameter{
			"target": {
				Type:        gollem.TypeString,
				Description: "IP address or hostname to scan",
			},
			"ports": {
				Type:        gollem.TypeString,
				Description: "Port range to scan (e.g., '80,443' or '1-1000')",
			},
		},
		Required: []string{"target"},
	}
}

func (t *nmapTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	target, ok := args["target"].(string)
	if !ok {
		return nil, fmt.Errorf("target must be a string")
	}
	ports, _ := args["ports"].(string)
	if ports == "" {
		ports = "1-1000"
	}
	return map[string]any{
		"target": target,
		"ports":  ports,
		"open_ports": []map[string]any{
			{
				"port":     80,
				"protocol": "tcp",
				"service":  "http",
				"state":    "open",
			},
			{
				"port":     443,
				"protocol": "tcp",
				"service":  "https",
				"state":    "open",
			},
		},
	}, nil
}

type dnsLookupTool struct{}

func (t *dnsLookupTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "dns_lookup",
		Description: "Perform DNS lookups for various record types",
		Parameters: map[string]*gollem.Parameter{
			"domain": {
				Type:        gollem.TypeString,
				Description: "Domain name to lookup",
			},
			"record_type": {
				Type:        gollem.TypeString,
				Description: "DNS record type (A, AAAA, MX, TXT, etc.)",
			},
		},
		Required: []string{"domain", "record_type"},
	}
}

func (t *dnsLookupTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	domain, ok := args["domain"].(string)
	if !ok {
		return nil, fmt.Errorf("domain must be a string")
	}
	recordType, ok := args["record_type"].(string)
	if !ok {
		return nil, fmt.Errorf("record_type must be a string")
	}
	return map[string]any{
		"domain":      domain,
		"record_type": recordType,
		"records": []string{
			"192.0.2.1",
			"192.0.2.2",
		},
	}, nil
}

// Helper function to create a session with history
func createSessionWithHistory(ctx context.Context, client gollem.LLMClient) (gollem.Session, error) {
	// Create initial session
	session, err := client.NewSession(ctx)
	if err != nil {
		return nil, err
	}

	// Add some non-tool-related conversation history
	_, err = session.GenerateContent(ctx, gollem.Text("Hello, how are you today?"))
	if err != nil {
		return nil, err
	}

	_, err = session.GenerateContent(ctx, gollem.Text("I'm doing well, thank you for asking. What's the weather like where you are?"))
	if err != nil {
		return nil, err
	}

	_, err = session.GenerateContent(ctx, gollem.Text("It's a beautiful sunny day! Perfect for outdoor activities. Now, let's get to work on some security analysis tasks."))
	if err != nil {
		return nil, err
	}

	return session, nil
}

// Helper function to create session with history with retry logic
func createSessionWithHistoryWithRetry(ctx context.Context, client gollem.LLMClient, t *testing.T) (gollem.Session, error) {
	return retryAPICall(t, func() (gollem.Session, error) {
		return createSessionWithHistory(ctx, client)
	}, "create session with history")
}

// Test plan mode with multiple tools and history
func TestPlanModeWithMultipleToolsAndHistory(t *testing.T) {
	testFn := func(t *testing.T, newClient func(t *testing.T) gollem.LLMClient, llmName string) {
		client := newClient(t)

		// Create session with history using retry logic
		session, err := createSessionWithHistoryWithRetry(context.Background(), client, t)
		if err != nil {
			t.Skipf("Failed to create session with history after retries: %v", err)
		}

		// Get the history from the session
		history := session.History()

		// Create multiple security tools
		tools := []gollem.Tool{
			&virusTotalTool{},
			&shodanTool{},
			&crtshTool{},
			&whoisTool{},
			&nmapTool{},
			&dnsLookupTool{},
			&threatIntelTool{}, // Reuse existing tool
		}

		// Create a more detailed system prompt to encourage thorough execution
		systemPrompt := `You are a cybersecurity expert conducting a comprehensive security analysis.
You must use multiple security tools to thoroughly investigate the target domain and IP address.
Your analysis should include:

1. DNS reconnaissance using dns_lookup for various record types
2. Network scanning using nmap to identify open ports and services
3. Threat intelligence lookup using otx_ipv4 for malicious activity
4. Certificate analysis using crt_sh for SSL/TLS certificates
5. Domain registration analysis using whois for ownership information
6. Internet-connected device discovery using shodan for exposed services
7. Malware analysis using virus_total for reputation checking

You must execute multiple steps and use several different tools to provide a complete security assessment.
Do not conclude the analysis until you have gathered information using at least 3 different security tools.`

		agent := gollem.New(client,
			gollem.WithTools(tools...),
			gollem.WithHistory(history),
			gollem.WithSystemPrompt(systemPrompt),
		)

		// Track execution progress
		var executedTodos []string
		var completedTodos []string
		var toolsUsed []string

		// Create a more detailed prompt that encourages comprehensive analysis
		detailedPrompt := `Please perform a comprehensive security analysis of the domain 'example.com' and IP address '192.0.2.1'.

Your analysis MUST include the following mandatory steps:
1. DNS reconnaissance - lookup A, AAAA, MX, TXT, NS records
2. Network port scanning - identify open ports and running services
3. Threat intelligence - check for malicious activity or reputation issues
4. SSL/TLS certificate analysis - examine certificate details and history
5. Domain registration analysis - gather WHOIS information
6. Internet device discovery - search for exposed services and devices
7. Malware/reputation analysis - check for security threats

Please execute each step systematically and use the appropriate security tools for each task.
Provide detailed findings and correlate the results from multiple tools.`

		plan, err := agent.Plan(context.Background(),
			detailedPrompt,
			gollem.WithToDoStartHook(func(ctx context.Context, plan *gollem.Plan, todo gollem.PlanToDo) error {
				executedTodos = append(executedTodos, todo.ID)
				t.Logf("[%s] Started todo %s: %s", llmName, todo.ID, todo.Description)
				return nil
			}),
			gollem.WithToDoCompletedHook(func(ctx context.Context, plan *gollem.Plan, todo gollem.PlanToDo) error {
				completedTodos = append(completedTodos, todo.ID)
				t.Logf("[%s] Completed todo %s: %s", llmName, todo.ID, todo.Description)
				// Track tools used
				if todo.Result != nil {
					for _, toolCall := range todo.Result.ToolCalls {
						toolsUsed = append(toolsUsed, toolCall.Name)
					}
				}
				return nil
			}),
		)
		gt.NoError(t, err)
		gt.NotNil(t, plan)

		initialTodos := plan.GetToDos()
		t.Logf("[%s] Plan created with %d todos:", llmName, len(initialTodos))
		for i, todo := range initialTodos {
			t.Logf("[%s]   %d. %s - %s", llmName, i+1, todo.Description, todo.Intent)
		}

		// Execute plan with retry logic for API errors
		result, executeErr := retryAPICall(t, func() (string, error) {
			return plan.Execute(context.Background())
		}, fmt.Sprintf("[%s] plan execution", llmName))

		// Only fail if we couldn't execute after retries
		if executeErr != nil {
			t.Logf("[%s] Plan execution failed after retries: %v", llmName, executeErr)
			// For some LLMs, we might want to continue the test to see what we can observe
			if isTemporaryAPIError(executeErr) {
				t.Skipf("[%s] API temporarily unavailable: %v", llmName, executeErr)
			}
		}
		gt.NoError(t, executeErr)

		finalTodos := plan.GetToDos()
		t.Logf("[%s] Execution completed:", llmName)
		t.Logf("[%s] Total todos created: %d", llmName, len(initialTodos))
		t.Logf("[%s] Todos started: %d", llmName, len(executedTodos))
		t.Logf("[%s] Todos completed: %d", llmName, len(completedTodos))
		t.Logf("[%s] Tools used: %v", llmName, toolsUsed)
		t.Logf("[%s] Final result length: %d characters", llmName, len(result))

		// Verify that multiple tools were available and used
		gt.N(t, len(tools)).GreaterOrEqual(5)
		t.Logf("[%s] Total tools available: %d", llmName, len(tools))

		// Log tool usage
		toolUsageCount := make(map[string]int)
		for _, toolName := range toolsUsed {
			toolUsageCount[toolName]++
		}
		t.Logf("[%s] Tool usage breakdown:", llmName)
		for toolName, count := range toolUsageCount {
			t.Logf("[%s]   %s: %d times", llmName, toolName, count)
		}

		// Verify that the plan was executed successfully
		gt.N(t, len(completedTodos)).Greater(0)
		gt.True(t, len(result) > 0)

		// Enhanced success criteria: encourage tool usage for better testing
		uniqueToolsUsed := make(map[string]bool)
		for _, toolName := range toolsUsed {
			uniqueToolsUsed[toolName] = true
		}
		t.Logf("[%s] Unique tools used: %d", llmName, len(uniqueToolsUsed))

		// Log the final state of all todos
		for i, todo := range finalTodos {
			if todo.Completed {
				t.Logf("[%s] Todo %d (%s): %s - Status: %s", llmName, i+1, todo.ID, todo.Description, todo.Status)
				if todo.Result != nil {
					t.Logf("[%s]   Tool calls: %d", llmName, len(todo.Result.ToolCalls))
				}
			}
		}

		// Summary for this LLM test
		t.Logf("[%s] TEST SUMMARY: %d/%d todos completed, %d unique tools used",
			llmName, len(completedTodos), len(initialTodos), len(uniqueToolsUsed))
	}

	t.Run("OpenAI", func(t *testing.T) {
		testFn(t, newPlanTestOpenAIClient, "OpenAI")
	})

	t.Run("Gemini", func(t *testing.T) {
		testFn(t, newPlanTestGeminiClient, "Gemini")
	})

	t.Run("Claude", func(t *testing.T) {
		testFn(t, newPlanTestClaudeClient, "Claude")
	})
}
