package gollem

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/m-mizutani/goerr/v2"
	"github.com/secmon-lab/warren/pkg/utils/clock"
)

// Plan represents an executable plan
type Plan struct {
	// Internal state (may be processed asynchronously except during Execute execution)
	id              string
	input           string
	interpretedGoal string // LLM-interpreted and articulated goal from user input
	todos           []planToDo
	state           PlanState

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

// PlanExecutionMode represents how the plan should handle task execution
type PlanExecutionMode string

const (
	PlanExecutionModeComplete  PlanExecutionMode = "complete"  // Execute all tasks without skipping
	PlanExecutionModeBalanced  PlanExecutionMode = "balanced"  // Default adaptive mode
	PlanExecutionModeEfficient PlanExecutionMode = "efficient" // Aggressively skip when possible
)

// Public constants for external use
const (
	PlanExecutionComplete  = PlanExecutionModeComplete
	PlanExecutionBalanced  = PlanExecutionModeBalanced
	PlanExecutionEfficient = PlanExecutionModeEfficient
)

// SkipDecision represents a decision to skip a specific todo with detailed reasoning
type SkipDecision struct {
	TodoID     string  `json:"todo_id"`
	SkipReason string  `json:"skip_reason"`
	Confidence float64 `json:"confidence"` // 0.0-1.0, higher means more confident
	Evidence   string  `json:"evidence"`   // Supporting evidence for the decision
}

// Validate validates the SkipDecision structure
func (s *SkipDecision) Validate() error {
	if s.TodoID == "" {
		return goerr.New("skip decision: todo_id cannot be empty")
	}
	if s.SkipReason == "" {
		return goerr.New("skip decision: skip_reason cannot be empty", goerr.Value("todo_id", s.TodoID))
	}
	if s.Confidence < 0.0 || s.Confidence > 1.0 {
		return goerr.New("skip decision: confidence must be between 0.0 and 1.0",
			goerr.Value("todo_id", s.TodoID), goerr.Value("confidence", s.Confidence))
	}
	return nil
}

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

	// PlanSkipConfirmationHook is called when a skip decision is made, returns true to approve skip
	PlanSkipConfirmationHook func(ctx context.Context, plan *Plan, decision SkipDecision) bool
)

// Internal hook types for library implementation (unexported)
type (
	// planStepStartHook is called when a plan todo starts execution (internal)
	planStepStartHook func(ctx context.Context, plan *Plan, todo *planToDo) error

	// planStepCompletedHook is called when a plan todo completes successfully (internal)
	planStepCompletedHook func(ctx context.Context, plan *Plan, todo *planToDo, result *toDoResult) error

	// planToDoUpdatedHook is called when todos are updated/refined during reflection (internal)
	planToDoUpdatedHook func(ctx context.Context, plan *Plan, changes []PlanToDoChange) error
)

// planReflection represents the result of plan reflection (private)
type planReflection struct {
	Type             PlanReflectionType `json:"reflection_type"`
	UpdatedToDos     []planToDo         `json:"updated_todos,omitempty"`
	NewToDos         []planToDo         `json:"new_todos,omitempty"`
	SkippedToDos     []string           `json:"skipped_todos,omitempty"`  // Todo IDs to skip (legacy)
	SkipDecisions    []SkipDecision     `json:"skip_decisions,omitempty"` // Enhanced skip decisions with reasoning
	CompletionReason string             `json:"completion_reason,omitempty"`
	Response         string             `json:"response,omitempty"`
	Changes          []PlanToDoChange   `json:"changes,omitempty"` // Detailed changes
}

// Validate validates the planReflection structure
func (r *planReflection) Validate() error {
	// Validate legacy skipped todos
	for i, skippedTodoID := range r.SkippedToDos {
		if skippedTodoID == "" {
			return goerr.New("skipped_todos: todo_id cannot be empty", goerr.Value("index", i))
		}
	}

	// Validate skip decisions
	for i, skipDecision := range r.SkipDecisions {
		if err := skipDecision.Validate(); err != nil {
			return goerr.Wrap(err, "skip decision validation failed", goerr.Value("index", i))
		}
	}

	// Validate updated todos
	for i, todo := range r.UpdatedToDos {
		if err := validateToDo(todo, fmt.Sprintf("updated_todos[%d]", i)); err != nil {
			return err
		}
	}

	// Validate new todos
	for i, todo := range r.NewToDos {
		if err := validateToDo(todo, fmt.Sprintf("new_todos[%d]", i)); err != nil {
			return err
		}
	}

	return nil
}

