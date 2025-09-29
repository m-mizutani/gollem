package planexec

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
)

// analyzeAndPlan analyzes user input and creates a plan using LLM
func analyzeAndPlan(ctx context.Context, client gollem.LLMClient, inputs []gollem.Input, middleware []gollem.ContentBlockMiddleware) (*Plan, error) {
	logger := ctxlog.From(ctx)
	logger.Debug("analyzing and planning")

	// Create a new session with JSON content type
	sessionOpts := []gollem.SessionOption{
		gollem.WithSessionContentType(gollem.ContentTypeJSON),
	}
	for _, mw := range middleware {
		sessionOpts = append(sessionOpts, gollem.WithSessionContentBlockMiddleware(mw))
	}

	session, err := client.NewSession(ctx, sessionOpts...)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create session")
	}

	// Build planning prompt
	planPrompt := buildPlanPrompt(ctx, inputs)

	// Generate plan using LLM
	response, err := session.GenerateContent(ctx, planPrompt...)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to generate plan")
	}

	// Parse the response to extract plan
	plan, err := parsePlanFromResponse(ctx, response)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to parse plan from response")
	}

	logger.Debug("plan created", "goal", plan.Goal, "tasks", len(plan.Tasks))
	return plan, nil
}

// parsePlanFromResponse extracts plan from LLM response
func parsePlanFromResponse(ctx context.Context, response *gollem.Response) (*Plan, error) {
	if response == nil || len(response.Texts) == 0 {
		return nil, goerr.New("empty response from LLM")
	}

	// Parse JSON response directly (WithSessionContentType ensures JSON format)
	var planResponse struct {
		NeedsPlan      bool   `json:"needs_plan"`
		DirectResponse string `json:"direct_response"`
		Goal           string `json:"goal"`
		Tasks          []struct {
			Description string `json:"description"`
		} `json:"tasks"`
	}

	if err := json.Unmarshal([]byte(response.Texts[0]), &planResponse); err != nil {
		// If JSON parsing fails, log error and return
		logger := ctxlog.From(ctx)
		logger.Debug("failed to parse plan JSON", "error", err.Error())
		return nil, goerr.Wrap(err, "failed to parse plan response as JSON")
	}

	// Create plan based on response
	if !planResponse.NeedsPlan {
		return &Plan{
			DirectResponse: planResponse.DirectResponse,
			Tasks:          []Task{},
		}, nil
	}

	// Convert to Plan with Tasks
	plan := &Plan{
		Goal:  planResponse.Goal,
		Tasks: make([]Task, len(planResponse.Tasks)),
	}

	for i, t := range planResponse.Tasks {
		plan.Tasks[i] = Task{
			ID:          uuid.New().String(),
			Description: t.Description,
			State:       TaskStatePending,
		}
	}

	return plan, nil
}
