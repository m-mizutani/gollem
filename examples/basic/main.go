package main

import (
	"context"
	"fmt"
	"os"

	"github.com/m-mizutani/servantic"
	"github.com/m-mizutani/servantic/llm/gpt"
)

// HelloTool is a simple tool that returns a greeting
type HelloTool struct{}

func (t *HelloTool) Spec() *servantic.ToolSpec {
	return &servantic.ToolSpec{
		Name:        "hello",
		Description: "Returns a greeting",
		Parameters: map[string]*servantic.Parameter{
			"name": {
				Name:        "name",
				Type:        servantic.TypeString,
				Description: "Name of the person to greet",
				Required:    true,
			},
		},
	}
}

func (t *HelloTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return map[string]any{
		"message": fmt.Sprintf("Hello, %s!", args["name"]),
	}, nil
}

func main() {
	// Create GPT client
	client, err := gpt.New(context.Background(), os.Getenv("OPENAI_API_KEY"))
	if err != nil {
		panic(err)
	}

	// Create Servantic instance
	s := servantic.New(client,
		servantic.WithTools(&HelloTool{}),
	)

	// Send a message
	if err := s.Order(context.Background(), "Hello, my name is Taro."); err != nil {
		panic(err)
	}
}
