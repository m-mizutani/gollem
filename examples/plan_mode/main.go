package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/openai"
)

// SearchTool simulates a search tool for research
type SearchTool struct{}

func (t *SearchTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "search",
		Description: "Search for information on the internet",
		Parameters: map[string]*gollem.Parameter{
			"query": {
				Type:        gollem.TypeString,
				Description: "Search query",
			},
		},
		Required: []string{"query"},
	}
}

func (t *SearchTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	query := args["query"].(string)

	// Simulate search results
	time.Sleep(500 * time.Millisecond) // Simulate API call

	return map[string]any{
		"results": fmt.Sprintf("Search results for: %s", query),
		"count":   5,
		"sources": []string{
			"https://example.com/article1",
			"https://example.com/article2",
			"https://example.com/article3",
		},
	}, nil
}

// AnalysisTool simulates an analysis tool
type AnalysisTool struct{}

func (t *AnalysisTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "analyze",
		Description: "Analyze data and generate insights",
		Parameters: map[string]*gollem.Parameter{
			"data": {
				Type:        gollem.TypeString,
				Description: "Data to analyze",
			},
			"type": {
				Type:        gollem.TypeString,
				Description: "Type of analysis (trend, sentiment, statistical)",
				Default:     "trend",
			},
		},
		Required: []string{"data"},
	}
}

func (t *AnalysisTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	data := args["data"].(string)
	analysisType := args["type"].(string)

	// Simulate analysis
	time.Sleep(300 * time.Millisecond)

	return map[string]any{
		"analysis": fmt.Sprintf("%s analysis of: %s", analysisType, data),
		"insights": []string{
			"Key insight 1",
			"Key insight 2",
			"Key insight 3",
		},
		"confidence": 0.85,
	}, nil
}

// ReportTool simulates a report generation tool
type ReportTool struct{}

func (t *ReportTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "generate_report",
		Description: "Generate a formatted report from analysis results",
		Parameters: map[string]*gollem.Parameter{
			"title": {
				Type:        gollem.TypeString,
				Description: "Report title",
			},
			"content": {
				Type:        gollem.TypeString,
				Description: "Report content",
			},
			"format": {
				Type:        gollem.TypeString,
				Description: "Report format (markdown, html, text)",
				Default:     "markdown",
			},
		},
		Required: []string{"title", "content"},
	}
}

func (t *ReportTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	title := args["title"].(string)
	content := args["content"].(string)
	format := args["format"].(string)

	// Simulate report generation
	time.Sleep(200 * time.Millisecond)

	report := fmt.Sprintf("# %s\n\n%s\n\nGenerated at: %s",
		title, content, time.Now().Format(time.RFC3339))

	return map[string]any{
		"report":       report,
		"format":       format,
		"word_count":   len(strings.Fields(report)),
		"generated_at": time.Now().Format(time.RFC3339),
	}, nil
}

// Helper functions for displaying todos
func displayToDoList(plan *gollem.Plan) {
	todos := plan.GetToDos()
	fmt.Printf("\n📋 Plan ToDos:\n")

	for _, todo := range todos {
		checkbox := "☐"
		description := todo.Description

		switch todo.Status {
		case "Completed":
			checkbox = "☑"
			description = fmt.Sprintf("\033[9m%s\033[0m", description) // Strike-through
		case "Executing":
			checkbox = "⟳"
		}

		fmt.Printf(" %s %s\n", checkbox, description)
	}
}

func displayProgress(plan *gollem.Plan) {
	todos := plan.GetToDos()
	total := len(todos)
	completed := 0
	pending := 0

	// Count todos by processing the returned data
	for _, todo := range todos {
		if todo.Completed {
			completed++
		} else if todo.Status == "Pending" {
			pending++
		}
	}

	fmt.Printf("\n📈 Progress: %d/%d completed", completed, total)
	if total > 0 {
		percentage := float64(completed) / float64(total) * 100
		fmt.Printf(" (%.1f%%)", percentage)
	}
	fmt.Printf(", %d pending\n", pending)

	if total > 0 {
		// Progress bar
		barLength := 30
		completedBars := int(float64(completed) / float64(total) * float64(barLength))

		fmt.Print("🔵 ")
		for i := range barLength {
			if i < completedBars {
				fmt.Print("█")
			} else {
				fmt.Print("░")
			}
		}
		fmt.Println()
	}
}

