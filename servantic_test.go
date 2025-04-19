package servantic_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/servantic"
	"github.com/m-mizutani/servantic/llm/gpt"
)

// RandomNumberTool is a tool that generates a random number within a specified range
type RandomNumberTool struct{}

func (t *RandomNumberTool) Spec() *servantic.ToolSpec {
	return &servantic.ToolSpec{
		Name:        "random_number",
		Description: "Generates a random number within a specified range",
		Parameters: map[string]*servantic.Parameter{
			"min": {
				Name:        "min",
				Type:        servantic.TypeNumber,
				Description: "Minimum value of the range",
				Required:    true,
			},
			"max": {
				Name:        "max",
				Type:        servantic.TypeNumber,
				Description: "Maximum value of the range",
				Required:    true,
			},
		},
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

func TestServanticWithTool(t *testing.T) {
	apiKey, ok := os.LookupEnv("TEST_OPENAI_API_KEY")
	if !ok {
		t.Skip("TEST_OPENAI_API_KEY is not set")
	}

	// Create GPT client
	client, err := gpt.New(context.Background(), apiKey)
	gt.NoError(t, err)

	toolCalled := false
	// Create Servantic instance
	s := servantic.New(client,
		servantic.WithTools(&RandomNumberTool{}),
		servantic.WithToolCallback(func(ctx context.Context, tool servantic.FunctionCall) error {
			toolCalled = true
			gt.Equal(t, tool.Name, "random_number")
			return nil
		}),
	)

	// Send a message
	err = s.Order(context.Background(), "Generate a random number between 1 and 100.")
	gt.NoError(t, err)
	gt.True(t, toolCalled)
}
