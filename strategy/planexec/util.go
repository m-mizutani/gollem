package planexec

import (
	"context"
	"fmt"
	"strings"

	"github.com/m-mizutani/gollem"
)

// getNextPendingTask returns the next task that needs to be executed
func getNextPendingTask(ctx context.Context, plan *Plan) *Task {
	if plan == nil {
		return nil
	}

	for i := range plan.Tasks {
		if plan.Tasks[i].State == TaskStatePending {
			return &plan.Tasks[i]
		}
	}

	return nil
}

// allTasksCompleted checks if all tasks in the plan are completed
func allTasksCompleted(ctx context.Context, plan *Plan) bool {
	if plan == nil || len(plan.Tasks) == 0 {
		return true
	}

	for _, task := range plan.Tasks {
		if task.State != TaskStateCompleted {
			return false
		}
	}

	return true
}

// generateFinalResponse creates the final response from the completed plan
func generateFinalResponse(ctx context.Context, plan *Plan) *gollem.ExecuteResponse {
	if plan == nil {
		return &gollem.ExecuteResponse{
			Texts: []string{"No plan was executed."},
		}
	}

	// If it was a direct response (no tasks), return it
	if len(plan.Tasks) == 0 && plan.DirectResponse != "" {
		return &gollem.ExecuteResponse{
			Texts: []string{plan.DirectResponse},
		}
	}

	// Build summary of completed tasks
	var results []string

	// Add goal if present
	if plan.Goal != "" {
		results = append(results, fmt.Sprintf("Goal: %s\n", plan.Goal))
	}

	// Add completed tasks and their results
	results = append(results, "Completed Tasks:")
	for i, task := range plan.Tasks {
		if task.State == TaskStateCompleted {
			results = append(results, fmt.Sprintf("%d. %s", i+1, task.Description))
			if task.Result != "" {
				// Indent result for readability
				resultLines := strings.Split(task.Result, "\n")
				for _, line := range resultLines {
					if line != "" {
						results = append(results, fmt.Sprintf("   %s", line))
					}
				}
			}
		}
	}

	// Join all results with newlines
	finalText := strings.Join(results, "\n")

	return &gollem.ExecuteResponse{
		Texts: []string{finalText},
	}
}
