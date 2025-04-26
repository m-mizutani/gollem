package main

import (
	"context"
	"fmt"
	"os"

	"github.com/m-mizutani/gollam"
	"github.com/m-mizutani/gollam/llm/gpt"
)

// HelloTool is a simple tool that returns a greeting
type HelloTool struct{}

func (t *HelloTool) Spec() *gollam.ToolSpec {
	return &gollam.ToolSpec{
		Name:        "hello",
		Description: "Returns a greeting",
		Parameters: map[string]*gollam.Parameter{
			"name": {
				Type:        gollam.TypeString,
				Description: "Name of the person to greet",
			},
		},
		Required: []string{"name"},
	}
}

func (t *HelloTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	name, ok := args["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name is required")
	}

	return map[string]any{
		"message": fmt.Sprintf("Hello, %s!", name),
	}, nil
}

func main() {
	// Create GPT client
	client, err := gpt.New(context.Background(), os.Getenv("OPENAI_API_KEY"))
	if err != nil {
		panic(err)
	}

	// Create gollam instance
	s := gollam.New(client,
		gollam.WithTools(&HelloTool{}),
	)

	// Send a message
	if err := s.Order(context.Background(), "Hello, my name is Taro."); err != nil {
		panic(err)
	}
}
