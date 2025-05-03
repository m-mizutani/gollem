package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/gemini"
)

// WeatherTool is a simple tool that returns a weather
type WeatherTool struct{}

func (t *WeatherTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "weather",
		Description: "Returns a weather",
		Parameters: map[string]*gollem.Parameter{
			"city": {
				Type:        gollem.TypeString,
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

	g := gollem.New(llmModel,
		gollem.WithResponseMode(gollem.ResponseModeStreaming),
		gollem.WithTools(&WeatherTool{}),
		gollem.WithMessageHook(func(ctx context.Context, msg string) error {
			fmt.Printf("%s", msg)
			return nil
		}),
		gollem.WithToolRequestHook(func(ctx context.Context, tool gollem.FunctionCall) error {
			fmt.Printf("⚡ Call: %s\n", tool.Name)
			return nil
		}),
	)

	tmpFile, err := os.CreateTemp("", "gollem-chat-*.txt")
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
		newHistory, err := g.Prompt(ctx, text, gollem.WithHistory(history))
		if err != nil {
			panic(err)
		}

		if err := dumpHistory(newHistory, tmpFile.Name()); err != nil {
			panic(err)
		}

		fmt.Printf("\n")
	}
}

func dumpHistory(history *gollem.History, path string) error {
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

func loadHistory(path string) (*gollem.History, error) {
	if st, err := os.Stat(path); err != nil || st.Size() == 0 {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	var history gollem.History
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}

	return &history, nil
}
