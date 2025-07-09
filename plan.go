package gollem

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/m-mizutani/goerr/v2"
)

// Plan represents an executable plan
type Plan struct {
	// Internal state (may be processed asynchronously except during Execute execution)
	id    string
	input string
	todos []planToDo
	state PlanState

	// Fields reconstructed at runtime (not serialized)
	agent       *Agent          `json:"-"`
	toolMap     map[string]Tool `json:"-"`
	config      *planConfig     `json:"-"`
	mainSession Session         `json:"-"` // Main session for plan execution (immutable once set)
	logger      *slog.Logger    `json:"-"` // Logger for plan execution
}

// planToDo represents a single task in the plan (private to avoid API confusion)
type planToDo struct {
	ID          string      `json:"todo_id"`
	Description string      `json:"todo_description"`
	Intent      string      `json:"todo_intent"` // High-level intention
	Status      ToDoStatus  `json:"todo_status"`
	Result      *toDoResult `json:"todo_result,omitempty"`
	Error       error       `json:"-"` // Not serialized
	ErrorMsg    string      `json:"todo_error,omitempty"`
	UpdatedAt   time.Time   `json:"todo_updated_at,omitempty"` // When todo was last updated
	CreatedAt   time.Time   `json:"todo_created_at,omitempty"` // When todo was created
}

// PlanState represents the current state of plan execution (private)
type PlanState string

const (
	PlanStateCreated   PlanState = "created"
	PlanStateRunning   PlanState = "running"
	PlanStateCompleted PlanState = "completed"
	PlanStateFailed    PlanState = "failed"
)

// ToDoStatus represents the status of a plan todo (private)
type ToDoStatus string

const (
	ToDoStatusPending   ToDoStatus = "pending"
	ToDoStatusExecuting ToDoStatus = "executing"
	ToDoStatusCompleted ToDoStatus = "completed"
	ToDoStatusFailed    ToDoStatus = "failed"
	ToDoStatusSkipped   ToDoStatus = "skipped"
)

// toDoResult represents the result of executing a plan todo (private)
type toDoResult struct {
	Output     string          `json:"todo_output"`
	ToolCalls  []*FunctionCall `json:"todo_tool_calls,omitempty"` // Use existing FunctionCall type
	Data       map[string]any  `json:"todo_data,omitempty"`
	ExecutedAt time.Time       `json:"todo_executed_at"`
}

// PlanReflectionType represents the type of reflection outcome
type PlanReflectionType string

const (
	PlanReflectionTypeContinue    PlanReflectionType = "continue"     // Plan should continue with current todos
	PlanReflectionTypeRefine      PlanReflectionType = "refine"       // Todos were refined/updated
	PlanReflectionTypeExpand      PlanReflectionType = "expand"       // New todos were added
	PlanReflectionTypeComplete    PlanReflectionType = "complete"     // Plan completed by reflection
	PlanReflectionTypeRefinedDone PlanReflectionType = "refined_done" // Plan completed after refinement
)

// PlanToDoChange represents a change to a todo during reflection
type PlanToDoChange struct {
	Type        PlanToDoChangeType `json:"change_type"`
	TodoID      string             `json:"todo_id"`
	OldToDo     *planToDo          `json:"old_todo,omitempty"`
	NewToDo     *planToDo          `json:"new_todo,omitempty"`
	Description string             `json:"description"` // Human-readable description of change
}

// PlanToDoChangeType represents the type of change to a todo
type PlanToDoChangeType string

const (
	PlanToDoChangeTypeUpdated PlanToDoChangeType = "updated" // Todo was updated/refined
	PlanToDoChangeTypeAdded   PlanToDoChangeType = "added"   // Todo was added
	PlanToDoChangeTypeRemoved PlanToDoChangeType = "removed" // Todo was removed
)

// Public constants for external use
const (
	PlanToDoChangeUpdated = PlanToDoChangeTypeUpdated
	PlanToDoChangeAdded   = PlanToDoChangeTypeAdded
	PlanToDoChangeRemoved = PlanToDoChangeTypeRemoved
)

