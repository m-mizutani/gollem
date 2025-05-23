package main

import (
	"context"
	"log"
	"os"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/gemini"
)

func main() {
	ctx := context.Background()

	// Initialize Gemini client
	client, err := gemini.New(ctx, os.Getenv("GEMINI_PROJECT_ID"), os.Getenv("GEMINI_LOCATION"))
	if err != nil {
		log.Fatal(err)
	}

	// Register tools
	tools := []gollem.Tool{
		&AddTool{},
		&MultiplyTool{},
	}

	servant := gollem.New(client,
		gollem.WithTools(tools...),
		gollem.WithMessageHook(func(ctx context.Context, msg string) error {
			log.Printf("Response: %s", msg)
			return nil
		}),
		/*
			gollem.WithLogger(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			}))),
		*/
	)

	query := "Add 5 and 3, then multiply the result by 2"
	log.Printf("Query: %s", query)
	if _, err := servant.Prompt(ctx, query); err != nil {
		log.Fatal(err)
	}
}

// AddTool is a tool that adds two numbers
type AddTool struct{}

func (t *AddTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	a := args["a"].(float64)
	b := args["b"].(float64)
	log.Printf("Add: %f + %f", a, b)
	return map[string]any{"result": a + b}, nil
}

func (t *AddTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "add",
		Description: "Adds two numbers together",
		Parameters: map[string]*gollem.Parameter{
			"a": {
				Type:        "number",
				Description: "First number",
			},
			"b": {
				Type:        "number",
				Description: "Second number",
			},
		},
	}
}

// MultiplyTool is a tool that multiplies two numbers
type MultiplyTool struct{}

func (t *MultiplyTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	a := args["a"].(float64)
	b := args["b"].(float64)
	log.Printf("Multiply: %f * %f", a, b)
	return map[string]any{"result": a * b}, nil
}

func (t *MultiplyTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "multiply",
		Description: "Multiplies two numbers together",
		Parameters: map[string]*gollem.Parameter{
			"a": {
				Type:        "number",
				Description: "First number",
			},
			"b": {
				Type:        "number",
				Description: "Second number",
			},
		},
	}
}
