package planexec

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
)

// reflectionResult holds the result of reflection
type reflectionResult struct {
	UpdatedTasks []Task // Modified tasks
	NewTasks     []Task // New tasks to add
}

// reflect performs reflection after task completion to update or add tasks
func reflect(ctx context.Context, client gollem.LLMClient, plan *Plan, completedTask *Task, tools []gollem.Tool, middleware []gollem.ContentBlockMiddleware) (*reflectionResult, error) {
	logger := ctxlog.From(ctx)
	logger.Debug("performing reflection")

	// Create a new session for reflection with JSON content type
	// NOTE: Do NOT pass tools to reflection session.
	// When tools are provided, some LLM providers (like Gemini) prioritize function calls
	// over JSON text responses, which breaks the reflection phase that requires JSON output.
	sessionOpts := []gollem.SessionOption{
		gollem.WithSessionContentType(gollem.ContentTypeJSON),
	}
	for _, mw := range middleware {
		sessionOpts = append(sessionOpts, gollem.WithSessionContentBlockMiddleware(mw))
	}

	session, err := client.NewSession(ctx, sessionOpts...)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create session for reflection")
	}

	// Build reflection prompt
	reflectPrompt := buildReflectPrompt(ctx, plan, completedTask.Result, tools)

	// Generate reflection using LLM
	response, err := session.GenerateContent(ctx, reflectPrompt...)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to generate reflection")
	}

	// Parse the reflection response
	result, err := parseReflectionFromResponse(ctx, response, plan)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to parse reflection response")
	}

	logger.Debug("reflection completed", "new_tasks", len(result.NewTasks), "updated_tasks", len(result.UpdatedTasks))
	return result, nil
}

// parseReflectionFromResponse extracts reflection results from LLM response
func parseReflectionFromResponse(ctx context.Context, response *gollem.Response, currentPlan *Plan) (*reflectionResult, error) {
	result := &reflectionResult{
		UpdatedTasks: []Task{},
		NewTasks:     []Task{},
	}

	if response == nil {
		// If no response, return empty result (no updates)
		return result, nil
	}

	if len(response.Texts) == 0 {
		// If no text but has function calls, return empty result (no updates)
		return result, nil
	}

	// Parse JSON response directly (WithSessionContentType ensures JSON format)
	var reflectionResponse struct {
		NewTasks     []string `json:"new_tasks"` // Task descriptions for new tasks
		UpdatedTasks []struct {
			ID          string `json:"id"`
			Description string `json:"description"`
			State       string `json:"state"`
		} `json:"updated_tasks"` // Tasks to update (mark as failed, pending, etc.)
		Reason string `json:"reason"` // Explanation
	}

	if err := json.Unmarshal([]byte(response.Texts[0]), &reflectionResponse); err != nil {
		// If JSON parsing fails, return empty result
		logger := ctxlog.From(ctx)
		logger.Debug("failed to parse reflection JSON, returning empty result", "error", err.Error())
		return result, nil
	}

	// Process new tasks
	for _, taskDesc := range reflectionResponse.NewTasks {
		result.NewTasks = append(result.NewTasks, Task{
			ID:          uuid.New().String(),
			Description: taskDesc,
			State:       TaskStatePending,
		})
	}

	// Process updated tasks
	for _, updatedTask := range reflectionResponse.UpdatedTasks {
		state := TaskStatePending
		switch updatedTask.State {
		case "pending":
			state = TaskStatePending
		case "in_progress":
			state = TaskStateInProgress
		case "completed":
			state = TaskStateCompleted
		}

		result.UpdatedTasks = append(result.UpdatedTasks, Task{
			ID:          updatedTask.ID,
			Description: updatedTask.Description,
			State:       state,
		})
	}

	logger := ctxlog.From(ctx)
	logger.Debug("reflection parsed", "new_tasks", len(result.NewTasks), "updated_tasks", len(result.UpdatedTasks), "reason", reflectionResponse.Reason)

	return result, nil
}
