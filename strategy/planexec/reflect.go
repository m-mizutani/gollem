package planexec

import (
	"context"
	"encoding/json"

	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
)

// reflect performs reflection after task completion to determine next steps
func reflect(ctx context.Context, client gollem.LLMClient, plan *Plan, middleware []gollem.ContentBlockMiddleware) (*Plan, bool, error) {
	logger := ctxlog.From(ctx)
	logger.Debug("performing reflection")

	// Create a new session for reflection with JSON content type
	sessionOpts := []gollem.SessionOption{
		gollem.WithSessionContentType(gollem.ContentTypeJSON),
	}

	session, err := client.NewSession(ctx, sessionOpts...)
	if err != nil {
		return nil, false, goerr.Wrap(err, "failed to create session for reflection")
	}

	// Build reflection prompt
	reflectPrompt := buildReflectPrompt(ctx, plan)

	// Generate reflection using LLM
	response, err := session.GenerateContent(ctx, reflectPrompt...)
	if err != nil {
		return nil, false, goerr.Wrap(err, "failed to generate reflection")
	}

	// Parse the reflection response
	updatedPlan, shouldContinue, err := parseReflectionFromResponse(ctx, response, plan)
	if err != nil {
		return nil, false, goerr.Wrap(err, "failed to parse reflection response")
	}

	logger.Debug("reflection completed", "should_continue", shouldContinue)
	return updatedPlan, shouldContinue, nil
}

// parseReflectionFromResponse extracts reflection results from LLM response
func parseReflectionFromResponse(ctx context.Context, response *gollem.Response, currentPlan *Plan) (*Plan, bool, error) {
	if response == nil || len(response.Texts) == 0 {
		// If no response, continue with current plan
		return nil, true, nil
	}

	// Parse JSON response directly (WithSessionContentType ensures JSON format)
	var reflectionResponse struct {
		GoalAchieved   bool   `json:"goal_achieved"`
		ShouldContinue bool   `json:"should_continue"`
		Reason         string `json:"reason"`
		PlanUpdates    struct {
			NewTasks    []string `json:"new_tasks"`
			RemoveTasks []string `json:"remove_tasks"`
		} `json:"plan_updates"`
	}

	if err := json.Unmarshal([]byte(response.Texts[0]), &reflectionResponse); err != nil {
		// If JSON parsing fails, continue with current plan
		logger := ctxlog.From(ctx)
		logger.Debug("failed to parse reflection JSON, continuing with current plan", "error", err.Error())
		return nil, true, nil
	}

	// Check if we should continue
	shouldContinue := reflectionResponse.ShouldContinue && !reflectionResponse.GoalAchieved

	// If plan updates are needed, create updated plan
	var updatedPlan *Plan
	if len(reflectionResponse.PlanUpdates.NewTasks) > 0 || len(reflectionResponse.PlanUpdates.RemoveTasks) > 0 {
		// Create a copy of the current plan
		updatedPlan = &Plan{
			Goal:  currentPlan.Goal,
			Tasks: make([]Task, 0, len(currentPlan.Tasks)),
		}

		// Copy existing tasks (except those to be removed)
		removeMap := make(map[string]bool)
		for _, taskDesc := range reflectionResponse.PlanUpdates.RemoveTasks {
			removeMap[taskDesc] = true
		}

		for _, task := range currentPlan.Tasks {
			if !removeMap[task.Description] {
				updatedPlan.Tasks = append(updatedPlan.Tasks, task)
			}
		}

		// Add new tasks
		for _, taskDesc := range reflectionResponse.PlanUpdates.NewTasks {
			updatedPlan.Tasks = append(updatedPlan.Tasks, Task{
				Description: taskDesc,
				State:       TaskStatePending,
			})
		}

		logger := ctxlog.From(ctx)
		logger.Debug("plan updated", "new_tasks", len(reflectionResponse.PlanUpdates.NewTasks),
			"removed_tasks", len(reflectionResponse.PlanUpdates.RemoveTasks))
	}

	return updatedPlan, shouldContinue, nil
}
