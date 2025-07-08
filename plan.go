package gollem

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// Hook types for monitoring plan lifecycle (following existing naming patterns)
type (
	// PlanCreatedHook is called when a plan is successfully created
	PlanCreatedHook func(ctx context.Context, plan *Plan) error

	// PlanStepStartHook is called when a plan todo starts execution
	PlanStepStartHook func(ctx context.Context, plan *Plan, todo *planToDo) error

	// PlanStepCompletedHook is called when a plan todo completes successfully
	PlanStepCompletedHook func(ctx context.Context, plan *Plan, todo *planToDo, result *toDoResult) error

	// PlanReflectionHook is called after reflection analysis
	PlanReflectionHook func(ctx context.Context, plan *Plan, reflection *planReflection) error

	// PlanCompletedHook is called when the entire plan is completed
	PlanCompletedHook func(ctx context.Context, plan *Plan, result string) error
)

// planReflection represents the result of plan reflection (private)
type planReflection struct {
	ShouldContinue   bool       `json:"should_continue"`
	UpdatedToDos     []planToDo `json:"updated_todos,omitempty"`
	NewToDos         []planToDo `json:"new_todos,omitempty"`
	CompletionReason string     `json:"completion_reason,omitempty"`
	Response         string     `json:"response,omitempty"`
}

// planConfig holds configuration for plan creation and execution
type planConfig struct {
	gollemConfig

	// Plan-specific hooks (internal)
	planCreatedHook       PlanCreatedHook
	planStepStartHook     PlanStepStartHook
	planStepCompletedHook PlanStepCompletedHook
	planReflectionHook    PlanReflectionHook
	planCompletedHook     PlanCompletedHook

	// Public hooks for external API
	publicToDoStartHook     PlanToDoStartPublicHook
	publicToDoCompletedHook PlanToDoCompletedPublicHook

	// Plan-specific settings
	// (reserved for future use)
}

// PlanOption represents configuration options for plan creation and execution
type PlanOption func(*planConfig)

// Default hook implementations
func defaultPlanCreatedHook(ctx context.Context, plan *Plan) error {
	return nil
}

func defaultPlanStepStartHook(ctx context.Context, plan *Plan, todo *planToDo) error {
	return nil
}

func defaultPlanStepCompletedHook(ctx context.Context, plan *Plan, todo *planToDo, result *toDoResult) error {
	return nil
}

func defaultPlanReflectionHook(ctx context.Context, plan *Plan, reflection *planReflection) error {
	return nil
}

func defaultPlanCompletedHook(ctx context.Context, plan *Plan, result string) error {
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
	toolMap, toolList, err := setupTools(ctx, &cfg.gollemConfig)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to setup tools for plan")
	}

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

	// Create independent session for this plan (not connected to Agent session)
	sessionOptions := []SessionOption{}
	if cfg.history != nil {
		sessionOptions = append(sessionOptions, WithSessionHistory(cfg.history))
	}
	mainSession, err := g.llm.NewSession(ctx, sessionOptions...)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create main session for plan")
	}

	plan := &Plan{
		id:    planID,
		input: prompt,
		todos: todos,
		state: PlanStateCreated,

		// Runtime fields
		agent:       g,
		toolMap:     toolMap,
		config:      cfg,
		mainSession: mainSession,
	}

	// Call PlanCreatedHook
	if cfg.planCreatedHook != nil {
		if err := cfg.planCreatedHook(ctx, plan); err != nil {
			return nil, goerr.Wrap(err, "failed to call PlanCreatedHook")
		}
	}

	logger.Info("plan created",
		"plan_id", planID,
		"todos_count", len(todos),
		"prompt", prompt)

	return plan, nil
}

// Execute executes the plan and returns the final result
func (p *Plan) Execute(ctx context.Context) (string, error) {
	if err := p.validateAndPrepareExecution(); err != nil {
		return "", err
	}

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

	for len(p.getPendingToDos()) > 0 {
		currentStep := p.getNextPendingToDo()
		if currentStep == nil {
			break
		}

		// Process single step
		result, shouldComplete, err := p.processSingleStep(ctx, currentStep)
		if err != nil {
			return "", err
		}

		if shouldComplete {
			logger.Info("plan completed successfully",
				"plan_id", p.id,
				"todos_executed", len(p.getCompletedToDos()))
			return result, nil
		}
	}

	p.state = PlanStateCompleted

	logger.Info("plan completed - all steps processed", "plan_id", p.id)
	return "Plan completed", nil
}

