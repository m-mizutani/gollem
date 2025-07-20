package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/openai"
)

func main() {
	// Get OpenAI API key from environment variable
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	// Create OpenAI client
	llmClient, err := openai.New(context.Background(), apiKey)
	if err != nil {
		log.Fatal("Failed to create OpenAI client:", err)
	}

	// Create history compactor with custom options
	compactor := gollem.NewHistoryCompactor(llmClient,
		gollem.WithMaxTokens(10000),           // Start compaction at 10k tokens
		gollem.WithPreserveRecentTokens(3000)) // Preserve 3k tokens of recent context

	// Create agent with history compaction features
	agent := gollem.New(llmClient,
		gollem.WithHistoryCompactor(compactor),                     // Set compactor
		gollem.WithHistoryCompaction(true),                         // Enable history compaction
		gollem.WithCompactionHook(compactionHook),                  // Compaction event logging
		gollem.WithSystemPrompt("You are a helpful AI assistant."), // System prompt
	)

	ctx := context.Background()

	// Execute multiple conversations to accumulate history
	conversations := []string{
		"Hello! My name is John. Nice to meet you today.",
		"I'm from New York and work as a programmer. I'm proficient in Go language.",
		"Recently, I've been interested in developing applications using LLMs.",
		"I particularly want to learn about conversation history management.",
		"Can you tell me how to handle long conversation histories?",
		"Are there ways to reduce memory usage?",
		"Which is better - summarization or truncation?",
		"Do you have any experience running this in production?",
		"Are there any cost considerations I should be aware of?",
		"What are some tips for optimizing performance?",
	}

	fmt.Println("=== History Compaction Demo ===")
	fmt.Println("Compaction settings: MaxTokens=10000, PreserveRecentTokens=3000")
	fmt.Println()

	for i, prompt := range conversations {
		fmt.Printf("--- Conversation %d ---\n", i+1)
		fmt.Printf("User: %s\n", prompt)

		// History compaction may occur automatically
		err := agent.Execute(ctx, prompt)
		if err != nil {
			log.Printf("Error in conversation %d: %v", i+1, err)
			continue
		}

		// Display current history status
		if session := agent.Session(); session != nil {
			history := session.History()
			fmt.Printf("History status: %d messages", history.ToCount())
			if history.Compacted {
				fmt.Printf(" (compacted, original length: %d)", history.OriginalLen)
			}
			fmt.Println()
		}
		fmt.Println()
	}

	fmt.Println("=== Demo Complete ===")
}

// compactionHook is called when history compaction occurs
func compactionHook(ctx context.Context, original, compacted *gollem.History) error {
	compactionRatio := float64(compacted.ToCount()) / float64(original.ToCount())
	fmt.Printf("🗜️  History compaction executed: %d → %d messages (%.1f%% reduction)\n",
		original.ToCount(),
		compacted.ToCount(),
		(1-compactionRatio)*100)

	if compacted.Summary != "" {
		fmt.Printf("📄 Summary: %s\n", compacted.Summary)
	}

	return nil
}
