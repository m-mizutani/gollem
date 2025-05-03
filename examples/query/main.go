package main

import (
	"context"
	"fmt"
	"os"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/claude"
	"github.com/m-mizutani/gollem/llm/gemini"
	"github.com/m-mizutani/gollem/llm/openai"
)

func main() {
	ctx := context.Background()

	if len(os.Args) != 4 {
		fmt.Println("Usage: go run main.go <gemini|claude|OpenAI> <model_name> <prompt>")
		os.Exit(1)
	}

	llm := os.Args[1]
	model := os.Args[2]
	prompt := os.Args[3]

	var client gollem.LLMClient

	switch llm {
	case "gemini":
		c, err := gemini.New(ctx, os.Getenv("GEMINI_PROJECT_ID"), os.Getenv("GEMINI_LOCATION"), gemini.WithModel(model))
		if err != nil {
			panic(err)
		}
		client = c

	case "claude":
		c, err := claude.New(ctx, os.Getenv("ANTHROPIC_API_KEY"), claude.WithModel(model))
		if err != nil {
			panic(err)
		}
		client = c

	case "OpenAI":
		c, err := openai.New(ctx, os.Getenv("OPENAI_API_KEY"), openai.WithModel(model))
		if err != nil {
			panic(err)
		}
		client = c
	}

	ssn, err := client.NewSession(ctx)
	if err != nil {
		panic(err)
	}

	result, err := ssn.GenerateContent(ctx, gollem.Text(prompt))
	if err != nil {
		panic(err)
	}

	fmt.Println(result.Texts)
}