// processSingleStep processes a single step including hooks, execution, and reflection
func (p *Plan) processSingleStep(ctx context.Context, currentStep *planToDo) (string, bool, error) {
	// Call step start hooks
	if err := p.callStepStartHooks(ctx, currentStep); err != nil {
		return "", false, err
	}

	// Execute step
	result, err := p.executeStep(ctx, currentStep)
	if err != nil {
		return "", false, p.handleStepError(currentStep, err)
	}

	currentStep.Status = ToDoStatusCompleted
	currentStep.Result = result

	// Call step completed hooks
	if err := p.callStepCompletedHooks(ctx, currentStep, result); err != nil {
		return "", false, err
	}

	// Reflection and re-planning
	reflection, err := p.reflect(ctx)
	if err != nil {
		return "", false, goerr.Wrap(err, "plan reflection failed")
	}

	// Call PlanReflectionHook
	if p.config.planReflectionHook != nil {
		if err := p.config.planReflectionHook(ctx, p, reflection); err != nil {
			return "", false, goerr.Wrap(err, "failed to call PlanReflectionHook")
		}
	}

	if !reflection.ShouldContinue {
		p.state = PlanStateCompleted

		// Call PlanCompletedHook
		if p.config.planCompletedHook != nil {
			if err := p.config.planCompletedHook(ctx, p, reflection.Response); err != nil {
				return "", false, goerr.Wrap(err, "failed to call PlanCompletedHook")
			}
		}

		return reflection.Response, true, nil
	}

	// Update plan
	if err := p.updatePlan(reflection); err != nil {
		return "", false, goerr.Wrap(err, "failed to update plan")
	}

	return "", false, nil
}

// callStepStartHooks calls all step start hooks
func (p *Plan) callStepStartHooks(ctx context.Context, currentStep *planToDo) error {
	// Call PlanStepStartHook
	if p.config.planStepStartHook != nil {
		if err := p.config.planStepStartHook(ctx, p, currentStep); err != nil {
			return goerr.Wrap(err, "failed to call PlanStepStartHook")
		}
	}

	// Call public ToDo start hook
	if p.config.publicToDoStartHook != nil {
		todo := currentStep.toPlanToDo()
		if err := p.config.publicToDoStartHook(ctx, p, todo); err != nil {
			return goerr.Wrap(err, "failed to call public ToDo start hook")
		}
	}

	return nil
}

// callStepCompletedHooks calls all step completed hooks
func (p *Plan) callStepCompletedHooks(ctx context.Context, currentStep *planToDo, result *toDoResult) error {
	// Call PlanStepCompletedHook
	if p.config.planStepCompletedHook != nil {
		if err := p.config.planStepCompletedHook(ctx, p, currentStep, result); err != nil {
			return goerr.Wrap(err, "failed to call PlanStepCompletedHook")
		}
	}

	// Call public ToDo completed hook
	if p.config.publicToDoCompletedHook != nil {
		todo := currentStep.toPlanToDo()
		if err := p.config.publicToDoCompletedHook(ctx, p, todo); err != nil {
			return goerr.Wrap(err, "failed to call public ToDo completed hook")
		}
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

		// Default hooks
		planCreatedHook:       defaultPlanCreatedHook,
		planStepStartHook:     defaultPlanStepStartHook,
		planStepCompletedHook: defaultPlanStepCompletedHook,
		planReflectionHook:    defaultPlanReflectionHook,
		planCompletedHook:     defaultPlanCompletedHook,

		// Public hooks (nil by default)
		publicToDoStartHook:     nil,
		publicToDoCompletedHook: nil,

		// Default settings
		// (reserved for future use)
	}

	for _, opt := range options {
		opt(cfg)
	}

	return cfg
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
		})
	}

	return todos, nil
}

