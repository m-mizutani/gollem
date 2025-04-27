package gollam_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/m-mizutani/gollam"
	"github.com/m-mizutani/gollam/llm/claude"
	"github.com/m-mizutani/gollam/llm/gemini"
	"github.com/m-mizutani/gollam/llm/gpt"
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
					gollam.WithToolCallback(func(ctx context.Context, tool gollam.FunctionCall) error {
						toolCalled = true
						gt.Equal(t, tool.Name, "random_number")
						return nil
					}),
					gollam.WithResponseMode(respMode),
				)

				err = s.Order(t.Context(), "Generate a random number between 1 and 100.")
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
