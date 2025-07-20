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

	// Create history compactor with custom options for plan execution
	compactor := gollem.NewHistoryCompactor(llmClient,
		gollem.WithMaxTokens(8000),            // Start compaction at 8k tokens
		gollem.WithPreserveRecentTokens(3000)) // Preserve 3k tokens of recent context

	// Create agent with plan compaction features
	agent := gollem.New(llmClient,
		gollem.WithSystemPrompt("You are a helpful AI assistant that can create and execute plans."),
	)

	ctx := context.Background()

	// Create a plan with compaction enabled
	plan, err := agent.Plan(ctx,
		"Create a comprehensive research plan about renewable energy sources. Include wind, solar, hydro, and geothermal energy. For each type, research current technology, efficiency rates, environmental impact, and cost analysis.",
		gollem.WithPlanHistoryCompactor(compactor),        // Set compactor
		gollem.WithPlanHistoryCompaction(true),            // Enable history compaction
		gollem.WithPlanCompactionHook(planCompactionHook), // Compaction event logging
		gollem.WithPlanLanguage("English"),                // Language preference
	)
	if err != nil {
		log.Fatal("Failed to create plan:", err)
	}

	fmt.Println("=== Plan Compaction Demo ===")
	fmt.Printf("Plan created with %d todos\n", len(plan.GetToDos()))
	fmt.Println("Compaction settings: MaxTokens=8000, PreserveRecentTokens=3000")
	fmt.Println()

	// Print initial plan
	fmt.Println("Initial Plan:")
	for i, todo := range plan.GetToDos() {
		fmt.Printf("%d. %s\n", i+1, todo.Description)
		fmt.Printf("   Intent: %s\n", todo.Intent)
	}
	fmt.Println()

	// Execute the plan - compaction may occur automatically during execution
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
		if history.Compacted {
			fmt.Printf(" (compacted, original length: %d)", history.OriginalLen)
		}
		fmt.Println()
	}
}

// planCompactionHook is called when history compaction occurs during plan execution
func planCompactionHook(ctx context.Context, original, compacted *gollem.History) error {
	compactionRatio := float64(compacted.ToCount()) / float64(original.ToCount())
	fmt.Printf("🗜️  Plan history compaction executed: %d → %d messages (%.1f%% reduction)\n",
		original.ToCount(),
		compacted.ToCount(),
		(1-compactionRatio)*100)

	if compacted.Summary != "" {
		fmt.Printf("📄 Summary: %s\n", compacted.Summary)
	}

	return nil
}
