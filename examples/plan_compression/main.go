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

	// Configure history compression options for plan execution
	compressOptions := gollem.HistoryCompressionOptions{
		MaxMessages:    8, // Start compression at 8 messages
		PreserveRecent: 4, // Preserve 4 recent messages
	}

	// Create agent with plan compression features
	agent := gollem.New(llmClient,
		gollem.WithSystemPrompt("You are a helpful AI assistant that can create and execute plans."),
	)

	ctx := context.Background()

	// Create a plan with compression enabled
	plan, err := agent.Plan(ctx,
		"Create a comprehensive research plan about renewable energy sources. Include wind, solar, hydro, and geothermal energy. For each type, research current technology, efficiency rates, environmental impact, and cost analysis.",
		gollem.WithPlanHistoryCompressor(gollem.DefaultHistoryCompressor(llmClient, compressOptions)), // Set compressor with LLM and options
		gollem.WithPlanHistoryCompression(true),             // Enable history compression
		gollem.WithPlanCompressionHook(planCompressionHook), // Compression event logging
		gollem.WithPlanLanguage("English"),                  // Language preference
	)
	if err != nil {
		log.Fatal("Failed to create plan:", err)
	}

	fmt.Println("=== Plan Compression Demo ===")
	fmt.Printf("Plan created with %d todos\n", len(plan.GetToDos()))
	fmt.Printf("Compression settings: MaxMessages=%d, PreserveRecent=%d\n\n",
		compressOptions.MaxMessages,
		compressOptions.PreserveRecent)

	// Print initial plan
	fmt.Println("Initial Plan:")
	for i, todo := range plan.GetToDos() {
		fmt.Printf("%d. %s\n", i+1, todo.Description)
		fmt.Printf("   Intent: %s\n", todo.Intent)
	}
	fmt.Println()

	// Execute the plan - compression may occur automatically during execution
	fmt.Println("=== Executing Plan ===")
	result, err := plan.Execute(ctx)
	if err != nil {
		log.Printf("Plan execution failed: %v", err)
		return
	}

	fmt.Println("=== Plan Execution Complete ===")
	fmt.Printf("Final Result:\n%s\n\n", result)

	// Display final plan status
	finalTodos := plan.GetToDos()
	var completed, pending, failed, skipped int
	for _, todo := range finalTodos {
		switch todo.Status {
		case "Completed":
			completed++
		case "Pending":
			pending++
		case "Failed":
			failed++
		case "Skipped":
			skipped++
		}
	}

	fmt.Printf("Final Status: %d completed, %d pending, %d failed, %d skipped\n",
		completed, pending, failed, skipped)

	// Display session history status
	if session := plan.Session(); session != nil {
		history := session.History()
		fmt.Printf("Session History: %d messages", history.ToCount())
		if history.Compressed {
			fmt.Printf(" (compressed, original length: %d)", history.OriginalLen)
		}
		fmt.Println()
	}
}

// planCompressionHook is called when history compression occurs during plan execution
func planCompressionHook(ctx context.Context, original, compressed *gollem.History) error {
	compressionRatio := float64(compressed.ToCount()) / float64(original.ToCount())
	fmt.Printf("🗜️  Plan history compression executed: %d → %d messages (%.1f%% reduction)\n",
		original.ToCount(),
		compressed.ToCount(),
		(1-compressionRatio)*100)

	if compressed.Summary != "" {
		fmt.Printf("📄 Summary: %s\n", compressed.Summary)
	}

	return nil
}
