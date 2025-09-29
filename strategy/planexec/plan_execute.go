package planexec

import (
	"context"

	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
)

// NewPlanExecuteStrategy creates a new PlanExecuteStrategy instance
func NewPlanExecuteStrategy(opts ...PlanExecuteOption) *PlanExecuteStrategy {
	s := &PlanExecuteStrategy{
		maxIterations: 20, // default max iterations
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
	return nil
}

// Handle determines the next input for the LLM based on the current state
func (s *PlanExecuteStrategy) Handle(ctx context.Context, state *gollem.StrategyState) ([]gollem.Input, *gollem.ExecuteResponse, error) {
	logger := ctxlog.From(ctx)
	logger.Debug("plan-execute strategy handle", "iteration", state.Iteration)

	// ========== Phase 1: Initialization and Planning ==========
	if state.Iteration == 0 {
		if s.client == nil {
			return nil, nil, goerr.New("LLM client is not set")
		}

		// Analyze and create plan using LLM
		plan, err := analyzeAndPlan(ctx, s.client, state.InitInput, s.middleware)
		if err != nil {
			return nil, nil, goerr.Wrap(err, "failed to analyze and plan")
		}
		s.plan = plan

		// No plan needed - return direct response
		if len(plan.Tasks) == 0 {
			return nil, &gollem.ExecuteResponse{
				Texts: []string{plan.DirectResponse},
			}, nil
		}

		// Hook: plan created
		if s.hooks.OnPlanCreated != nil {
			if err := s.hooks.OnPlanCreated(ctx, plan); err != nil {
				return nil, nil, goerr.Wrap(err, "hook OnPlanCreated failed")
			}
		}
		// Proceed to phase 3 to select first task
	}

	// ========== Phase 2: Task Result Processing and Reflection ==========
	if s.waitingForTask && state.LastResponse != nil {
		// Save task result
		s.currentTask.Result = parseTaskResult(ctx, state.LastResponse)
		s.currentTask.State = TaskStateCompleted
		s.waitingForTask = false

		// Perform reflection
		updatedPlan, shouldContinue, err := reflect(ctx, s.client, s.plan, s.middleware)
		if err != nil {
			return nil, nil, goerr.Wrap(err, "reflection failed")
		}

		// Update plan if changed
		if updatedPlan != nil {
			s.plan = updatedPlan
			if s.hooks.OnPlanUpdated != nil {
				if err := s.hooks.OnPlanUpdated(ctx, s.plan); err != nil {
					return nil, nil, goerr.Wrap(err, "hook OnPlanUpdated failed")
				}
			}
		}

		// Check if we should terminate
		if !shouldContinue || allTasksCompleted(ctx, s.plan) {
			return nil, generateFinalResponse(ctx, s.plan), nil
		}
		// Proceed to phase 3 to select next task
	}

	// ========== Phase 3: Next Task Selection and Execution ==========
	if !s.waitingForTask {
		s.currentTask = getNextPendingTask(ctx, s.plan)

		// All tasks completed
		if s.currentTask == nil {
			return nil, generateFinalResponse(ctx, s.plan), nil
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

// WithLLMClient sets the LLM client for the strategy
func WithLLMClient(client gollem.LLMClient) PlanExecuteOption {
	return func(s *PlanExecuteStrategy) {
		s.client = client
	}
}

// WithMiddleware sets the content block middleware
func WithMiddleware(middleware []gollem.ContentBlockMiddleware) PlanExecuteOption {
	return func(s *PlanExecuteStrategy) {
		s.middleware = middleware
	}
}

// WithHooks sets the lifecycle hooks
func WithHooks(hooks PlanExecuteHooks) PlanExecuteOption {
	return func(s *PlanExecuteStrategy) {
		s.hooks = hooks
	}
}

// WithMaxIterations sets the maximum number of iterations
func WithMaxIterations(max int) PlanExecuteOption {
	return func(s *PlanExecuteStrategy) {
		s.maxIterations = max
	}
}
