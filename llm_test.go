package gollam_test

import (
	"context"
	"os"
	"testing"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollam"
	"github.com/m-mizutani/gollam/llm/claude"
	"github.com/m-mizutani/gollam/llm/gemini"
	"github.com/m-mizutani/gollam/llm/gpt"
	"github.com/m-mizutani/gt"
)

// Sample tool implementation for testing
type randomNumberTool struct{}

func (t *randomNumberTool) Spec() gollam.ToolSpec {
	return gollam.ToolSpec{
		Name:        "random_number",
		Description: "A tool for generating random numbers within a specified range",
		Parameters: map[string]*gollam.Parameter{
			"min": {
				Type:        gollam.TypeNumber,
				Description: "Minimum value of the random number",
			},
			"max": {
				Type:        gollam.TypeNumber,
				Description: "Maximum value of the random number",
			},
		},
		Required: []string{"min", "max"},
	}
}

func (t *randomNumberTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	params, ok := args["params"].(map[string]any)
	if !ok {
		return nil, goerr.New("params is required")
	}

	min, ok := params["min"].(float64)
	if !ok {
		return nil, goerr.New("min is required")
	}

	max, ok := params["max"].(float64)
	if !ok {
		return nil, goerr.New("max is required")
	}

	if min >= max {
		return nil, goerr.New("min must be less than max")
	}

	// Note: In real implementation, you would use a proper random number generator
	// This is just for testing purposes
	result := (min + max) / 2

	return map[string]any{"result": result}, nil
}

func testLLM(t *testing.T, session gollam.Session) {
	ctx := t.Context()

	// Test case 1: Generate random number
	t.Run("generate random number", func(t *testing.T) {
		resp, err := session.Generate(ctx, gollam.Text("Please generate a random number between 1 and 10"))
		gt.NoError(t, err)
		gt.Array(t, resp.FunctionCalls).Length(1).Required()
		gt.Value(t, resp.FunctionCalls[0].Name).Equal("random_number")

		args := resp.FunctionCalls[0].Arguments
		gt.Value(t, args["min"]).Equal(1.0)
		gt.Value(t, args["max"]).Equal(10.0)

		resp, err = session.Generate(ctx, gollam.FunctionResponse{
			ID:   resp.FunctionCalls[0].ID,
			Name: "random_number",
			Data: map[string]any{"result": 5.5},
		})
		gt.NoError(t, err)
		gt.Array(t, resp.Texts).Length(1).Required()
		t.Log("Response:", resp.Texts[0])
	})

	// Test case 2: Generate random number with different range
	t.Run("generate random number with different range", func(t *testing.T) {
		resp, err := session.Generate(ctx, gollam.Text("Please generate a random number between 100 and 200"))
		gt.NoError(t, err)
		gt.Array(t, resp.FunctionCalls).Length(1).Required()
		gt.Value(t, resp.FunctionCalls[0].Name).Equal("random_number")

		args := resp.FunctionCalls[0].Arguments
		gt.Value(t, args["min"]).Equal(100.0)
		gt.Value(t, args["max"]).Equal(200.0)
	})
}

func TestGemini(t *testing.T) {
	var testProjectID, testLocation string
	v, ok := os.LookupEnv("TEST_GCP_PROJECT_ID")
	if !ok {
		t.Skip("TEST_GCP_PROJECT_ID is not set")
	} else {
		testProjectID = v
	}

	v, ok = os.LookupEnv("TEST_GCP_LOCATION")
	if !ok {
		t.Skip("TEST_GCP_LOCATION is not set")
	} else {
		testLocation = v
	}

	ctx := t.Context()
	client, err := gemini.New(ctx, testProjectID, testLocation)
	gt.NoError(t, err)

	// Setup tools
	tools := []gollam.Tool{&randomNumberTool{}}
	session, err := client.NewSession(ctx, tools)
	gt.NoError(t, err)

	testLLM(t, session)
}

func TestGPT(t *testing.T) {
	apiKey, ok := os.LookupEnv("TEST_OPENAI_API_KEY")
	if !ok {
		t.Skip("TEST_OPENAI_API_KEY is not set")
	}

	ctx := t.Context()
	client, err := gpt.New(ctx, apiKey)
	gt.NoError(t, err)

	// Setup tools
	tools := []gollam.Tool{&randomNumberTool{}}
	session, err := client.NewSession(ctx, tools)
	gt.NoError(t, err)

	testLLM(t, session)
}

func TestClaude(t *testing.T) {
	apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
	if !ok {
		t.Skip("TEST_CLAUDE_API_KEY is not set")
	}

	claudeClient, err := claude.New(context.Background(), apiKey)
	gt.NoError(t, err)

	session, err := claudeClient.NewSession(context.Background(), []gollam.Tool{&randomNumberTool{}})
	gt.NoError(t, err)

	testLLM(t, session)
}