// validateToDo validates a single planToDo structure
func validateToDo(todo planToDo, context string) error {
	if todo.ID == "" {
		return goerr.New("todo_id cannot be empty", goerr.Value("context", context))
	}
	if todo.Description == "" {
		return goerr.New("todo_description cannot be empty", goerr.Value("context", context), goerr.Value("todo_id", todo.ID))
	}
	if todo.Intent == "" {
		return goerr.New("todo_intent cannot be empty", goerr.Value("context", context), goerr.Value("todo_id", todo.ID))
	}
	return nil
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

	// Internal hooks for library implementation
	internalStepStartHook     planStepStartHook
	internalStepCompletedHook planStepCompletedHook
	internalToDoUpdatedHook   planToDoUpdatedHook
	internalReflectionHook    PlanReflectionHook // Keep as exported since reflection is complex

	// Plan-specific settings
	executionMode           PlanExecutionMode
	skipConfidenceThreshold float64
	skipConfirmationHook    PlanSkipConfirmationHook
	maxToolRetries          int // Maximum number of recursive tool calls
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

func defaultPlanSkipConfirmationHook(ctx context.Context, plan *Plan, decision SkipDecision) bool {
	// Default behavior: approve skip based on confidence threshold
	return decision.Confidence >= 0.8 // High confidence threshold by default
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

	// Interpret user goal first
	interpretedGoal, err := g.interpretUserGoal(ctx, prompt, cfg)
	if err != nil {
		logger.Warn("failed to interpret user goal, using original prompt", "error", err)
		interpretedGoal = prompt // Fallback to original prompt
	}

	logger.Debug("user goal interpreted",
		"original_prompt", prompt,
		"interpreted_goal", interpretedGoal)

	// Create planner session
	plannerSession, err := g.createPlannerSession(ctx, cfg, toolList)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create planner session")
	}

	// Generate plan (use interpreted goal for planning)
	todos, err := g.generatePlan(ctx, plannerSession, interpretedGoal, toolList, cfg.systemPrompt)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to generate plan")
	}

	// Create plan with runtime fields
	plan, err := g.createPlanWithRuntime(planID, prompt, interpretedGoal, todos, PlanStateCreated, toolMap, toolList, cfg, logger, ctx)
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
	logger := p.logger.With("gollem.plan_id", p.id)
	ctx = ctxWithLogger(ctx, logger)
	ctx = ctxWithPlan(ctx, p)

	logger.Debug("plan execute started", "state", p.state)

	if err := p.validateAndPrepareExecution(); err != nil {
		logger.Debug("plan validation failed", "error", err)
		return "", err
	}

	logger.Debug("plan validation passed, starting execution")
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
	logger.Debug("executeSteps started", "pending_todos_count", len(p.getPendingToDos()))

	for len(p.getPendingToDos()) > 0 {
		currentToDo := p.getNextPendingToDo()
		if currentToDo == nil {
			logger.Debug("no more pending todos found")
			break
		}

		ctx = ctxWithPlanToDo(ctx, currentToDo)

		logger.Debug("processing plan step",
			"todo_id", currentToDo.ID,
			"step_description", currentToDo.Description,
			"pending_count", len(p.getPendingToDos()))

		// Process single step
		result, shouldComplete, err := p.processSingleStep(ctx, currentToDo)
		if err != nil {
			logger.Error("plan step processing failed",
				"todo_id", currentToDo.ID,
				"error", err)
			return "", err
		}

		if shouldComplete {
			logger.Debug("plan completed successfully by reflection",
				"todo_id", currentToDo.ID,
				"todos_executed", len(p.getCompletedToDos()),
				"result", result)
			return result, nil
		}

		logger.Debug("plan step completed, continuing to next step",
			"todo_id", currentToDo.ID,
			"remaining_pending", len(p.getPendingToDos()))
	}

	p.state = PlanStateCompleted

	logger.Info("plan completed - all steps processed")

	// Generate comprehensive execution summary
	summary, err := p.generateExecutionSummary(ctx)
	if err != nil {
		logger.Warn("failed to generate execution summary, using fallback", "error", err)
		return "Plan completed successfully", nil
	}

	return summary, nil
}

