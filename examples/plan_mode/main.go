package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/openai"
)

// Simple search tool for demonstration
type searchTool struct{}

func (s *searchTool) Spec() gollem.ToolSpec {
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

func (s *searchTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	query, ok := args["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query must be a string")
	}

	// Simulate search results
	return map[string]any{
		"results": fmt.Sprintf("Search results for: %s", query),
		"count":   3,
	}, nil
}

// Analysis tool for demonstration
type analysisTool struct{}

func (a *analysisTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "analyze",
		Description: "Analyze data and extract insights",
		Parameters: map[string]*gollem.Parameter{
			"data": {
				Type:        gollem.TypeString,
				Description: "Data to analyze",
			},
		},
		Required: []string{"data"},
	}
}

func (a *analysisTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	data, ok := args["data"].(string)
	if !ok {
		return nil, fmt.Errorf("data must be a string")
	}

	// Simulate analysis
	return map[string]any{
		"insights": fmt.Sprintf("Analysis insights from: %s", data),
		"trends":   []string{"trend1", "trend2", "trend3"},
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
	// Check for OpenAI API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	// Create OpenAI client
	llmClient, err := openai.New(context.Background(), apiKey)
	if err != nil {
		log.Fatal("Failed to create OpenAI client:", err)
	}

	// Create gollem agent with tools
	agent := gollem.New(llmClient,
		gollem.WithTools(&searchTool{}, &analysisTool{}),
		gollem.WithSystemPrompt("You are a helpful assistant that creates detailed plans and executes them step by step."),
	)

	// Create a plan with hooks for progress tracking
	fmt.Println("🚀 Creating plan...")
	plan, err := agent.Plan(context.Background(),
		"Find information about electric cars and summarize the benefits",
		gollem.WithPlanSystemPrompt("You are an expert research assistant focusing on clean energy technologies."),
		gollem.WithPlanCreatedHook(func(ctx context.Context, plan *gollem.Plan) error {
			fmt.Println("✨ Plan created successfully!")
			displayToDoList(plan)
			displayProgress(plan)
			return nil
		}),
		gollem.WithPlanToDoStartHook(func(ctx context.Context, plan *gollem.Plan, todo gollem.PlanToDo) error {
			fmt.Println("\n" + strings.Repeat("=", 80))
			fmt.Printf("🔄 STARTING TODO: %s\n", todo.Description)
			if todo.Intent != "" {
				fmt.Printf("   💡 Intent: %s\n", todo.Intent)
			}
			fmt.Println(strings.Repeat("=", 80))
			displayToDoList(plan)
			displayProgress(plan)
			return nil
		}),
		gollem.WithPlanToDoCompletedHook(func(ctx context.Context, plan *gollem.Plan, todo gollem.PlanToDo) error {
			fmt.Println("\n" + strings.Repeat("-", 80))
			fmt.Printf("✅ COMPLETED TODO: %s\n", todo.Description)
			fmt.Println(strings.Repeat("-", 80))
			displayToDoList(plan)
			displayProgress(plan)
			return nil
		}),
		gollem.WithPlanToDoUpdatedHook(func(ctx context.Context, plan *gollem.Plan, changes []gollem.PlanToDoChange) error {
			fmt.Println("\n" + strings.Repeat("~", 80))
			fmt.Printf("🔄 PLAN UPDATED: %d changes detected\n", len(changes))
			for _, change := range changes {
				switch change.Type {
				case gollem.PlanToDoChangeUpdated:
					fmt.Printf("   🔧 Updated: %s\n", change.Description)
				case gollem.PlanToDoChangeAdded:
					fmt.Printf("   ➕ Added: %s\n", change.Description)
				case gollem.PlanToDoChangeRemoved:
					fmt.Printf("   ➖ Removed: %s\n", change.Description)
				}
			}
			fmt.Println(strings.Repeat("~", 80))
			displayToDoList(plan)
			displayProgress(plan)
			return nil
		}),
		gollem.WithPlanMessageHook(func(ctx context.Context, plan *gollem.Plan, message gollem.PlanExecutionMessage) error {
			// Only display important messages to avoid spam
			if message.Type == gollem.PlanMessageResponse && strings.TrimSpace(message.Content) != "" {
				fmt.Printf("💬 [%s] %s\n", message.Type, message.Content)
			}
			return nil
		}),
	)
	if err != nil {
		log.Fatal("Failed to create plan:", err)
	}

	// Execute the plan
	fmt.Println("\n🚀 Executing plan...")
	result, err := plan.Execute(context.Background())
	if err != nil {
		log.Fatal("Failed to execute plan:", err)
	}

	fmt.Println("\n🎉 === Plan Execution Completed ===")
	fmt.Println("📄 Final Result:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println(result)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Show final summary
	fmt.Println("\n📊 Final Plan Summary:")
	displayToDoList(plan)
	displayProgress(plan)
}
