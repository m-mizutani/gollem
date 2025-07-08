package gollem

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/m-mizutani/goerr/v2"
)

// Plan represents an executable plan
type Plan struct {
	// Internal state (may be processed asynchronously except during Execute execution)
	id      string
	input   string
	todos   []planToDo
	state   PlanState
	history *History // For maintaining state between plans

	// Fields reconstructed at runtime (not serialized)
	agent   *Agent          `json:"-"`
	toolMap map[string]Tool `json:"-"`
	config  *planConfig     `json:"-"`
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
	maxPlanSteps      int
	reflectionEnabled bool
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

	plan := &Plan{
		id:      planID,
		input:   prompt,
		todos:   todos,
		state:   PlanStateCreated,
		history: cfg.history,

		// Runtime fields
		agent:   g,
		toolMap: toolMap,
		config:  cfg,
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
	if p.state != PlanStateCreated {
		return "", ErrPlanAlreadyExecuted
	}

	p.state = PlanStateRunning

	// Restore runtime fields (when deserialized)
	if p.agent == nil {
		return "", ErrPlanNotInitialized
	}

	logger := LoggerFromContext(ctx) // Use existing context.go function

	for len(p.getPendingToDos()) > 0 {
		currentStep := p.getNextPendingToDo()
		if currentStep == nil {
			break
		}

		// Call PlanStepStartHook
		if p.config.planStepStartHook != nil {
			if err := p.config.planStepStartHook(ctx, p, currentStep); err != nil {
				return "", goerr.Wrap(err, "failed to call PlanStepStartHook")
			}
		}

		// Call public ToDo start hook
		if p.config.publicToDoStartHook != nil {
			todo := currentStep.toPlanToDo()
			if err := p.config.publicToDoStartHook(ctx, p, todo); err != nil {
				return "", goerr.Wrap(err, "failed to call public ToDo start hook")
			}
		}

		// Execute step
		result, err := p.executeStep(ctx, currentStep)
		if err != nil {
			currentStep.Status = ToDoStatusFailed
			currentStep.Error = err
			currentStep.ErrorMsg = err.Error()
			p.state = PlanStateFailed
			return "", goerr.Wrap(err, "plan step execution failed", goerr.V("step_id", currentStep.ID))
		}

		currentStep.Status = ToDoStatusCompleted
		currentStep.Result = result

		// Call PlanStepCompletedHook
		if p.config.planStepCompletedHook != nil {
			if err := p.config.planStepCompletedHook(ctx, p, currentStep, result); err != nil {
				return "", goerr.Wrap(err, "failed to call PlanStepCompletedHook")
			}
		}

		// Call public ToDo completed hook
		if p.config.publicToDoCompletedHook != nil {
			todo := currentStep.toPlanToDo()
			if err := p.config.publicToDoCompletedHook(ctx, p, todo); err != nil {
				return "", goerr.Wrap(err, "failed to call public ToDo completed hook")
			}
		}

		// Reflection and re-planning
		reflection, err := p.reflect(ctx)
		if err != nil {
			return "", goerr.Wrap(err, "plan reflection failed")
		}

		// Call PlanReflectionHook
		if p.config.planReflectionHook != nil {
			if err := p.config.planReflectionHook(ctx, p, reflection); err != nil {
				return "", goerr.Wrap(err, "failed to call PlanReflectionHook")
			}
		}

		if !reflection.ShouldContinue {
			p.state = PlanStateCompleted

			// Call PlanCompletedHook
			if p.config.planCompletedHook != nil {
				if err := p.config.planCompletedHook(ctx, p, reflection.Response); err != nil {
					return "", goerr.Wrap(err, "failed to call PlanCompletedHook")
				}
			}

			logger.Info("plan completed successfully",
				"plan_id", p.id,
				"todos_executed", len(p.getCompletedToDos()))

			return reflection.Response, nil
		}

		// Update plan
		if err := p.updatePlan(reflection); err != nil {
			return "", goerr.Wrap(err, "failed to update plan")
		}
	}

	p.state = PlanStateCompleted
	logger.Info("plan completed - all steps processed", "plan_id", p.id)
	return "Plan completed", nil
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
		maxPlanSteps:      10,
		reflectionEnabled: true,
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

	// Create executor session
	executorSession, err := p.agent.createExecutorSession(ctx, p.config, p.getToolList())
	if err != nil {
		return nil, err
	}

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

// createExecutorSession creates a session for step execution
func (g *Agent) createExecutorSession(ctx context.Context, cfg *planConfig, toolList []Tool) (Session, error) {
	sessionOptions := []SessionOption{}

	if cfg.history != nil {
		sessionOptions = append(sessionOptions, WithSessionHistory(cfg.history))
	}
	if len(toolList) > 0 {
		sessionOptions = append(sessionOptions, WithSessionTools(toolList...))
	}

	return g.llm.NewSession(ctx, sessionOptions...)
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

func (p *Plan) getToolList() []Tool {
	tools := make([]Tool, 0, len(p.toolMap))
	for _, tool := range p.toolMap {
		tools = append(tools, tool)
	}
	return tools
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
	History *History   `json:"history"`
}

const (
	PlanVersion = 1 // Follow existing HistoryVersion pattern
)

// Serialize serializes the plan to JSON
func (p *Plan) Serialize() ([]byte, error) {
	data := planData{
		Version: PlanVersion,
		ID:      p.id,
		Input:   p.input,
		ToDos:   p.todos,
		State:   p.state,
		History: p.history,
	}
	return json.Marshal(data)
}

// DeserializePlan deserializes a plan from JSON (use existing setupTools pattern)
func (g *Agent) DeserializePlan(data []byte, options ...PlanOption) (*Plan, error) {
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

	return &Plan{
		id:      planData.ID,
		input:   planData.Input,
		todos:   planData.ToDos,
		state:   planData.State,
		history: planData.History,

		agent:   g,
		toolMap: toolMap,
		config:  cfg,
	}, nil
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
	for k, v := range p.Result.Data {
		data[k] = v
	}
	
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

// WithMaxPlanSteps sets the maximum number of steps in a plan
func WithMaxPlanSteps(max int) PlanOption {
	return func(cfg *planConfig) {
		cfg.maxPlanSteps = max
	}
}

// WithReflectionEnabled enables or disables reflection between steps
func WithReflectionEnabled(enabled bool) PlanOption {
	return func(cfg *planConfig) {
		cfg.reflectionEnabled = enabled
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