// PlanExecutionMessage represents a message during plan execution
type PlanExecutionMessage struct {
	Type      PlanMessageType `json:"message_type"`
	Content   string          `json:"content"`
	TodoID    string          `json:"todo_id,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// PlanMessageType represents the type of plan execution message
type PlanMessageType string

const (
	PlanMessageTypeThought  PlanMessageType = "thought"  // LLM thinking/reasoning
	PlanMessageTypeAction   PlanMessageType = "action"   // Action being taken
	PlanMessageTypeResponse PlanMessageType = "response" // Response to user
	PlanMessageTypeSystem   PlanMessageType = "system"   // System message
)

// Public constants for external use
const (
	PlanMessageThought  = PlanMessageTypeThought
	PlanMessageAction   = PlanMessageTypeAction
	PlanMessageResponse = PlanMessageTypeResponse
	PlanMessageSystem   = PlanMessageTypeSystem
)

// Public hook types for external API
type (
	// PlanCreatedHook is called when a plan is successfully created
	PlanCreatedHook func(ctx context.Context, plan *Plan) error

	// PlanToDoStartHook is called when a plan todo starts execution
	PlanToDoStartHook func(ctx context.Context, plan *Plan, todo PlanToDo) error

	// PlanToDoCompletedHook is called when a plan todo completes successfully
	PlanToDoCompletedHook func(ctx context.Context, plan *Plan, todo PlanToDo) error

	// PlanReflectionHook is called after reflection analysis
	PlanReflectionHook func(ctx context.Context, plan *Plan, reflection *planReflection) error

	// PlanCompletedHook is called when the entire plan is completed
	PlanCompletedHook func(ctx context.Context, plan *Plan, result string) error

	// PlanToDoUpdatedHook is called when todos are updated/refined during reflection
	PlanToDoUpdatedHook func(ctx context.Context, plan *Plan, changes []PlanToDoChange) error

	// PlanMessageHook is called when a message is generated during plan execution
	PlanMessageHook func(ctx context.Context, plan *Plan, message PlanExecutionMessage) error
)

// Internal hook types for library implementation (unexported)
type (
	// planStepStartHook is called when a plan todo starts execution (internal)
	planStepStartHook func(ctx context.Context, plan *Plan, todo *planToDo) error

	// planStepCompletedHook is called when a plan todo completes successfully (internal)
	planStepCompletedHook func(ctx context.Context, plan *Plan, todo *planToDo, result *toDoResult) error

	// planToDoUpdatedHook is called when todos are updated/refined during reflection (internal)
	planToDoUpdatedHook func(ctx context.Context, plan *Plan, changes []PlanToDoChange) error

	// planMessageHook is called when a message is generated during plan execution (internal)
	planMessageHook func(ctx context.Context, plan *Plan, message PlanExecutionMessage) error
)

// planReflection represents the result of plan reflection (private)
type planReflection struct {
	Type             PlanReflectionType `json:"reflection_type"`
	ShouldContinue   *bool              `json:"should_continue"`
	UpdatedToDos     []planToDo         `json:"updated_todos,omitempty"`
	NewToDos         []planToDo         `json:"new_todos,omitempty"`
	CompletionReason string             `json:"completion_reason,omitempty"`
	Response         string             `json:"response,omitempty"`
	Changes          []PlanToDoChange   `json:"changes,omitempty"` // Detailed changes
}

// planConfig holds configuration for plan creation and execution
type planConfig struct {
	gollemConfig

	// Public hooks for external API
	planCreatedHook       PlanCreatedHook
	planToDoStartHook     PlanToDoStartHook
	planToDoCompletedHook PlanToDoCompletedHook
	planCompletedHook     PlanCompletedHook
	planToDoUpdatedHook   PlanToDoUpdatedHook
	planMessageHook       PlanMessageHook

	// Internal hooks for library implementation
	internalStepStartHook     planStepStartHook
	internalStepCompletedHook planStepCompletedHook
	internalToDoUpdatedHook   planToDoUpdatedHook
	internalMessageHook       planMessageHook
	internalReflectionHook    PlanReflectionHook // Keep as exported since reflection is complex

	// Plan-specific settings
	// (reserved for future use)
}

// PlanOption represents configuration options for plan creation and execution
type PlanOption func(*planConfig)

// Default hook implementations for public API
func defaultPlanCreatedHook(ctx context.Context, plan *Plan) error {
	return nil
}

func defaultPlanToDoStartHook(ctx context.Context, plan *Plan, todo PlanToDo) error {
	return nil
}

func defaultPlanToDoCompletedHook(ctx context.Context, plan *Plan, todo PlanToDo) error {
	return nil
}

func defaultPlanCompletedHook(ctx context.Context, plan *Plan, result string) error {
	return nil
}

func defaultPlanToDoUpdatedHook(ctx context.Context, plan *Plan, changes []PlanToDoChange) error {
	return nil
}

func defaultPlanMessageHook(ctx context.Context, plan *Plan, message PlanExecutionMessage) error {
	return nil
}

// Default hook implementations for internal use
func defaultInternalStepStartHook(ctx context.Context, plan *Plan, todo *planToDo) error {
	return nil
}

func defaultInternalStepCompletedHook(ctx context.Context, plan *Plan, todo *planToDo, result *toDoResult) error {
	return nil
}

func defaultInternalReflectionHook(ctx context.Context, plan *Plan, reflection *planReflection) error {
	return nil
}

func defaultInternalToDoUpdatedHook(ctx context.Context, plan *Plan, changes []PlanToDoChange) error {
	return nil
}

func defaultInternalMessageHook(ctx context.Context, plan *Plan, message PlanExecutionMessage) error {
	return nil
}

// Plan creates an execution plan based on the given prompt
func (g *Agent) Plan(ctx context.Context, prompt string, options ...PlanOption) (*Plan, error) {
	cfg := g.createPlanConfig(options...)

	// UUID import required ("github.com/google/uuid")
	planID := uuid.New().String()
	logger := cfg.logger.With("gollem.plan_id", planID)
	ctx = ctxWithLogger(ctx, logger) // Use existing context.go function

	// Tool setup (use existing setupTools function)
	// Facilitator is not supported in plan mode
	cfg.gollemConfig.facilitator = nil
	toolMap, toolList, err := setupTools(ctx, &cfg.gollemConfig)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to setup tools for plan")
	}

	// DEBUG: Log tools available for plan
	logger.Debug("tools setup for plan", "tool_count", len(toolList), "tool_names", func() []string {
		names := make([]string, len(toolList))
		for i, tool := range toolList {
			names[i] = tool.Spec().Name
		}
		return names
	}())

	// Create planner session
	plannerSession, err := g.createPlannerSession(ctx, cfg, toolList)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create planner session")
	}

	// Generate plan (provide tool information)
	todos, err := g.generatePlan(ctx, plannerSession, prompt, toolList, cfg.systemPrompt)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to generate plan")
	}

	// Create plan with runtime fields
	plan, err := g.createPlanWithRuntime(planID, prompt, todos, PlanStateCreated, toolMap, toolList, cfg, logger, ctx)
	if err != nil {
		return nil, err
	}

	// Call PlanCreatedHook
	if err := cfg.planCreatedHook(ctx, plan); err != nil {
		return nil, goerr.Wrap(err, "failed to call PlanCreatedHook")
	}

	logger.Info("plan created",
		"plan_id", planID,
		"todos_count", len(todos),
		"prompt", prompt)

	return plan, nil
}

// Execute executes the plan and returns the final result
func (p *Plan) Execute(ctx context.Context) (string, error) {
	// Embed logger into context for internal methods to use
	ctx = ctxWithLogger(ctx, p.logger)

	p.logger.Debug("plan execute started", "plan_id", p.id, "state", p.state)

	if err := p.validateAndPrepareExecution(); err != nil {
		p.logger.Debug("plan validation failed", "plan_id", p.id, "error", err)
		return "", err
	}

	p.logger.Debug("plan validation passed, starting execution", "plan_id", p.id)
	return p.executeSteps(ctx)
}

// validateAndPrepareExecution validates the plan state and prepares for execution
func (p *Plan) validateAndPrepareExecution() error {
	if p.state != PlanStateCreated {
		return ErrPlanAlreadyExecuted
	}

	p.state = PlanStateRunning

	// Restore runtime fields (when deserialized)
	if p.agent == nil {
		return ErrPlanNotInitialized
	}
	if p.mainSession == nil {
		return ErrPlanNotInitialized
	}

	return nil
}

// executeSteps executes all pending steps in the plan
func (p *Plan) executeSteps(ctx context.Context) (string, error) {
	logger := LoggerFromContext(ctx) // Use existing context.go function
	logger.Debug("executeSteps started", "plan_id", p.id, "pending_todos_count", len(p.getPendingToDos()))

	for len(p.getPendingToDos()) > 0 {
		currentStep := p.getNextPendingToDo()
		if currentStep == nil {
			logger.Debug("no more pending todos found", "plan_id", p.id)
			break
		}

		logger.Debug("processing plan step",
			"plan_id", p.id,
			"step_id", currentStep.ID,
			"step_description", currentStep.Description,
			"pending_count", len(p.getPendingToDos()))

		// Process single step
		result, shouldComplete, err := p.processSingleStep(ctx, currentStep)
		if err != nil {
			logger.Error("plan step processing failed",
				"plan_id", p.id,
				"step_id", currentStep.ID,
				"error", err)
			return "", err
		}

		if shouldComplete {
			logger.Debug("plan completed successfully by reflection",
				"plan_id", p.id,
				"step_id", currentStep.ID,
				"todos_executed", len(p.getCompletedToDos()),
				"result", result)
			return result, nil
		}

		logger.Debug("plan step completed, continuing to next step",
			"plan_id", p.id,
			"step_id", currentStep.ID,
			"remaining_pending", len(p.getPendingToDos()))
	}

	p.state = PlanStateCompleted

	logger.Info("plan completed - all steps processed", "plan_id", p.id)
	return "Plan completed", nil
}

// processSingleStep processes a single step including hooks, execution, and reflection
func (p *Plan) processSingleStep(ctx context.Context, currentStep *planToDo) (string, bool, error) {
	logger := LoggerFromContext(ctx)
	logger.Debug("starting step processing", "plan_id", p.id, "step_id", currentStep.ID, "step_intent", currentStep.Intent)

	// Call step start hooks
	if err := p.callStepStartHooks(ctx, currentStep); err != nil {
		return "", false, err
	}

	logger.Debug("executing step", "plan_id", p.id, "step_id", currentStep.ID)
	// Execute step
	result, err := p.executeStep(ctx, currentStep)
	if err != nil {
		logger.Debug("step execution failed", "plan_id", p.id, "step_id", currentStep.ID, "error", err)
		return "", false, p.handleStepError(currentStep, err)
	}

	logger.Debug("step execution completed", "plan_id", p.id, "step_id", currentStep.ID, "output_length", len(result.Output), "tool_calls_count", len(result.ToolCalls))
	currentStep.Status = ToDoStatusCompleted
	currentStep.Result = result

	// Call step completed hooks
	if err := p.callStepCompletedHooks(ctx, currentStep, result); err != nil {
		return "", false, err
	}

	// Reflection and re-planning
	logger.Debug("starting plan reflection", "plan_id", p.id, "step_id", currentStep.ID)
	reflection, err := p.reflect(ctx)
	if err != nil {
		logger.Debug("plan reflection failed", "plan_id", p.id, "step_id", currentStep.ID, "error", err)
		return "", false, goerr.Wrap(err, "plan reflection failed")
	}

	// Helper for logging bool pointer
	shouldContinueValue := "nil"
	if reflection.ShouldContinue != nil {
		shouldContinueValue = fmt.Sprintf("%t", *reflection.ShouldContinue)
	}

	logger.Debug("plan reflection completed",
		"plan_id", p.id,
		"should_continue", shouldContinueValue,
		"completion_reason", reflection.CompletionReason,
		"new_todos_count", len(reflection.NewToDos),
		"updated_todos_count", len(reflection.UpdatedToDos))

	// Call internal reflection hook
	if err := p.config.internalReflectionHook(ctx, p, reflection); err != nil {
		return "", false, goerr.Wrap(err, "failed to call internal reflection hook")
	}

	// Check if reflection indicates completion
	// If ShouldContinue is nil or false, we consider it as completion
	shouldContinue := reflection.ShouldContinue != nil && *reflection.ShouldContinue
	if !shouldContinue {
		p.state = PlanStateCompleted

		// Update plan first to generate changes if needed
		if err := p.updatePlan(reflection); err != nil {
			logger.Debug("plan update failed during completion", "plan_id", p.id, "step_id", currentStep.ID, "error", err)
			return "", false, goerr.Wrap(err, "failed to update plan during completion")
		}

		// Set completion type based on whether there were changes
		if len(reflection.Changes) > 0 {
			reflection.Type = PlanReflectionTypeRefinedDone
		} else {
			reflection.Type = PlanReflectionTypeComplete
		}

		logger.Debug("plan marked as completed by reflection",
			"plan_id", p.id,
			"completion_reason", reflection.CompletionReason,
			"response_length", len(reflection.Response),
			"reflection_type", reflection.Type)

		// Call PlanToDoUpdatedHook if there are changes
		if len(reflection.Changes) > 0 {
			if err := p.callToDoUpdatedHook(ctx, reflection.Changes); err != nil {
				return "", false, err
			}
		}

		// Send completion message
		if reflection.Response != "" {
			completionMessage := PlanExecutionMessage{
				Type:      PlanMessageTypeResponse,
				Content:   reflection.Response,
				TodoID:    currentStep.ID,
				Timestamp: time.Now(),
			}
			if err := p.callMessageHook(ctx, completionMessage); err != nil {
				logger.Warn("failed to call message hook for completion", "error", err)
			}
		}

		// Call PlanCompletedHook
		if err := p.config.planCompletedHook(ctx, p, reflection.Response); err != nil {
			return "", false, goerr.Wrap(err, "failed to call PlanCompletedHook")
		}

		return reflection.Response, true, nil
	}

	// Update plan
	logger.Debug("updating plan based on reflection", "plan_id", p.id, "step_id", currentStep.ID)
	if err := p.updatePlan(reflection); err != nil {
		logger.Debug("plan update failed", "plan_id", p.id, "step_id", currentStep.ID, "error", err)
		return "", false, goerr.Wrap(err, "failed to update plan")
	}

	// Call PlanToDoUpdatedHook if there are changes
	if len(reflection.Changes) > 0 {
		if err := p.callToDoUpdatedHook(ctx, reflection.Changes); err != nil {
			return "", false, err
		}
	}

	logger.Debug("plan updated successfully, continuing execution", "plan_id", p.id, "step_id", currentStep.ID)
	return "", false, nil
}

// callStepStartHooks calls all step start hooks
func (p *Plan) callStepStartHooks(ctx context.Context, currentStep *planToDo) error {
	// Call internal step start hook
	if err := p.config.internalStepStartHook(ctx, p, currentStep); err != nil {
		return goerr.Wrap(err, "failed to call internal step start hook")
	}

	// Call public ToDo start hook
	todo := currentStep.toPlanToDo()
	if err := p.config.planToDoStartHook(ctx, p, todo); err != nil {
		return goerr.Wrap(err, "failed to call public ToDo start hook")
	}

	return nil
}

// callStepCompletedHooks calls all step completed hooks
func (p *Plan) callStepCompletedHooks(ctx context.Context, currentStep *planToDo, result *toDoResult) error {
	// Call internal step completed hook
	if err := p.config.internalStepCompletedHook(ctx, p, currentStep, result); err != nil {
		return goerr.Wrap(err, "failed to call internal step completed hook")
	}

	// Call public ToDo completed hook
	todo := currentStep.toPlanToDo()
	if err := p.config.planToDoCompletedHook(ctx, p, todo); err != nil {
		return goerr.Wrap(err, "failed to call public ToDo completed hook")
	}

	return nil
}

// callToDoUpdatedHook calls the todo updated hook
func (p *Plan) callToDoUpdatedHook(ctx context.Context, changes []PlanToDoChange) error {
	if err := p.config.planToDoUpdatedHook(ctx, p, changes); err != nil {
		return goerr.Wrap(err, "failed to call PlanToDoUpdatedHook")
	}
	return nil
}

// callMessageHook calls the message hook
func (p *Plan) callMessageHook(ctx context.Context, message PlanExecutionMessage) error {
	if err := p.config.planMessageHook(ctx, p, message); err != nil {
		return goerr.Wrap(err, "failed to call PlanMessageHook")
	}
	return nil
}

// handleStepError handles step execution errors
func (p *Plan) handleStepError(step *planToDo, err error) error {
	step.Status = ToDoStatusFailed
	step.Error = err
	step.ErrorMsg = err.Error()
	p.state = PlanStateFailed
	return goerr.Wrap(err, "plan step execution failed", goerr.V("step_id", step.ID))
}

// createPlanConfig creates plan configuration from options
func (g *Agent) createPlanConfig(options ...PlanOption) *planConfig {
	cfg := &planConfig{
		gollemConfig: *g.gollemConfig.Clone(),

		// Default public hooks
		planCreatedHook:       defaultPlanCreatedHook,
		planToDoStartHook:     defaultPlanToDoStartHook,
		planToDoCompletedHook: defaultPlanToDoCompletedHook,
		planCompletedHook:     defaultPlanCompletedHook,
		planToDoUpdatedHook:   defaultPlanToDoUpdatedHook,
		planMessageHook:       defaultPlanMessageHook,

		// Default internal hooks
		internalStepStartHook:     defaultInternalStepStartHook,
		internalStepCompletedHook: defaultInternalStepCompletedHook,
		internalToDoUpdatedHook:   defaultInternalToDoUpdatedHook,
		internalMessageHook:       defaultInternalMessageHook,
		internalReflectionHook:    defaultInternalReflectionHook,

		// Default settings
		// (reserved for future use)
	}

	for _, opt := range options {
		opt(cfg)
	}

	return cfg
}

// createPlanWithRuntime creates a plan with all runtime fields initialized
func (g *Agent) createPlanWithRuntime(id, input string, todos []planToDo, state PlanState, toolMap map[string]Tool, toolList []Tool, cfg *planConfig, logger *slog.Logger, ctx context.Context) (*Plan, error) {
	// Create independent session for this plan (not connected to Agent session)
	sessionOptions := []SessionOption{}
	if cfg.history != nil {
		sessionOptions = append(sessionOptions, WithSessionHistory(cfg.history))
	}
	// CRITICAL: Add tools to main session so LLM knows what tools are available
	sessionOptions = append(sessionOptions, WithSessionTools(toolList...))

	// DEBUG: Log tools being added to session
	if logger != nil {
		toolNames := make([]string, len(toolList))
		for i, tool := range toolList {
			toolNames[i] = tool.Spec().Name
		}
		logger.Debug("creating plan session with tools", "tool_count", len(toolList), "tools", toolNames)
	}

	mainSession, err := g.llm.NewSession(ctx, sessionOptions...)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create main session for plan")
	}

	// DEBUG: Verify session was created with tools
	if logger != nil {
		// Try to access session config to verify tools were set
		logger.Debug("plan session created successfully", "session_type", fmt.Sprintf("%T", mainSession))
	}

	plan := &Plan{
		id:    id,
		input: input,
		todos: todos,
		state: state,

		// Runtime fields
		agent:       g,
		toolMap:     toolMap,
		config:      cfg,
		mainSession: mainSession,
		logger:      logger,
	}

	return plan, nil
}

// createPlannerSession creates a session for plan generation
func (g *Agent) createPlannerSession(ctx context.Context, cfg *planConfig, _ []Tool) (Session, error) {
	sessionOptions := []SessionOption{
		WithSessionContentType(ContentTypeJSON),
	}

	if cfg.history != nil {
		sessionOptions = append(sessionOptions, WithSessionHistory(cfg.history))
	}

	return g.llm.NewSession(ctx, sessionOptions...)
}

// generatePlan generates initial plan using LLM
func (g *Agent) generatePlan(ctx context.Context, session Session, prompt string, availableTools []Tool, systemPrompt string) ([]planToDo, error) {
	// Organize tool information (direct specification is prohibited, but provide information about capabilities)
	toolCapabilities := make([]string, len(availableTools))
	for i, tool := range availableTools {
		spec := tool.Spec()
		toolCapabilities[i] = fmt.Sprintf("- %s: %s", spec.Name, spec.Description)
	}
	toolInfo := strings.Join(toolCapabilities, "\n")

	// Use template for prompt generation
	var promptBuffer bytes.Buffer
	templateData := plannerTemplateData{
		ToolInfo:     toolInfo,
		Goal:         prompt,
		SystemPrompt: systemPrompt,
	}

	if err := plannerTmpl.Execute(&promptBuffer, templateData); err != nil {
		return nil, goerr.Wrap(err, "failed to execute planner template")
	}

	response, err := session.GenerateContent(ctx, Text(promptBuffer.String()))
	if err != nil {
		return nil, err
	}

	if len(response.Texts) == 0 {
		return nil, goerr.New("no response from planner")
	}

	var planData struct {
		Steps []struct {
			Description string `json:"description"`
			Intent      string `json:"intent"`
		} `json:"steps"`
	}

	if err := json.Unmarshal([]byte(response.Texts[0]), &planData); err != nil {
		return nil, goerr.Wrap(err, "failed to parse plan")
	}

	todos := make([]planToDo, 0, len(planData.Steps))
	now := time.Now()
	for _, s := range planData.Steps {
		// Skip steps with empty descriptions
		if strings.TrimSpace(s.Description) == "" {
			continue
		}

		todos = append(todos, planToDo{
			ID:          fmt.Sprintf("todo_%d", len(todos)+1),
			Description: strings.TrimSpace(s.Description),
			Intent:      strings.TrimSpace(s.Intent),
			Status:      ToDoStatusPending,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}

	return todos, nil
}

// executeStep executes a single plan step
func (p *Plan) executeStep(ctx context.Context, todo *planToDo) (*toDoResult, error) {
	logger := LoggerFromContext(ctx)

	todo.Status = ToDoStatusExecuting

	logger.Debug("start executeStep", "todo", todo)

	// Use main session for step execution to maintain history continuity
	if p.mainSession == nil {
		return nil, goerr.Wrap(ErrPlanNotInitialized, "plan main session is not initialized")
	}
	executorSession := p.mainSession

	// Generate execution prompt (using template)
	var promptBuffer bytes.Buffer
	templateData := executorTemplateData{
		Intent:          todo.Intent,
		ProgressSummary: p.getProgressSummary(),
		SystemPrompt:    p.config.systemPrompt,
	}

	if err := executorTmpl.Execute(&promptBuffer, templateData); err != nil {
		return nil, goerr.Wrap(err, "failed to execute executor template")
	}

	// Tool execution (process tools with GenerateContent return value)
	response, err := executorSession.GenerateContent(ctx, Text(promptBuffer.String()))
	if err != nil {
		return nil, goerr.Wrap(err, "executor session failed")
	}
	logger.Debug("got response", "response", response)

	// Send response message through hook
	if len(response.Texts) > 0 {
		responseMessage := PlanExecutionMessage{
			Type:      PlanMessageTypeResponse,
			Content:   strings.Join(response.Texts, "\n"),
			TodoID:    todo.ID,
			Timestamp: time.Now(),
		}
		if err := p.callMessageHook(ctx, responseMessage); err != nil {
			logger.Warn("failed to call message hook for response", "error", err)
		}
	}

	// Process response (use existing handleResponse)
	// Requirement: Tool usage must process Tools from GenerateContent return value
	output := strings.Join(response.Texts, "\n")

	result := &toDoResult{
		Output:     output,
		ToolCalls:  response.FunctionCalls, // []*FunctionCall type
		ExecutedAt: time.Now(),
	}

	// Process tool call results (use existing handleResponse pattern)
	if len(response.FunctionCalls) > 0 {
		logger := LoggerFromContext(ctx)
		logger.Debug("processing tool calls", "plan_id", p.id, "tool_calls_count", len(response.FunctionCalls))

		newInput, err := handleResponse(ctx, p.config.gollemConfig, response, p.toolMap)
		if err != nil {
			// Special handling for ErrExitConversation
			if errors.Is(err, ErrExitConversation) {
				logger.Debug("conversation exit requested by tool", "plan_id", p.id, "tool_calls", response.FunctionCalls)
				// Process step as successful but mark plan completion
				result.Output += "\n[Conversation exit requested by tool]"
				return result, nil
			}
			logger.Debug("tool execution failed", "plan_id", p.id, "error", err)
			return nil, goerr.Wrap(err, "tool execution failed")
		}

		// Store tool results in Data
		result.Data = make(map[string]any)
		for _, input := range newInput {
			if funcResp, ok := input.(FunctionResponse); ok {
				result.Data[funcResp.Name] = funcResp.Data
			}
		}
	}

	return result, nil
}

// reflect analyzes execution results and determines next actions
func (p *Plan) reflect(ctx context.Context) (*planReflection, error) {
	// Create reflection session
	reflectorSession, err := p.agent.createReflectorSession(ctx, p.config)
	if err != nil {
		return nil, err
	}

	// Generate reflection prompt (using template)
	var promptBuffer bytes.Buffer
	templateData := reflectorTemplateData{
		Goal:           p.input,
		OriginalPlan:   p.getPlanSummary(),
		CompletedSteps: p.getCompletedStepsSummary(),
		LastStepResult: p.getLastStepResult(),
		SystemPrompt:   p.config.systemPrompt,
	}

	if err := reflectorTmpl.Execute(&promptBuffer, templateData); err != nil {
		return nil, goerr.Wrap(err, "failed to execute reflector template")
	}

	response, err := reflectorSession.GenerateContent(ctx, Text(promptBuffer.String()))
	if err != nil {
		return nil, err
	}

	if len(response.Texts) == 0 {
		return nil, goerr.New("no response from reflector")
	}

	var reflection planReflection
	if err := json.Unmarshal([]byte(response.Texts[0]), &reflection); err != nil {
		// If JSON parsing fails, process as text response
		logger := LoggerFromContext(ctx)
		logger.Debug("reflection JSON parse failed, using fallback",
			"plan_id", p.id,
			"error", err,
			"response_text", response.Texts[0])

		shouldContinue := false
		reflection = planReflection{
			ShouldContinue:   &shouldContinue,
			Response:         response.Texts[0],
			CompletionReason: "manual_completion",
		}
	} else {
		// Validate that ShouldContinue field was provided
		logger := LoggerFromContext(ctx)
		if reflection.ShouldContinue == nil {
			logger.Debug("reflection missing should_continue field, defaulting to false",
				"plan_id", p.id,
				"response_text", response.Texts[0])
			shouldContinue := false
			reflection.ShouldContinue = &shouldContinue
		}
	}

	return &reflection, nil
}

// createReflectorSession creates a session for reflection
func (g *Agent) createReflectorSession(ctx context.Context, cfg *planConfig) (Session, error) {
	sessionOptions := []SessionOption{
		WithSessionContentType(ContentTypeJSON),
	}

	if cfg.history != nil {
		sessionOptions = append(sessionOptions, WithSessionHistory(cfg.history))
	}

	return g.llm.NewSession(ctx, sessionOptions...)
}

// Helper methods for Plan
func (p *Plan) getPendingToDos() []planToDo {
	var pending []planToDo
	for _, step := range p.todos {
		if step.Status == ToDoStatusPending {
			pending = append(pending, step)
		}
	}
	return pending
}

func (p *Plan) getNextPendingToDo() *planToDo {
	for i := range p.todos {
		if p.todos[i].Status == ToDoStatusPending {
			return &p.todos[i]
		}
	}
	return nil
}

func (p *Plan) getCompletedToDos() []planToDo {
	var completed []planToDo
	for _, step := range p.todos {
		if step.Status == ToDoStatusCompleted {
			completed = append(completed, step)
		}
	}
	return completed
}

func (p *Plan) getProgressSummary() string {
	var summary strings.Builder
	completed := p.getCompletedToDos()

	if len(completed) == 0 {
		summary.WriteString("No steps completed yet.")
	} else {
		summary.WriteString("Completed steps:\n")
		for _, step := range completed {
			summary.WriteString(fmt.Sprintf("- %s: %s\n", step.Description, step.Result.Output))
		}
	}

	return summary.String()
}

func (p *Plan) getPlanSummary() string {
	var summary strings.Builder
	for i, step := range p.todos {
		summary.WriteString(fmt.Sprintf("%d. %s\n", i+1, step.Description))
	}
	return summary.String()
}

func (p *Plan) getCompletedStepsSummary() string {
	var summary strings.Builder
	for _, step := range p.todos {
		if step.Status == ToDoStatusCompleted {
			summary.WriteString(fmt.Sprintf("- %s: %s\n", step.Description, step.Result.Output))
		}
	}
	return summary.String()
}

func (p *Plan) getLastStepResult() string {
	for i := len(p.todos) - 1; i >= 0; i-- {
		if p.todos[i].Status == ToDoStatusCompleted && p.todos[i].Result != nil {
			return p.todos[i].Result.Output
		}
	}
	return "No completed steps yet."
}

func (p *Plan) updatePlan(reflection *planReflection) error {
	now := time.Now()
	var changes []PlanToDoChange

	// Track changes for updated todos
	if len(reflection.UpdatedToDos) > 0 {
		// Create a map of existing todos for comparison
		existingTodos := make(map[string]*planToDo)
		for i := range p.todos {
			existingTodos[p.todos[i].ID] = &p.todos[i]
		}

		// Replace existing incomplete todos with updated todos
		newToDos := p.getCompletedToDos()
		for _, updatedTodo := range reflection.UpdatedToDos {
			updatedTodo.UpdatedAt = now
			if oldTodo, exists := existingTodos[updatedTodo.ID]; exists {
				// This is an update to an existing todo
				oldTodoCopy := *oldTodo
				changes = append(changes, PlanToDoChange{
					Type:        PlanToDoChangeTypeUpdated,
					TodoID:      updatedTodo.ID,
					OldToDo:     &oldTodoCopy,
					NewToDo:     &updatedTodo,
					Description: fmt.Sprintf("Updated todo: %s", updatedTodo.Description),
				})
			}
		}
		newToDos = append(newToDos, reflection.UpdatedToDos...)
		p.todos = newToDos
	}

	// Track changes for new todos
	if len(reflection.NewToDos) > 0 {
		for _, newTodo := range reflection.NewToDos {
			newTodo.CreatedAt = now
			newTodo.UpdatedAt = now
			changes = append(changes, PlanToDoChange{
				Type:        PlanToDoChangeTypeAdded,
				TodoID:      newTodo.ID,
				NewToDo:     &newTodo,
				Description: fmt.Sprintf("Added new todo: %s", newTodo.Description),
			})
		}
		p.todos = append(p.todos, reflection.NewToDos...)
	}

	// Determine reflection type based on changes
	if len(changes) > 0 {
		hasUpdates := false
		hasAdded := false
		for _, change := range changes {
			switch change.Type {
			case PlanToDoChangeTypeUpdated:
				hasUpdates = true
			case PlanToDoChangeTypeAdded:
				hasAdded = true
			}
		}

		if hasUpdates && hasAdded {
			reflection.Type = PlanReflectionTypeExpand
		} else if hasUpdates {
			reflection.Type = PlanReflectionTypeRefine
		} else if hasAdded {
			reflection.Type = PlanReflectionTypeExpand
		}
	} else {
		reflection.Type = PlanReflectionTypeContinue
	}

	// Store changes in reflection for hook calls
	reflection.Changes = changes

	return nil
}

// Serialization methods

// planData represents serializable plan data (private)
type planData struct {
	Version int        `json:"version"`
	ID      string     `json:"id"`
	Input   string     `json:"input"`
	ToDos   []planToDo `json:"todos"`
	State   PlanState  `json:"state"`
}

const (
	PlanVersion = 1 // Follow existing HistoryVersion pattern
)

// Serialize serializes the plan to JSON
func (p *Plan) Serialize() ([]byte, error) {
	return json.Marshal(p)
}

// MarshalJSON implements json.Marshaler interface for Plan
func (p *Plan) MarshalJSON() ([]byte, error) {
	data := planData{
		Version: PlanVersion,
		ID:      p.id,
		Input:   p.input,
		ToDos:   p.todos,
		State:   p.state,
	}
	return json.Marshal(data)
}

// NewPlanFromData creates a plan from serialized JSON data (use existing setupTools pattern)
func (g *Agent) NewPlanFromData(ctx context.Context, data []byte, options ...PlanOption) (*Plan, error) {
	var planData planData
	if err := json.Unmarshal(data, &planData); err != nil {
		return nil, goerr.Wrap(err, "failed to unmarshal plan data")
	}

	// Version check (follow existing History pattern)
	if planData.Version != PlanVersion {
		return nil, goerr.Wrap(ErrInvalidHistoryData, "plan version mismatch",
			goerr.V("expected", PlanVersion), goerr.V("actual", planData.Version))
	}

	cfg := g.createPlanConfig(options...)

	// Rebuild tool map (use existing setupTools)
	toolMap, toolList, err := setupTools(ctx, &cfg.gollemConfig)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to setup tools for deserialized plan")
	}

	// Create plan with runtime fields using shared logic
	logger := cfg.logger.With("gollem.plan_id", planData.ID)
	plan, err := g.createPlanWithRuntime(planData.ID, planData.Input, planData.ToDos, planData.State, toolMap, toolList, cfg, logger, context.Background())
	if err != nil {
		return nil, goerr.Wrap(err, "failed to recreate plan with runtime fields")
	}

	return plan, nil
}

// Session returns the session used by this plan for conversation history.
// This session is independent from the Agent's session and maintains the plan's context.
// The session is immutable once set during plan creation or deserialization.
// Returns nil if the plan is not properly initialized (e.g., only unmarshaled without Agent.NewPlanFromData).
func (p *Plan) Session() Session {
	return p.mainSession
}

// UnmarshalJSON implements json.Unmarshaler interface for Plan
// Note: This method requires the Plan to be associated with an Agent after unmarshaling
// Use Agent.NewPlanFromData() for complete restoration
func (p *Plan) UnmarshalJSON(data []byte) error {
	var planData planData
	if err := json.Unmarshal(data, &planData); err != nil {
		return goerr.Wrap(err, "failed to unmarshal plan data")
	}

	// Version check
	if planData.Version != PlanVersion {
		return goerr.Wrap(ErrInvalidHistoryData, "plan version mismatch",
			goerr.V("expected", PlanVersion), goerr.V("actual", planData.Version))
	}

	// Set the unmarshaled data
	p.id = planData.ID
	p.input = planData.Input
	p.todos = planData.ToDos
	p.state = planData.State

	// Runtime fields (agent, toolMap, config, mainSession) remain nil
	// These need to be set separately via Agent.NewPlanFromData()

	return nil
}

// Public methods for Plan inspection

// GetToDos returns a copy of all todos with their status (external reference)
func (p *Plan) GetToDos() []PlanToDo {
	todos := make([]PlanToDo, len(p.todos))
	for i, todo := range p.todos {
		todos[i] = PlanToDo{
			ID:          todo.ID,
			Description: todo.Description,
			Intent:      todo.Intent,
			Status:      toDoStatusToString(todo.Status),
			Completed:   todo.Status == ToDoStatusCompleted,
			Error:       todo.Error,
			ErrorMsg:    todo.ErrorMsg,
			Result:      todo.copyResult(),
		}
	}
	return todos
}

// PlanToDo represents a public view of a todo (external reference structure)
type PlanToDo struct {
	ID          string
	Description string
	Intent      string
	Status      string
	Completed   bool
	Error       error
	ErrorMsg    string
	Result      *PlanToDoResult
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// PlanToDoResult represents a public view of todo execution result
type PlanToDoResult struct {
	Output     string
	ToolCalls  []*FunctionCall
	Data       map[string]any
	ExecutedAt time.Time
}

// These are now handled by the main public hook types above

// Helper method to convert internal todo to public structure
func (p *planToDo) toPlanToDo() PlanToDo {
	return PlanToDo{
		ID:          p.ID,
		Description: p.Description,
		Intent:      p.Intent,
		Status:      toDoStatusToString(p.Status),
		Completed:   p.Status == ToDoStatusCompleted,
		Error:       p.Error,
		ErrorMsg:    p.ErrorMsg,
		Result:      p.copyResult(),
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

// Helper method to copy result data for external reference
func (p *planToDo) copyResult() *PlanToDoResult {
	if p.Result == nil {
		return nil
	}

	// Deep copy of Data map
	data := make(map[string]any)
	maps.Copy(data, p.Result.Data)

	return &PlanToDoResult{
		Output:     p.Result.Output,
		ToolCalls:  p.Result.ToolCalls, // []*FunctionCall is immutable after creation
		Data:       data,
		ExecutedAt: p.Result.ExecutedAt,
	}
}

func toDoStatusToString(status ToDoStatus) string {
	switch status {
	case ToDoStatusPending:
		return "Pending"
	case ToDoStatusExecuting:
		return "Executing"
	case ToDoStatusCompleted:
		return "Completed"
	case ToDoStatusFailed:
		return "Failed"
	case ToDoStatusSkipped:
		return "Skipped"
	default:
		return "Unknown"
	}
}

// Plan option methods

// WithPlanCreatedHook sets a hook for plan creation
func WithPlanCreatedHook(hook PlanCreatedHook) PlanOption {
	return func(cfg *planConfig) {
		cfg.planCreatedHook = hook
	}
}

// WithPlanToDoStartHook sets a hook for plan todo start
func WithPlanToDoStartHook(hook PlanToDoStartHook) PlanOption {
	return func(cfg *planConfig) {
		cfg.planToDoStartHook = hook
	}
}

// WithPlanToDoCompletedHook sets a hook for plan todo completion
func WithPlanToDoCompletedHook(hook PlanToDoCompletedHook) PlanOption {
	return func(cfg *planConfig) {
		cfg.planToDoCompletedHook = hook
	}
}

// These hooks are deprecated, use WithPlanToDoStartHook/WithPlanToDoCompletedHook instead

// WithPlanCompletedHook sets a hook for plan completion
func WithPlanCompletedHook(hook PlanCompletedHook) PlanOption {
	return func(cfg *planConfig) {
		cfg.planCompletedHook = hook
	}
}

// WithPlanSystemPrompt sets the system prompt for plan execution
func WithPlanSystemPrompt(systemPrompt string) PlanOption {
	return func(cfg *planConfig) {
		cfg.systemPrompt = systemPrompt
	}
}

// Public hook option functions

// WithToDoStartHook sets a hook for todo start (public API)
func WithToDoStartHook(hook PlanToDoStartHook) PlanOption {
	return func(cfg *planConfig) {
		cfg.planToDoStartHook = hook
	}
}

// WithToDoCompletedHook sets a hook for todo completion (public API)
func WithToDoCompletedHook(hook PlanToDoCompletedHook) PlanOption {
	return func(cfg *planConfig) {
		cfg.planToDoCompletedHook = hook
	}
}

// WithPlanToDoUpdatedHook sets a hook for todo updates/refinements
func WithPlanToDoUpdatedHook(hook PlanToDoUpdatedHook) PlanOption {
	return func(cfg *planConfig) {
		cfg.planToDoUpdatedHook = hook
	}
}

// WithPlanMessageHook sets a hook for plan execution messages
func WithPlanMessageHook(hook PlanMessageHook) PlanOption {
	return func(cfg *planConfig) {
		cfg.planMessageHook = hook
	}
}

// These hooks are already provided by WithPlanToDoUpdatedHook and WithPlanMessageHook

// WithPlanHistory sets the history for plan execution (plan-specific)
func WithPlanHistory(history *History) PlanOption {
	return func(cfg *planConfig) {
		cfg.history = history
	}
}
