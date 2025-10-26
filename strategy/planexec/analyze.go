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
// It uses system prompt and history to embed necessary context into the Plan's goal
// This is an internal analysis process - the conversation history is not preserved
func analyzeAndPlan(ctx context.Context, client gollem.LLMClient, inputs []gollem.Input, tools []gollem.Tool, middleware []gollem.ContentBlockMiddleware, systemPrompt string, history *gollem.History) (*Plan, error) {
	logger := ctxlog.From(ctx)
	logger.Debug("analyzing and planning", "has_system_prompt", systemPrompt != "", "has_history", history != nil)

	// Create a new session with JSON content type
	// NOTE: Do NOT pass tools to planning session.
	// When tools are provided, some LLM providers (like Gemini) prioritize function calls
	// over JSON text responses, which breaks the planning phase that requires JSON output.
	sessionOpts := []gollem.SessionOption{
		gollem.WithSessionContentType(gollem.ContentTypeJSON),
	}

	// Add system prompt if provided
	// The system prompt helps the planner understand domain-specific constraints
	// which should be embedded into the Plan's `context_summary` and `constraints` fields.
	if systemPrompt != "" {
		sessionOpts = append(sessionOpts, gollem.WithSessionSystemPrompt(systemPrompt))
	}

	// Add history if provided
	// The history provides conversation context that should be considered
	// when creating the plan and embedded into the `context_summary` field.
	if history != nil {
		sessionOpts = append(sessionOpts, gollem.WithSessionHistory(history))
	}

	for _, mw := range middleware {
		sessionOpts = append(sessionOpts, gollem.WithSessionContentBlockMiddleware(mw))
	}

	session, err := client.NewSession(ctx, sessionOpts...)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create session")
	}

	// Build planning prompt
	planPrompt := buildPlanPrompt(ctx, inputs, tools)

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
	if response == nil {
		return nil, goerr.New("response is nil")
	}
	if len(response.Texts) == 0 {
		return nil, goerr.New("empty response from LLM")
	}

	// Parse JSON response directly (WithSessionContentType ensures JSON format)
	var planResponse struct {
		NeedsPlan      bool   `json:"needs_plan"`
		DirectResponse string `json:"direct_response"`
		Goal           string `json:"goal"`
		ContextSummary string `json:"context_summary"`
		Constraints    string `json:"constraints"`
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
		Goal:           planResponse.Goal,
		ContextSummary: planResponse.ContextSummary,
		Constraints:    planResponse.Constraints,
		Tasks:          make([]Task, len(planResponse.Tasks)),
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