// processSingleStep processes a single step including hooks, execution, and reflection
func (p *Plan) processSingleStep(ctx context.Context, currentStep *planToDo) (string, bool, error) {
	logger := LoggerFromContext(ctx)
	logger.Debug("starting step processing", "step_id", currentStep.ID, "step_intent", currentStep.Intent)

	// Call step start hooks
	if err := p.callStepStartHooks(ctx, currentStep); err != nil {
		return "", false, err
	}

	logger.Debug("executing step", "step_id", currentStep.ID)
	// Execute step
	result, err := p.executeStep(ctx, currentStep)
	if err != nil {
		logger.Debug("step execution failed", "step_id", currentStep.ID, "error", err)
		return "", false, p.handleStepError(currentStep, err)
	}

	logger.Debug("step execution completed", "step_id", currentStep.ID, "output_length", len(result.Output), "tool_calls_count", len(result.ToolCalls))
	currentStep.Status = ToDoStatusCompleted
	currentStep.Result = result

	// Call step completed hooks
	if err := p.callStepCompletedHooks(ctx, currentStep, result); err != nil {
		return "", false, err
	}

	// Reflection and re-planning
	logger.Debug("starting plan reflection", "step_id", currentStep.ID)
	reflection, err := p.reflect(ctx)
	if err != nil {
		logger.Debug("plan reflection failed", "step_id", currentStep.ID, "error", err)
		return "", false, goerr.Wrap(err, "plan reflection failed")
	}

	logger.Debug("plan reflection completed",
		"plan_id", p.id,
		"completion_reason", reflection.CompletionReason,
		"new_todos_count", len(reflection.NewToDos),
		"updated_todos_count", len(reflection.UpdatedToDos),
		"skipped_todos_count", len(reflection.SkippedToDos),
		"reflection_type", reflection.Type,
		"response_length", len(reflection.Response))

	// Call internal reflection hook
	if err := p.config.internalReflectionHook(ctx, p, reflection); err != nil {
		return "", false, goerr.Wrap(err, "failed to call internal reflection hook")
	}

	// Update plan
	logger.Debug("updating plan based on reflection", "step_id", currentStep.ID)
	if err := p.updatePlan(reflection); err != nil {
		logger.Debug("plan update failed", "step_id", currentStep.ID, "error", err)
		return "", false, goerr.Wrap(err, "failed to update plan")
	}

	// Call PlanToDoUpdatedHook if there are changes
	if len(reflection.Changes) > 0 {
		if err := p.callToDoUpdatedHook(ctx, reflection.Changes); err != nil {
			return "", false, err
		}
	}

	// Determine if execution should continue based on TODO status
	pendingTodos := p.getPendingToDos()
	if len(pendingTodos) == 0 {
		// No pending todos - plan is complete
		p.state = PlanStateCompleted

		// Set completion type based on whether there were changes
		if len(reflection.Changes) > 0 {
			reflection.Type = PlanReflectionTypeRefinedDone
		} else {
			reflection.Type = PlanReflectionTypeComplete
		}

		logger.Debug("plan completed - no pending todos remaining", "step_id", currentStep.ID)

		// Generate comprehensive execution summary
		summary, err := p.generateExecutionSummary(ctx)
		if err != nil {
			logger.Warn("failed to generate execution summary, using reflection response", "error", err)
			summary = reflection.Response
		}

		// Call PlanCompletedHook
		if err := p.config.planCompletedHook(ctx, p, summary); err != nil {
			return "", false, goerr.Wrap(err, "failed to call PlanCompletedHook")
		}

		return summary, true, nil
	}

	// Continue execution with remaining todos
	logger.Debug("plan updated successfully, continuing execution", "step_id", currentStep.ID)
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
		skipConfirmationHook:  defaultPlanSkipConfirmationHook,

		// Default internal hooks
		internalStepStartHook:     defaultInternalStepStartHook,
		internalStepCompletedHook: defaultInternalStepCompletedHook,
		internalToDoUpdatedHook:   defaultInternalToDoUpdatedHook,
		internalReflectionHook:    defaultInternalReflectionHook,

		// Default plan-specific settings
		executionMode:           PlanExecutionModeBalanced,
		skipConfidenceThreshold: 0.8,
		maxToolRetries:          10,
	}

	for _, opt := range options {
		opt(cfg)
	}

	return cfg
}

