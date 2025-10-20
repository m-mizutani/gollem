package planexec

import (
	"context"
	"fmt"
	"strings"

	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
)

// getNextPendingTask returns the next task that needs to be executed
func getNextPendingTask(_ context.Context, plan *Plan) *Task {
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

// allTasksCompleted checks if all tasks in the plan are completed or skipped
func allTasksCompleted(ctx context.Context, plan *Plan) bool {
	if plan == nil || len(plan.Tasks) == 0 {
		return true
	}

	for _, task := range plan.Tasks {
		// Tasks that are completed or skipped are considered "done"
		if task.State != TaskStateCompleted && task.State != TaskStateSkipped {
			return false
		}
	}

	return true
}

// getFinalConclusion asks LLM to generate final conclusion based on completed tasks
func getFinalConclusion(ctx context.Context, client gollem.LLMClient, plan *Plan, middleware []gollem.ContentBlockMiddleware) (*gollem.ExecuteResponse, error) {
	logger := ctxlog.From(ctx)
	logger.Debug("generating final conclusion")

	if plan == nil {
		return &gollem.ExecuteResponse{
			Texts: []string{"No plan was executed."},
		}, nil
	}

	// If it was a direct response (no tasks), return it
	if len(plan.Tasks) == 0 && plan.DirectResponse != "" {
		return &gollem.ExecuteResponse{
			Texts: []string{plan.DirectResponse},
		}, nil
	}

	// Build summary of completed tasks
	var taskSummaries []string
	for _, task := range plan.Tasks {
		if task.State == TaskStateCompleted {
			summary := fmt.Sprintf("- %s", task.Description)
			if task.Result != "" {
				summary += fmt.Sprintf("\n  Result: %s", task.Result)
			}
			taskSummaries = append(taskSummaries, summary)
		}
	}

	// Create conclusion prompt
	conclusionPrompt := fmt.Sprintf(`All tasks have been completed. Please provide a final summary.

Goal: %s

Completed Tasks:
%s

IMPORTANT: You should now provide a text summary of what was accomplished. Do NOT use function calls for this response. Simply summarize the results in natural language.`,
		plan.Goal,
		strings.Join(taskSummaries, "\n"))

	// Create new session for conclusion
	sessionOpts := []gollem.SessionOption{}
	for _, mw := range middleware {
		sessionOpts = append(sessionOpts, gollem.WithSessionContentBlockMiddleware(mw))
	}

	session, err := client.NewSession(ctx, sessionOpts...)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create session for conclusion")
	}

	// Generate conclusion
	response, err := session.GenerateContent(ctx, gollem.Text(conclusionPrompt))
	if err != nil {
		return nil, goerr.Wrap(err, "failed to generate conclusion")
	}

	return &gollem.ExecuteResponse{
		Texts: response.Texts,
	}, nil
}

// generateFinalResponse creates the final response from the completed plan (without LLM call)
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
