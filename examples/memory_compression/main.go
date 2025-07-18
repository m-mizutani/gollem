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

	// Configure history compression options
	compressOptions := gollem.HistoryCompressionOptions{
		MaxMessages:    10, // Start compression at 10 messages
		PreserveRecent: 5,  // Preserve 5 recent messages
	}

	// Create agent with history compression features
	agent := gollem.New(llmClient,
		gollem.WithHistoryCompressor(gollem.DefaultHistoryCompressor(llmClient)), // Set compressor with LLM for summarization
		gollem.WithHistoryCompression(true, compressOptions),                     // Enable history compression
		gollem.WithCompressionHook(compressionHook),                              // Compression event logging
		gollem.WithSystemPrompt("You are a helpful AI assistant."),               // System prompt
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

	fmt.Println("=== History Compression Demo ===")
	fmt.Printf("Compression settings: MaxMessages=%d, PreserveRecent=%d\n\n",
		compressOptions.MaxMessages,
		compressOptions.PreserveRecent)

	for i, prompt := range conversations {
		fmt.Printf("--- Conversation %d ---\n", i+1)
		fmt.Printf("User: %s\n", prompt)

		// History compression may occur automatically
		err := agent.Execute(ctx, prompt)
		if err != nil {
			log.Printf("Error in conversation %d: %v", i+1, err)
			continue
		}

		// Display current history status
		if session := agent.Session(); session != nil {
			history := session.History()
			fmt.Printf("History status: %d messages", history.ToCount())
			if history.Compressed {
				fmt.Printf(" (compressed, original length: %d)", history.OriginalLen)
			}
			fmt.Println()
		}
		fmt.Println()
	}

	fmt.Println("=== Demo Complete ===")
}

// compressionHook is called when history compression occurs
func compressionHook(ctx context.Context, original, compressed *gollem.History) error {
	compressionRatio := float64(compressed.ToCount()) / float64(original.ToCount())
	fmt.Printf("🗜️  History compression executed: %d → %d messages (%.1f%% reduction)\n",
		original.ToCount(),
		compressed.ToCount(),
		(1-compressionRatio)*100)

	if compressed.Summary != "" {
		fmt.Printf("📄 Summary: %s\n", compressed.Summary)
	}

	return nil
}
