package planexec

import (
	"context"

	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
)

// New creates a new PlanExecuteStrategy instance
func New(client gollem.LLMClient, opts ...PlanExecuteOption) *PlanExecuteStrategy {
	s := &PlanExecuteStrategy{
		client:        client,
		maxIterations: DefaultMaxIterations,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Init initializes the strategy with initial inputs
func (s *PlanExecuteStrategy) Init(ctx context.Context, inputs []gollem.Input) error {
	// Initialize strategy state
	s.plan = nil
	s.currentTask = nil
	s.waitingForTask = false
	s.taskIterationCount = 0
	return nil
}

// Handle determines the next input for the LLM based on the current state
func (s *PlanExecuteStrategy) Handle(ctx context.Context, state *gollem.StrategyState) ([]gollem.Input, *gollem.ExecuteResponse, error) {
	logger := ctxlog.From(ctx)
	logger.Debug("plan-execute strategy handle",
		"iteration", state.Iteration,
		"next_input_len", len(state.NextInput),
		"last_response_nil", state.LastResponse == nil)

	// ========== Phase 0: Pass through NextInput (e.g., tool responses) ==========
	// If there's pending input (like tool responses), we must send it to the LLM
	// before proceeding with strategy logic.
	// IMPORTANT: Don't pass through on iteration 0 - that's the initial input for planning
	if state.Iteration > 0 && len(state.NextInput) > 0 {
		logger.Debug("passing through NextInput", "count", len(state.NextInput))
		return state.NextInput, nil, nil
	}

	// ========== Phase 1: Initialization and Planning ==========
	if state.Iteration == 0 {
		if s.client == nil {
			return nil, nil, goerr.New("LLM client is not set")
		}

		// Analyze and create plan using LLM
		plan, err := analyzeAndPlan(ctx, s.client, state.InitInput, state.Tools, s.middleware)
		if err != nil {
			return nil, nil, goerr.Wrap(err, "failed to analyze and plan")
		}
		s.plan = plan

		// Hook: plan created (always call after plan is created)
		if s.hooks.OnPlanCreated != nil {
			if err := s.hooks.OnPlanCreated(ctx, plan); err != nil {
				return nil, nil, goerr.Wrap(err, "hook OnPlanCreated failed")
			}
		}

		// No plan needed - return direct response
		if len(plan.Tasks) == 0 {
			return nil, &gollem.ExecuteResponse{
				Texts: []string{plan.DirectResponse},
			}, nil
		}
		// Proceed to phase 3 to select first task
	}

	// ========== Phase 2: Task Result Processing and Reflection ==========
	if s.waitingForTask && state.LastResponse != nil {
		// Save task result
		if s.currentTask == nil {
			return nil, nil, goerr.New("unexpected state: waiting for task but no current task is set")
		}
		s.currentTask.Result = parseTaskResult(state.LastResponse)
		s.currentTask.State = TaskStateCompleted
		s.waitingForTask = false
		s.taskIterationCount++

		// Check max iteration limit (safety net against infinite loops)
		if s.taskIterationCount >= s.maxIterations {
			finalResponse, err := getFinalConclusion(ctx, s.client, s.plan, s.middleware)
			if err != nil {
				logger.Debug("failed to generate conclusion, using simple summary", "error", err.Error())
				return nil, generateFinalResponse(ctx, s.plan), nil
			}
			return nil, finalResponse, nil
		}

		// Perform reflection only if enabled
		reflectionResult, err := reflect(ctx, s.client, s.plan, state.Tools, s.middleware)
		if err != nil {
			return nil, nil, goerr.Wrap(err, "reflection failed")
		}

		// Apply task updates from reflection
		if len(reflectionResult.UpdatedTasks) > 0 {
			taskMap := make(map[string]*Task)
			for i := range s.plan.Tasks {
				taskMap[s.plan.Tasks[i].ID] = &s.plan.Tasks[i]
			}
			for _, updatedTask := range reflectionResult.UpdatedTasks {
				if task, exists := taskMap[updatedTask.ID]; exists {
					task.Description = updatedTask.Description
					task.State = updatedTask.State
				}
			}
		}

		// Add new tasks from reflection
		if len(reflectionResult.NewTasks) > 0 {
			s.plan.Tasks = append(s.plan.Tasks, reflectionResult.NewTasks...)
			if s.hooks.OnPlanUpdated != nil {
				if err := s.hooks.OnPlanUpdated(ctx, s.plan); err != nil {
					return nil, nil, goerr.Wrap(err, "hook OnPlanUpdated failed")
				}
			}
		}

		// Proceed to phase 3 to select next task
	}

	// ========== Phase 3: Next Task Selection and Execution ==========
	if !s.waitingForTask {
		s.currentTask = getNextPendingTask(ctx, s.plan)

		// All tasks completed - get final conclusion from LLM
		if s.currentTask == nil {
			finalResponse, err := getFinalConclusion(ctx, s.client, s.plan, s.middleware)
			if err != nil {
				// If conclusion generation fails, fall back to simple summary
				logger.Debug("failed to generate conclusion, using simple summary", "error", err.Error())
				return nil, generateFinalResponse(ctx, s.plan), nil
			}
			return nil, finalResponse, nil
		}

		// Start task execution
		s.currentTask.State = TaskStateInProgress
		s.waitingForTask = true

		// Return task execution prompt
		return buildExecutePrompt(ctx, s.currentTask, s.plan), nil, nil
	}

	// ========== Error: Unexpected State ==========
	return nil, nil, goerr.New("unexpected state in Handle")
}

// Tools returns the tools that this strategy provides
func (s *PlanExecuteStrategy) Tools(ctx context.Context) ([]gollem.Tool, error) {
	// Plan & Execute strategy does not provide additional tools
	return []gollem.Tool{}, nil
}

// Option functions

// WithMiddleware sets the content block middleware
func WithMiddleware(middleware ...gollem.ContentBlockMiddleware) PlanExecuteOption {
	return func(s *PlanExecuteStrategy) {
		s.middleware = append(s.middleware, middleware...)
	}
}

// WithHooks sets the lifecycle hooks
func WithHooks(hooks PlanExecuteHooks) PlanExecuteOption {
	return func(s *PlanExecuteStrategy) {
		s.hooks = hooks
	}
}

// WithMaxIterations sets the maximum number of task execution iterations
func WithMaxIterations(max int) PlanExecuteOption {
	return func(s *PlanExecuteStrategy) {
		s.maxIterations = max
	}
}