// executeStep executes a single plan step
func (p *Plan) executeStep(ctx context.Context, todo *planToDo) (*toDoResult, error) {
	todo.Status = ToDoStatusExecuting

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
		newInput, err := handleResponse(ctx, p.config.gollemConfig, response, p.toolMap)
		if err != nil {
			// Special handling for ErrExitConversation
			if errors.Is(err, ErrExitConversation) {
				// Process step as successful but mark plan completion
				result.Output += "\n[Conversation exit requested by tool]"
				return result, nil
			}
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
		reflection = planReflection{
			ShouldContinue:   false,
			Response:         response.Texts[0],
			CompletionReason: "manual_completion",
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
	// Update plan with new todos
	if len(reflection.UpdatedToDos) > 0 {
		// Replace existing incomplete todos with updated todos
		newToDos := p.getCompletedToDos()
		newToDos = append(newToDos, reflection.UpdatedToDos...)
		p.todos = newToDos
	}

	if len(reflection.NewToDos) > 0 {
		p.todos = append(p.todos, reflection.NewToDos...)
	}

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
func (g *Agent) NewPlanFromData(data []byte, options ...PlanOption) (*Plan, error) {
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
	toolMap, _, err := setupTools(context.Background(), &cfg.gollemConfig)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to setup tools for deserialized plan")
	}

	// Recreate independent session for deserialized plan
	sessionOptions := []SessionOption{}
	mainSession, err := g.llm.NewSession(context.Background(), sessionOptions...)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to recreate main session for deserialized plan")
	}

	plan := &Plan{
		id:    planData.ID,
		input: planData.Input,
		todos: planData.ToDos,
		state: planData.State,

		agent:       g,
		toolMap:     toolMap,
		config:      cfg,
		mainSession: mainSession,
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
}

// PlanToDoResult represents a public view of todo execution result
type PlanToDoResult struct {
	Output     string
	ToolCalls  []*FunctionCall
	Data       map[string]any
	ExecutedAt time.Time
}

// Public hook function types that use public interfaces
type (
	// PlanCreatedPublicHook is called when a plan is successfully created (public API)
	PlanCreatedPublicHook func(ctx context.Context, plan *Plan) error

	// PlanToDoStartPublicHook is called when a plan todo starts execution (public API)
	PlanToDoStartPublicHook func(ctx context.Context, plan *Plan, todo PlanToDo) error

	// PlanToDoCompletedPublicHook is called when a plan todo completes successfully (public API)
	PlanToDoCompletedPublicHook func(ctx context.Context, plan *Plan, todo PlanToDo) error
)

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
func WithPlanCreatedHook(hook func(ctx context.Context, plan *Plan) error) PlanOption {
	return func(cfg *planConfig) {
		cfg.planCreatedHook = hook
	}
}

// WithPlanStepStartHook sets a hook for step start
func WithPlanStepStartHook(hook func(ctx context.Context, plan *Plan, todo *planToDo) error) PlanOption {
	return func(cfg *planConfig) {
		cfg.planStepStartHook = hook
	}
}

// WithPlanStepCompletedHook sets a hook for step completion
func WithPlanStepCompletedHook(hook func(ctx context.Context, plan *Plan, todo *planToDo, result *toDoResult) error) PlanOption {
	return func(cfg *planConfig) {
		cfg.planStepCompletedHook = hook
	}
}

// WithPlanReflectionHook sets a hook for plan reflection
func WithPlanReflectionHook(hook func(ctx context.Context, plan *Plan, reflection *planReflection) error) PlanOption {
	return func(cfg *planConfig) {
		cfg.planReflectionHook = hook
	}
}

// WithPlanCompletedHook sets a hook for plan completion
func WithPlanCompletedHook(hook func(ctx context.Context, plan *Plan, result string) error) PlanOption {
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
func WithToDoStartHook(hook PlanToDoStartPublicHook) PlanOption {
	return func(cfg *planConfig) {
		cfg.publicToDoStartHook = hook
	}
}

// WithToDoCompletedHook sets a hook for todo completion (public API)
func WithToDoCompletedHook(hook PlanToDoCompletedPublicHook) PlanOption {
	return func(cfg *planConfig) {
		cfg.publicToDoCompletedHook = hook
	}
}

// WithPlanHistory sets the history for plan execution (plan-specific)
func WithPlanHistory(history *History) PlanOption {
	return func(cfg *planConfig) {
		cfg.history = history
	}
}
