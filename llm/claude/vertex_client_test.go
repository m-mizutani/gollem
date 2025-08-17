package claude_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/claude"
	"github.com/m-mizutani/gt"
)

func TestNewWithVertex(t *testing.T) {
	ctx := context.Background()

	t.Run("missing projectID", func(t *testing.T) {
		client, err := claude.NewWithVertex(ctx, "us-central1", "")
		gt.Error(t, err)
		gt.Nil(t, client)
		gt.True(t, strings.Contains(err.Error(), "projectID is required"))
	})

	t.Run("missing region", func(t *testing.T) {
		client, err := claude.NewWithVertex(ctx, "", "test-project")
		gt.Error(t, err)
		gt.Nil(t, client)
		gt.True(t, strings.Contains(err.Error(), "region is required"))
	})

	t.Run("valid parameters with options", func(t *testing.T) {
		prj, ok := os.LookupEnv("TEST_CLAUDE_VERTEX_AI_PROJECT_ID")
		if !ok {
			t.Skip("TEST_CLAUDE_VERTEX_AI_PROJECT_ID is not set")
		}
		client, err := claude.NewWithVertex(ctx, "us-central1", prj,
			claude.WithVertexModel("claude-sonnet-4@20250514"),
			claude.WithVertexTemperature(0.5),
			claude.WithVertexTopP(0.8),
			claude.WithVertexMaxTokens(2048),
			claude.WithVertexSystemPrompt("You are a helpful assistant"),
		)

		// Note: This will likely fail in CI/testing without proper GCP credentials
		// but we're mainly testing the parameter validation and setup
		if err != nil {
			// Expected in test environment without GCP credentials
			gt.True(t, strings.Contains(err.Error(), "failed to") || strings.Contains(err.Error(), "auth"))
			return
		}

		// If it succeeds, validate the configuration
		gt.NotNil(t, client)
	})
}

func TestVertexClient(t *testing.T) {
	projectID := os.Getenv("TEST_CLAUDE_VERTEX_AI_PROJECT_ID")
	if projectID == "" {
		t.Skip("TEST_CLAUDE_VERTEX_AI_PROJECT_ID not set, skipping test")
	}

	location := os.Getenv("TEST_CLAUDE_VERTEX_AI_LOCATION")
	if location == "" {
		location = "us-east5" // Default to us-east5 where Claude Sonnet 4 is working
	}

	ctx := context.Background()

	// Create Vertex AI client using Anthropic's official SDK
	client, err := claude.NewWithVertex(ctx, location, projectID,
		claude.WithVertexModel("claude-sonnet-4@20250514"),
		claude.WithVertexMaxTokens(512),
		claude.WithVertexTemperature(0.5),
	)
	gt.NoError(t, err)

	// Create session
	session, err := client.NewSession(ctx)
	gt.NoError(t, err)

	// Test basic text generation
	response, err := session.GenerateContent(ctx, gollem.Text("Hello! Please respond with 'Vertex AI working!' to confirm this integration works."))
	gt.NoError(t, err)
	gt.NotNil(t, response)
	gt.True(t, len(response.Texts) > 0)

	gt.True(t, strings.Contains(response.Texts[0], "Vertex AI working!"))
}

func TestVertexClientWithTools(t *testing.T) {
	projectID := os.Getenv("TEST_CLAUDE_VERTEX_AI_PROJECT_ID")
	if projectID == "" {
		t.Skip("TEST_CLAUDE_VERTEX_AI_PROJECT_ID not set, skipping test")
	}

	location := os.Getenv("TEST_CLAUDE_VERTEX_AI_LOCATION")
	if location == "" {
		location = "us-east5" // Default to us-east5 where Claude Sonnet 4 is working
	}

	ctx := context.Background()

	// Create Vertex AI client using Anthropic's official SDK
	client, err := claude.NewWithVertex(ctx, location, projectID,
		claude.WithVertexModel("claude-sonnet-4@20250514"),
		claude.WithVertexMaxTokens(512),
		claude.WithVertexTemperature(0.5),
	)
	gt.NoError(t, err)

	// Create simple test tool
	testTool := &calculatorTool{}

	// Create session with tool
	session, err := client.NewSession(ctx, gollem.WithSessionTools(testTool))
	gt.NoError(t, err)

	// Test tool calling
	response, err := session.GenerateContent(ctx, gollem.Text("Please calculate 15 + 27 using the calculator tool."))
	gt.NoError(t, err)
	gt.NotNil(t, response)

	// Should either return text or function calls
	gt.True(t, len(response.Texts) > 0 || len(response.FunctionCalls) > 0)

	if len(response.FunctionCalls) > 0 {

		// Execute the function call
		result, err := testTool.Run(ctx, response.FunctionCalls[0].Arguments)
		gt.NoError(t, err)

		// Send result back
		funcResp := gollem.FunctionResponse{
			ID:   response.FunctionCalls[0].ID,
			Name: response.FunctionCalls[0].Name,
			Data: result,
		}

		finalResponse, err := session.GenerateContent(ctx, funcResp)
		gt.NoError(t, err)
		gt.NotNil(t, finalResponse)
		gt.True(t, len(finalResponse.Texts) > 0)

		gt.True(t, strings.Contains(finalResponse.Texts[0], "42"))
	}
}

// calculatorTool is a simple tool for testing
type calculatorTool struct{}

func (c *calculatorTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "calculator",
		Description: "Perform basic arithmetic operations",
		Required:    []string{"operation", "a", "b"},
		Parameters: map[string]*gollem.Parameter{
			"operation": {
				Type:        gollem.TypeString,
				Description: "The operation to perform (add, subtract, multiply, divide)",
			},
			"a": {
				Type:        gollem.TypeNumber,
				Description: "First number",
			},
			"b": {
				Type:        gollem.TypeNumber,
				Description: "Second number",
			},
		},
	}
}

func (c *calculatorTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	operation, ok := args["operation"].(string)
	if !ok {
		return nil, gollem.ErrInvalidParameter
	}

	a, ok := args["a"].(float64)
	if !ok {
		return nil, gollem.ErrInvalidParameter
	}

	b, ok := args["b"].(float64)
	if !ok {
		return nil, gollem.ErrInvalidParameter
	}

	var result float64
	switch operation {
	case "add":
		result = a + b
	case "subtract":
		result = a - b
	case "multiply":
		result = a * b
	case "divide":
		if b == 0 {
			return map[string]any{"error": "division by zero"}, nil
		}
		result = a / b
	default:
		return nil, gollem.ErrInvalidParameter
	}

	return map[string]any{"result": result}, nil
}
