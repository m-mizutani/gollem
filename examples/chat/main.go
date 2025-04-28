package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/m-mizutani/gollam"
	"github.com/m-mizutani/gollam/llm/gemini"
)

// WeatherTool is a simple tool that returns a weather
type WeatherTool struct{}

func (t *WeatherTool) Spec() gollam.ToolSpec {
	return gollam.ToolSpec{
		Name:        "weather",
		Description: "Returns a weather",
		Parameters: map[string]*gollam.Parameter{
			"city": {
				Type:        gollam.TypeString,
				Description: "City name",
			},
		},
		Required: []string{"city"},
	}
}

func (t *WeatherTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	city, ok := args["city"].(string)
	if !ok {
		return nil, fmt.Errorf("city is required")
	}

	return map[string]any{
		"message": fmt.Sprintf("The weather in %s is sunny.", city),
	}, nil
}

func main() {
	ctx := context.Background()
	llmModel, err := gemini.New(ctx, os.Getenv("GEMINI_PROJECT_ID"), os.Getenv("GEMINI_LOCATION"))
	if err != nil {
		panic(err)
	}

	g := gollam.New(llmModel,
		gollam.WithResponseMode(gollam.ResponseModeStreaming),
		gollam.WithTools(&WeatherTool{}),
		gollam.WithMsgCallback(func(ctx context.Context, msg string) error {
			fmt.Printf("%s", msg)
			return nil
		}),
		gollam.WithToolCallback(func(ctx context.Context, tool gollam.FunctionCall) error {
			fmt.Printf("⚡ Call: %s\n", tool.Name)
			return nil
		}),
	)

	var history *gollam.History
	for {
		fmt.Print("> ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		text := scanner.Text()

		fmt.Printf("🤖 ")
		newHistory, err := g.Order(ctx, text, history)
		if err != nil {
			panic(err)
		}
		history = newHistory
		fmt.Printf("\n")
	}
}
