package main

import (
	"bufio"
	"context"
	"encoding/json"
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

	tmpFile, err := os.CreateTemp("", "gollam-chat-*.txt")
	if err != nil {
		panic(err)
	}
	if err := tmpFile.Close(); err != nil {
		panic(err)
	}
	println("history file:", tmpFile.Name())

	for {
		history, err := loadHistory(tmpFile.Name())
		if err != nil {
			panic(err)
		}

		fmt.Print("> ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		text := scanner.Text()

		fmt.Printf("🤖 ")
		newHistory, err := g.Order(ctx, text, history)
		if err != nil {
			panic(err)
		}

		if err := dumpHistory(newHistory, tmpFile.Name()); err != nil {
			panic(err)
		}

		fmt.Printf("\n")
	}
}

func dumpHistory(history *gollam.History, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(history); err != nil {
		return err
	}

	return nil
}

func loadHistory(path string) (*gollam.History, error) {
	if st, err := os.Stat(path); err != nil || st.Size() == 0 {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var history gollam.History
	if err := json.NewDecoder(f).Decode(&history); err != nil {
		return nil, err
	}
	return &history, nil
}