func main() {
	// Get OpenAI API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	// Create LLM client
	client, err := openai.New(context.Background(), apiKey)
	if err != nil {
		log.Fatal("Failed to create OpenAI client:", err)
	}

	// Create agent with tools
	agent := gollem.New(client,
		gollem.WithTools(&SearchTool{}, &AnalysisTool{}, &ReportTool{}),
	)

	fmt.Println("🚀 Plan Mode with Adaptive Skip Demo")
	fmt.Println("=====================================")

	// Demo 1: Complete Mode (no skipping)
	fmt.Println("\n📋 Demo 1: Complete Mode (Execute All Tasks)")
	fmt.Println("---------------------------------------------")

	completePlan, err := agent.Plan(context.Background(),
		"Research the latest trends in AI and machine learning, analyze the findings, and create a comprehensive report",
		gollem.WithPlanExecutionMode(gollem.PlanExecutionModeComplete),
		gollem.WithPlanCreatedHook(createPlanCreatedHook("Complete")),
		gollem.WithPlanToDoStartHook(createToDoStartHook()),
		gollem.WithPlanToDoCompletedHook(createToDoCompletedHook()),
		gollem.WithPlanToDoUpdatedHook(createToDoUpdatedHook()),
	)
	if err != nil {
		log.Fatal("Failed to create complete plan:", err)
	}

	result, err := completePlan.Execute(context.Background())
	if err != nil {
		log.Fatal("Failed to execute complete plan:", err)
	}

	fmt.Printf("✅ Complete Mode Result: %s\n", result)
	printPlanSummary(completePlan, "Complete Mode")

	// Demo 2: Balanced Mode with Custom Skip Confirmation
	fmt.Println("\n⚖️  Demo 2: Balanced Mode (Skip with Confirmation)")
	fmt.Println("-----------------------------------------------")

	balancedPlan, err := agent.Plan(context.Background(),
		"Research AI trends, analyze the data, and generate a report. Focus on efficiency and avoid redundant work.",
		gollem.WithPlanExecutionMode(gollem.PlanExecutionModeBalanced), // Default mode
		gollem.WithSkipConfidenceThreshold(0.7),                        // Lower than default (0.8)
		gollem.WithSkipConfirmationHook(createSkipConfirmationHook()),  // Custom confirmation
		gollem.WithPlanCreatedHook(createPlanCreatedHook("Balanced")),
		gollem.WithPlanToDoStartHook(createToDoStartHook()),
		gollem.WithPlanToDoCompletedHook(createToDoCompletedHook()),
		gollem.WithPlanToDoUpdatedHook(createToDoUpdatedHook()),
	)
	if err != nil {
		log.Fatal("Failed to create balanced plan:", err)
	}

	result, err = balancedPlan.Execute(context.Background())
	if err != nil {
		log.Fatal("Failed to execute balanced plan:", err)
	}

	fmt.Printf("✅ Balanced Mode Result: %s\n", result)
	printPlanSummary(balancedPlan, "Balanced Mode")

	// Demo 3: Efficient Mode (aggressive skipping)
	fmt.Println("\n⚡ Demo 3: Efficient Mode (Aggressive Skipping)")
	fmt.Println("----------------------------------------------")

	efficientPlan, err := agent.Plan(context.Background(),
		"Quickly research AI trends and create a brief summary. Optimize for speed and skip unnecessary steps.",
		gollem.WithPlanExecutionMode(gollem.PlanExecutionModeEfficient), // Aggressive skipping
		gollem.WithSkipConfidenceThreshold(0.6),                         // Lower threshold for efficiency
		gollem.WithPlanCreatedHook(createPlanCreatedHook("Efficient")),
		gollem.WithPlanToDoStartHook(createToDoStartHook()),
		gollem.WithPlanToDoCompletedHook(createToDoCompletedHook()),
		gollem.WithPlanToDoUpdatedHook(createToDoUpdatedHook()),
	)
	if err != nil {
		log.Fatal("Failed to create efficient plan:", err)
	}

	result, err = efficientPlan.Execute(context.Background())
	if err != nil {
		log.Fatal("Failed to execute efficient plan:", err)
	}

	fmt.Printf("✅ Efficient Mode Result: %s\n", result)
	printPlanSummary(efficientPlan, "Efficient Mode")

	// Demo 4: Using Defaults (Balanced mode with 0.8 threshold)
	fmt.Println("\n🔧 Demo 4: Using Default Settings")
	fmt.Println("----------------------------------")

	defaultPlan, err := agent.Plan(context.Background(),
		"Research AI trends and create a summary using default settings.",
		// No options = PlanExecutionModeBalanced + 0.8 threshold + default confirmation
		gollem.WithPlanCreatedHook(createPlanCreatedHook("Default")),
		gollem.WithPlanToDoStartHook(createToDoStartHook()),
		gollem.WithPlanToDoCompletedHook(createToDoCompletedHook()),
		gollem.WithPlanToDoUpdatedHook(createToDoUpdatedHook()),
	)
	if err != nil {
		log.Fatal("Failed to create default plan:", err)
	}

	result, err = defaultPlan.Execute(context.Background())
	if err != nil {
		log.Fatal("Failed to execute default plan:", err)
	}

	fmt.Printf("✅ Default Mode Result: %s\n", result)
	printPlanSummary(defaultPlan, "Default Mode (Balanced + 0.8 threshold)")

	fmt.Println("\n🎉 All demos completed successfully!")
}