// createPlanWithRuntime creates a plan with all runtime fields initialized
func (g *Agent) createPlanWithRuntime(id, input, interpretedGoal string, todos []planToDo, state PlanState, toolMap map[string]Tool, toolList []Tool, cfg *planConfig, logger *slog.Logger, ctx context.Context) (*Plan, error) {
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
		logger.Debug("plan session tools verification", "tools_added_count", len(toolList))
		for i, tool := range toolList {
			logger.Debug("plan session tool", "index", i, "name", tool.Spec().Name, "description", tool.Spec().Description)
		}
	}

	plan := &Plan{
		id:              id,
		input:           input,
		interpretedGoal: interpretedGoal,
		todos:           todos,
		state:           state,

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
		return nil, goerr.Wrap(err, "failed to parse plan", goerr.V("response_text", response.Texts[0]))
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

// executeStepWithInput handles recursive tool processing with maximum retry control
func executeStepWithInput(ctx context.Context, session Session, config gollemConfig, toolMap map[string]Tool, todo *planToDo, inputs []Input, maxRetries int) (*toDoResult, error) {
	logger := LoggerFromContext(ctx)

	result := &toDoResult{
		Output:     "",
		ToolCalls:  []*FunctionCall{},
		ExecutedAt: clock.Now(ctx),
		Data:       make(map[string]any),
	}

	// Add timeout for processing
	stepCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	response, err := session.GenerateContent(stepCtx, inputs...)
	if err != nil {
		return nil, goerr.Wrap(err, "session GenerateContent failed")
	}

	logger.Debug("executeStepWithInput: received response", "texts_count", len(response.Texts), "function_calls_count", len(response.FunctionCalls))
	
	// DEBUG: Log all function calls in detail
	for i, fc := range response.FunctionCalls {
		logger.Debug("executeStepWithInput: function call detail", "index", i, "name", fc.Name, "args", fc.Arguments)
	}
	
	// DEBUG: Log actual response text to understand why tools weren't called
	for i, text := range response.Texts {
		logger.Debug("executeStepWithInput: response text", "index", i, "content", text)
	}

	// Process text response
	if len(response.Texts) > 0 {
		textOutput := strings.Join(response.Texts, "\n")
		if result.Output == "" {
			result.Output = textOutput
		} else {
			result.Output += "\n" + textOutput
		}
	}

	// Add function calls to result
	result.ToolCalls = append(result.ToolCalls, response.FunctionCalls...)

	// Check retry limit
	var additionalInput []Input
	if maxRetries <= -3 {
		return nil, goerr.New("maximum hard-limit retries exceeded", goerr.V("todo", todo))
	}
	if maxRetries <= 0 {
		logger.Warn("maximum retries exceeded, stopping tool processing", "max_retries", maxRetries)

		if len(result.ToolCalls) == 0 {
			return result, nil
		}

		additionalInput = append(additionalInput, Text("IMPORTANT: maximum retries already exceeded, more tool call is not allowed"))
	}

	logger.Debug("executeStepWithInput: processing tool calls", "tool_calls_count", len(response.FunctionCalls), "retries_remaining", maxRetries)

	// Process tool calls
	newInput, err := handleResponse(ctx, config, response, toolMap)
	if err != nil {
		logger.Debug("executeStepWithInput: tool processing failed", "error", err)
		return nil, goerr.Wrap(err, "tool execution failed")
	}
	newInput = append(newInput, additionalInput...)

	// Store tool results in Data
	for _, input := range newInput {
		if funcResp, ok := input.(FunctionResponse); ok {
			result.Data[funcResp.Name] = funcResp.Data
		}
	}

	// Recursively process with tool results if any
	if len(newInput) > 0 {
		logger.Debug("executeStepWithInput: recursively processing tool results", "input_count", len(newInput), "retries_remaining", maxRetries-1)

		recursiveResult, err := executeStepWithInput(ctx, session, config, toolMap, todo, newInput, maxRetries-1)
		if err != nil {
			return nil, goerr.Wrap(err, "recursive processing failed")
		}

		// Merge recursive results
		if recursiveResult.Output != "" {
			if result.Output == "" {
				result.Output = recursiveResult.Output
			} else {
				result.Output += "\n" + recursiveResult.Output
			}
		}
		result.ToolCalls = append(result.ToolCalls, recursiveResult.ToolCalls...)
		maps.Copy(result.Data, recursiveResult.Data)
	}

	return result, nil
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

	// Generate execution prompt (using template)
	var promptBuffer bytes.Buffer
	if err := executorTmpl.Execute(&promptBuffer, executorTemplateData{
		Intent:          todo.Intent,
		ProgressSummary: p.getProgressSummary(),
		SystemPrompt:    p.config.systemPrompt,
	}); err != nil {
		return nil, goerr.Wrap(err, "failed to execute executor template")
	}

	// Execute with initial prompt and configured maximum retries
	result, err := executeStepWithInput(ctx, p.mainSession, p.config.gollemConfig, p.toolMap, todo, []Input{Text(promptBuffer.String())}, p.config.maxToolRetries)
	if err != nil {
		return nil, goerr.Wrap(err, "executeStepWithInput failed")
	}

	return result, nil
}

// reflect analyzes execution results and determines next actions
func (p *Plan) reflect(ctx context.Context) (*planReflection, error) {
	logger := LoggerFromContext(ctx)
	logger.Debug("starting reflection")

	// Create reflection session
	reflectorSession, err := p.agent.createReflectorSession(ctx, p.config)
	if err != nil {
		logger.Debug("failed to create reflector session", "error", err)
		return nil, err
	}
	logger.Debug("reflector session created")

	// Generate reflection prompt (using template)
	var promptBuffer bytes.Buffer
	templateData := reflectorTemplateData{
		Goal:              p.input,
		InterpretedGoal:   p.interpretedGoal,
		CurrentPlanStatus: p.getCurrentPlanStatus(),
		OriginalPlan:      p.getPlanSummary(),
		CompletedSteps:    p.getCompletedStepsSummary(),
		LastStepResult:    p.getLastStepResult(),
		SystemPrompt:      p.config.systemPrompt,
	}

	if err := reflectorTmpl.Execute(&promptBuffer, templateData); err != nil {
		logger.Debug("failed to execute reflector template", "error", err)
		return nil, goerr.Wrap(err, "failed to execute reflector template")
	}
	logger.Debug("reflector prompt generated", "prompt_length", promptBuffer.Len())

	// Add timeout for reflection to prevent hanging
	reflectionCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	logger.Debug("sending reflection request to LLM")
	response, err := reflectorSession.GenerateContent(reflectionCtx, Text(promptBuffer.String()))
	if err != nil {
		logger.Debug("reflection request failed", "error", err)
		return nil, err
	}
	logger.Debug("reflection response received", "response_length", len(response.Texts))

	if len(response.Texts) == 0 {
		logger.Debug("no response from reflector")
		return nil, goerr.New("no response from reflector")
	}

	logger.Debug("parsing reflection response", "response_text", response.Texts[0])
	var reflection planReflection
	if err := json.Unmarshal([]byte(response.Texts[0]), &reflection); err != nil {
		// If JSON parsing fails, process as text response
		logger.Debug("reflection JSON parse failed, using fallback",
			"error", err,
			"response_text", response.Texts[0])

		reflection = planReflection{
			Response:         response.Texts[0],
			CompletionReason: "reflection_fallback_used",
		}
	} else {
		// Validate reflection
		if err := reflection.Validate(); err != nil {
			logger.Debug("reflection validation failed, using fallback",
				"error", err,
				"response_text", response.Texts[0])

			reflection = planReflection{
				Response:         response.Texts[0],
				CompletionReason: "reflection_validation_failed",
			}
		} else {
			logger.Debug("reflection validation successful", "type", reflection.Type)
		}
	}

	logger.Debug("reflection completed", "type", reflection.Type)
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

// interpretUserGoal interprets and articulates the user's input goal using LLM
func (g *Agent) interpretUserGoal(ctx context.Context, userInput string, cfg *planConfig) (string, error) {
	// Create goal interpretation session
	sessionOptions := []SessionOption{}
	if cfg.history != nil {
		sessionOptions = append(sessionOptions, WithSessionHistory(cfg.history))
	}

	goalSession, err := g.llm.NewSession(ctx, sessionOptions...)
	if err != nil {
		return "", goerr.Wrap(err, "failed to create goal interpretation session")
	}

	// Generate goal interpretation prompt using template
	var promptBuffer bytes.Buffer
	templateData := goalInterpreterTemplateData{
		UserInput:    userInput,
		SystemPrompt: cfg.systemPrompt,
	}

	if err := goalInterpreterTmpl.Execute(&promptBuffer, templateData); err != nil {
		return "", goerr.Wrap(err, "failed to execute goal interpreter template")
	}

	response, err := goalSession.GenerateContent(ctx, Text(promptBuffer.String()))
	if err != nil {
		return "", goerr.Wrap(err, "failed to interpret user goal")
	}

	if len(response.Texts) == 0 {
		return "", goerr.New("no response from goal interpretation")
	}

	interpretedGoal := strings.Join(response.Texts, "\n")
	interpretedGoal = strings.TrimSpace(interpretedGoal)

	return interpretedGoal, nil
}

// generateExecutionSummary creates a comprehensive summary of plan execution results
func (p *Plan) generateExecutionSummary(ctx context.Context) (string, error) {
	logger := LoggerFromContext(ctx)
	logger.Debug("generating execution summary")

	// Create summarizer session
	summarizerSession, err := p.agent.createSummarizerSession(ctx, p.config)
	if err != nil {
		logger.Debug("failed to create summarizer session", "error", err)
		return "", err
	}

	// Prepare execution details
	executionDetails := p.getDetailedExecutionReport()
	overallStatus := p.getCurrentPlanStatus()

	// Generate summary prompt using template
	var promptBuffer bytes.Buffer
	templateData := summarizerTemplateData{
		Goal:             p.input,
		InterpretedGoal:  p.interpretedGoal,
		ExecutionDetails: executionDetails,
		OverallStatus:    overallStatus,
		SystemPrompt:     p.config.systemPrompt,
	}

	if err := summarizerTmpl.Execute(&promptBuffer, templateData); err != nil {
		logger.Debug("failed to execute summarizer template", "error", err)
		return "", goerr.Wrap(err, "failed to execute summarizer template")
	}

	// Add timeout for summary generation
	summaryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	logger.Debug("sending summary request to LLM")
	response, err := summarizerSession.GenerateContent(summaryCtx, Text(promptBuffer.String()))
	if err != nil {
		logger.Debug("summary request failed", "error", err)
		return "", err
	}

	if len(response.Texts) == 0 {
		logger.Debug("no response from summarizer")
		return "", goerr.New("no response from summarizer")
	}

	summary := strings.Join(response.Texts, "\n")
	logger.Debug("execution summary generated", "summary_length", len(summary))

	return summary, nil
}

// createSummarizerSession creates a session for summary generation
func (g *Agent) createSummarizerSession(ctx context.Context, cfg *planConfig) (Session, error) {
	sessionOptions := []SessionOption{}

	if cfg.history != nil {
		sessionOptions = append(sessionOptions, WithSessionHistory(cfg.history))
	}

	return g.llm.NewSession(ctx, sessionOptions...)
}

// getDetailedExecutionReport provides detailed information about plan execution
func (p *Plan) getDetailedExecutionReport() string {
	var report strings.Builder

	// Count todos by status
	var completed, pending, failed, skipped int
	for _, todo := range p.todos {
		switch todo.Status {
		case ToDoStatusCompleted:
			completed++
		case ToDoStatusPending:
			pending++
		case ToDoStatusFailed:
			failed++
		case ToDoStatusSkipped:
			skipped++
		}
	}

	report.WriteString("## Execution Statistics\n")
	report.WriteString(fmt.Sprintf("- Total Tasks: %d\n", len(p.todos)))
	report.WriteString(fmt.Sprintf("- Completed: %d\n", completed))
	if pending > 0 {
		report.WriteString(fmt.Sprintf("- Pending: %d\n", pending))
	}
	if failed > 0 {
		report.WriteString(fmt.Sprintf("- Failed: %d\n", failed))
	}
	if skipped > 0 {
		report.WriteString(fmt.Sprintf("- Skipped: %d\n", skipped))
	}

	// Detailed task breakdown
	report.WriteString("\n## Task Details\n")

	// Completed tasks
	if completed > 0 {
		report.WriteString("\n### ✅ Completed Tasks\n")
		for _, todo := range p.todos {
			if todo.Status == ToDoStatusCompleted {
				report.WriteString(fmt.Sprintf("- **%s**: %s\n", todo.Description, todo.Intent))
				if todo.Result != nil && todo.Result.Output != "" {
					// Truncate long outputs
					output := todo.Result.Output
					if len(output) > 200 {
						output = output[:200] + "..."
					}
					report.WriteString(fmt.Sprintf("  Result: %s\n", output))
				}
				if todo.Result != nil && len(todo.Result.ToolCalls) > 0 {
					report.WriteString(fmt.Sprintf("  Tools used: %d\n", len(todo.Result.ToolCalls)))
				}
			}
		}
	}

	// Failed tasks
	if failed > 0 {
		report.WriteString("\n### ❌ Failed Tasks\n")
		for _, todo := range p.todos {
			if todo.Status == ToDoStatusFailed {
				report.WriteString(fmt.Sprintf("- **%s**: %s\n", todo.Description, todo.Intent))
				if todo.ErrorMsg != "" {
					report.WriteString(fmt.Sprintf("  Error: %s\n", todo.ErrorMsg))
				}
			}
		}
	}

	// Skipped tasks
	if skipped > 0 {
		report.WriteString("\n### ⏭️ Skipped Tasks\n")
		for _, todo := range p.todos {
			if todo.Status == ToDoStatusSkipped {
				report.WriteString(fmt.Sprintf("- **%s**: %s\n", todo.Description, todo.Intent))
			}
		}
	}

	// Pending tasks (if any)
	if pending > 0 {
		report.WriteString("\n### ⏳ Remaining Tasks\n")
		for _, todo := range p.todos {
			if todo.Status == ToDoStatusPending {
				report.WriteString(fmt.Sprintf("- **%s**: %s\n", todo.Description, todo.Intent))
			}
		}
	}

	return report.String()
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
			output := "No output"
			if step.Result != nil {
				output = step.Result.Output
			}
			summary.WriteString(fmt.Sprintf("- %s: %s\n", step.Description, output))
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
			output := "No output"
			if step.Result != nil {
				output = step.Result.Output
			}
			summary.WriteString(fmt.Sprintf("- %s: %s\n", step.Description, output))
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

func (p *Plan) getCurrentPlanStatus() string {
	var summary strings.Builder

	// Count todos by status
	var completed, pending, failed, skipped int
	for _, todo := range p.todos {
		switch todo.Status {
		case ToDoStatusCompleted:
			completed++
		case ToDoStatusPending:
			pending++
		case ToDoStatusFailed:
			failed++
		case ToDoStatusSkipped:
			skipped++
		}
	}

	total := len(p.todos)
	summary.WriteString(fmt.Sprintf("Total tasks: %d\n", total))
	summary.WriteString(fmt.Sprintf("✅ Completed: %d\n", completed))
	summary.WriteString(fmt.Sprintf("⏳ Pending: %d\n", pending))
	if failed > 0 {
		summary.WriteString(fmt.Sprintf("❌ Failed: %d\n", failed))
	}
	if skipped > 0 {
		summary.WriteString(fmt.Sprintf("⏭️ Skipped: %d\n", skipped))
	}

	return summary.String()
}

func (p *Plan) updatePlan(reflection *planReflection) error {
	now := time.Now()
	var changes []PlanToDoChange

	// Validate skip decisions and legacy skipped todos for conflicts
	skipMap := make(map[string]bool)
	for _, skippedTodoID := range reflection.SkippedToDos {
		skipMap[skippedTodoID] = true
	}
	for _, skipDecision := range reflection.SkipDecisions {
		if skipMap[skipDecision.TodoID] {
			p.logger.Warn("todo appears in both skipped_todos and skip_decisions, using skip_decisions",
				"todo_id", skipDecision.TodoID)
		}
		skipMap[skipDecision.TodoID] = true
	}

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
		for i := range reflection.NewToDos {
			newTodo := &reflection.NewToDos[i]
			if newTodo.ID == "" {
				newTodo.ID = uuid.New().String()
			}
			newTodo.CreatedAt = now
			newTodo.UpdatedAt = now
			changes = append(changes, PlanToDoChange{
				Type:        PlanToDoChangeTypeAdded,
				TodoID:      newTodo.ID,
				NewToDo:     newTodo,
				Description: fmt.Sprintf("Added new todo: %s", newTodo.Description),
			})
		}
		p.todos = append(p.todos, reflection.NewToDos...)
	}

	// Track changes for skipped todos
	if len(reflection.SkippedToDos) > 0 {
		for _, skippedTodoID := range reflection.SkippedToDos {
			// Find the todo to skip
			for i := range p.todos {
				if p.todos[i].ID == skippedTodoID {
					oldTodo := p.todos[i]
					p.todos[i].Status = ToDoStatusSkipped
					p.todos[i].UpdatedAt = now
					changes = append(changes, PlanToDoChange{
						Type:        PlanToDoChangeTypeUpdated,
						TodoID:      skippedTodoID,
						OldToDo:     &oldTodo,
						NewToDo:     &p.todos[i],
						Description: fmt.Sprintf("Skipped todo: %s", p.todos[i].Description),
					})
					break
				}
			}
		}
	}

	// Process enhanced skip decisions with confirmation
	if len(reflection.SkipDecisions) > 0 {
		logger := p.logger
		for _, skipDecision := range reflection.SkipDecisions {
			// Check execution mode
			shouldSkip := false
			switch p.config.executionMode {
			case PlanExecutionModeComplete:
				// Never skip in complete mode
				shouldSkip = false
				logger.Debug("skip denied due to complete execution mode",
					"todo_id", skipDecision.TodoID, "reason", skipDecision.SkipReason)
			case PlanExecutionModeEfficient:
				// Skip if confidence meets threshold
				shouldSkip = skipDecision.Confidence >= p.config.skipConfidenceThreshold
				logger.Debug("skip decision in efficient mode",
					"todo_id", skipDecision.TodoID, "confidence", skipDecision.Confidence,
					"threshold", p.config.skipConfidenceThreshold, "approved", shouldSkip)
			case PlanExecutionModeBalanced:
				// Check confidence and get confirmation
				if skipDecision.Confidence >= p.config.skipConfidenceThreshold {
					shouldSkip = p.config.skipConfirmationHook(context.Background(), p, skipDecision)
					logger.Debug("skip decision in balanced mode",
						"todo_id", skipDecision.TodoID, "confidence", skipDecision.Confidence,
						"confirmed", shouldSkip)
				} else {
					logger.Debug("skip denied due to low confidence",
						"todo_id", skipDecision.TodoID, "confidence", skipDecision.Confidence,
						"threshold", p.config.skipConfidenceThreshold)
				}
			}

			if shouldSkip {
				// Find the todo to skip
				for i := range p.todos {
					if p.todos[i].ID == skipDecision.TodoID {
						oldTodo := p.todos[i]
						p.todos[i].Status = ToDoStatusSkipped
						p.todos[i].UpdatedAt = now
						changes = append(changes, PlanToDoChange{
							Type:    PlanToDoChangeTypeUpdated,
							TodoID:  skipDecision.TodoID,
							OldToDo: &oldTodo,
							NewToDo: &p.todos[i],
							Description: fmt.Sprintf("Skipped todo: %s (reason: %s, confidence: %.2f)",
								p.todos[i].Description, skipDecision.SkipReason, skipDecision.Confidence),
						})
						logger.Info("todo skipped",
							"todo_id", skipDecision.TodoID,
							"reason", skipDecision.SkipReason,
							"confidence", skipDecision.Confidence)
						break
					}
				}
			}
		}
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

	// Log skip decision statistics
	if len(reflection.SkipDecisions) > 0 {
		var approvedSkips, deniedSkips int
		var totalConfidence float64

		for _, skipDecision := range reflection.SkipDecisions {
			// Check if this todo was actually skipped
			wasSkipped := false
			for _, change := range changes {
				if change.TodoID == skipDecision.TodoID && change.NewToDo != nil && change.NewToDo.Status == ToDoStatusSkipped {
					wasSkipped = true
					break
				}
			}

			if wasSkipped {
				approvedSkips++
			} else {
				deniedSkips++
			}
			totalConfidence += skipDecision.Confidence
		}

		avgConfidence := totalConfidence / float64(len(reflection.SkipDecisions))
		p.logger.Info("skip decision summary",
			"total_decisions", len(reflection.SkipDecisions),
			"approved", approvedSkips,
			"denied", deniedSkips,
			"avg_confidence", avgConfidence,
			"execution_mode", p.config.executionMode)
	}

	return nil
}

// Serialization methods

// planData represents serializable plan data (private)
type planData struct {
	Version         int        `json:"version"`
	ID              string     `json:"id"`
	Input           string     `json:"input"`
	InterpretedGoal string     `json:"interpreted_goal"`
	ToDos           []planToDo `json:"todos"`
	State           PlanState  `json:"state"`
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
		Version:         PlanVersion,
		ID:              p.id,
		Input:           p.input,
		InterpretedGoal: p.interpretedGoal,
		ToDos:           p.todos,
		State:           p.state,
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
	plan, err := g.createPlanWithRuntime(planData.ID, planData.Input, planData.InterpretedGoal, planData.ToDos, planData.State, toolMap, toolList, cfg, logger, context.Background())
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
	p.interpretedGoal = planData.InterpretedGoal
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
		todos[i] = todo.toPlanToDo()
	}
	return todos
}

// GetInterpretedGoal returns the LLM-interpreted and articulated goal from user input
func (p *Plan) GetInterpretedGoal() string {
	return p.interpretedGoal
}

// GetOriginalInput returns the original user input prompt
func (p *Plan) GetOriginalInput() string {
	return p.input
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

// These hooks are already provided by WithPlanToDoUpdatedHook and WithPlanMessageHook

// WithPlanHistory sets the history for plan execution (plan-specific)
func WithPlanHistory(history *History) PlanOption {
	return func(cfg *planConfig) {
		cfg.history = history
	}
}

// WithPlanExecutionMode sets the execution mode for the plan.
// Default: PlanExecutionModeBalanced
func WithPlanExecutionMode(mode PlanExecutionMode) PlanOption {
	return func(cfg *planConfig) {
		cfg.executionMode = mode
	}
}

// WithSkipConfidenceThreshold sets the confidence threshold for skip decisions.
// Default: 0.8 (80% confidence required)
func WithSkipConfidenceThreshold(threshold float64) PlanOption {
	return func(cfg *planConfig) {
		cfg.skipConfidenceThreshold = threshold
	}
}

// WithSkipConfirmationHook sets a hook to confirm skip decisions.
// Default: Auto-approve skip decisions that meet the confidence threshold (0.8)
func WithSkipConfirmationHook(hook PlanSkipConfirmationHook) PlanOption {
	return func(cfg *planConfig) {
		cfg.skipConfirmationHook = hook
	}
}

// WithMaxToolRetries sets the maximum number of recursive tool calls allowed during step execution.
// Default: 10
func WithMaxToolRetries(maxRetries int) PlanOption {
	return func(cfg *planConfig) {
		cfg.maxToolRetries = maxRetries
	}
}
