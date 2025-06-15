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

	// Create agent with tools
	agent := gollem.New(client,
		gollem.WithTools(tools...),
		gollem.WithSystemPrompt("You are a helpful calculator assistant. Use the available tools to perform mathematical operations."),
		gollem.WithMessageHook(func(ctx context.Context, msg string) error {
			log.Printf("🤖 %s", msg)
			return nil
		}),
		gollem.WithToolRequestHook(func(ctx context.Context, tool gollem.FunctionCall) error {
			log.Printf("⚡ Using tool: %s", tool.Name)
			return nil
		}),
	)

	query := "Add 5 and 3, then multiply the result by 2"
	log.Printf("📝 Query: %s", query)

	// Execute with automatic session management
	if err := agent.Execute(ctx, query); err != nil {
		log.Fatal(err)
	}

	log.Printf("✅ Calculation completed!")
}

// AddTool is a tool that adds two numbers
type AddTool struct{}

func (t *AddTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	a := args["a"].(float64)
	b := args["b"].(float64)
	result := a + b
	log.Printf("🔢 Add: %.2f + %.2f = %.2f", a, b, result)
	return map[string]any{"result": result}, nil
}

func (t *AddTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "add",
		Description: "Adds two numbers together",
		Parameters: map[string]*gollem.Parameter{
			"a": {
				Type:        gollem.TypeNumber,
				Description: "First number",
			},
			"b": {
				Type:        gollem.TypeNumber,
				Description: "Second number",
			},
		},
		Required: []string{"a", "b"},
	}
}

// MultiplyTool is a tool that multiplies two numbers
type MultiplyTool struct{}

func (t *MultiplyTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	a := args["a"].(float64)
	b := args["b"].(float64)
	result := a * b
	log.Printf("🔢 Multiply: %.2f × %.2f = %.2f", a, b, result)
	return map[string]any{"result": result}, nil
}

func (t *MultiplyTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "multiply",
		Description: "Multiplies two numbers together",
		Parameters: map[string]*gollem.Parameter{
			"a": {
				Type:        gollem.TypeNumber,
				Description: "First number",
			},
			"b": {
				Type:        gollem.TypeNumber,
				Description: "Second number",
			},
		},
		Required: []string{"a", "b"},
	}
}