// Hook functions for monitoring plan execution

func createPlanCreatedHook(mode string) gollem.PlanCreatedHook {
	return func(ctx context.Context, plan *gollem.Plan) error {
		todos := plan.GetToDos()
		fmt.Printf("📋 [%s] Plan created with %d tasks:\n", mode, len(todos))
		for i, todo := range todos {
			fmt.Printf("  %d. %s\n", i+1, todo.Description)
		}
		return nil
	}
}

func createToDoStartHook() gollem.PlanToDoStartHook {
	return func(ctx context.Context, plan *gollem.Plan, todo gollem.PlanToDo) error {
		fmt.Printf("🔄 Starting: %s\n", todo.Description)
		return nil
	}
}

func createToDoCompletedHook() gollem.PlanToDoCompletedHook {
	return func(ctx context.Context, plan *gollem.Plan, todo gollem.PlanToDo) error {
		status := "✅"
		switch todo.Status {
		case "Failed":
			status = "❌"
		case "Skipped":
			status = "⏭️"
		}
		fmt.Printf("%s Completed: %s (Status: %s)\n", status, todo.Description, todo.Status)
		return nil
	}
}

func createToDoUpdatedHook() gollem.PlanToDoUpdatedHook {
	return func(ctx context.Context, plan *gollem.Plan, changes []gollem.PlanToDoChange) error {
		for _, change := range changes {
			switch change.Type {
			case gollem.PlanToDoChangeUpdated:
				if change.NewToDo != nil && change.NewToDo.Status == "Skipped" {
					fmt.Printf("⏭️  Skipped: %s\n", change.Description)
				} else {
					fmt.Printf("🔄 Updated: %s\n", change.Description)
				}
			case gollem.PlanToDoChangeAdded:
				fmt.Printf("➕ Added: %s\n", change.Description)
			case gollem.PlanToDoChangeRemoved:
				fmt.Printf("➖ Removed: %s\n", change.Description)
			}
		}
		return nil
	}
}

func createSkipConfirmationHook() gollem.PlanSkipConfirmationHook {
	return func(ctx context.Context, plan *gollem.Plan, decision gollem.SkipDecision) bool {
		// Auto-approve very high confidence decisions
		if decision.Confidence >= 0.9 {
			fmt.Printf("🤖 Auto-approving skip (confidence: %.2f): %s\n",
				decision.Confidence, decision.SkipReason)
			return true
		}

		// For demo purposes, auto-approve medium confidence decisions
		// In a real application, you might ask the user
		if decision.Confidence >= 0.7 {
			fmt.Printf("🤔 Skip decision (confidence: %.2f):\n", decision.Confidence)
			fmt.Printf("   Reason: %s\n", decision.SkipReason)
			fmt.Printf("   Evidence: %s\n", decision.Evidence)
			fmt.Printf("   → Approving for demo\n")
			return true
		}

		// Deny low confidence decisions
		fmt.Printf("❌ Denying skip (low confidence: %.2f): %s\n",
			decision.Confidence, decision.SkipReason)
		return false
	}
}

func printPlanSummary(plan *gollem.Plan, mode string) {
	todos := plan.GetToDos()

	var completed, skipped, failed int
	for _, todo := range todos {
		switch todo.Status {
		case "Completed":
			completed++
		case "Skipped":
			skipped++
		case "Failed":
			failed++
		}
	}

	fmt.Printf("\n📊 [%s] Summary:\n", mode)
	fmt.Printf("   Total tasks: %d\n", len(todos))
	fmt.Printf("   ✅ Completed: %d\n", completed)
	if skipped > 0 {
		fmt.Printf("   ⏭️  Skipped: %d\n", skipped)
	}
	if failed > 0 {
		fmt.Printf("   ❌ Failed: %d\n", failed)
	}
	fmt.Printf("   📈 Efficiency: %.1f%% (completed + skipped)\n",
		float64(completed+skipped)/float64(len(todos))*100)
}
