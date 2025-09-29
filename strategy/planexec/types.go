package planexec

import (
	"context"

	"github.com/m-mizutani/gollem"
)

// TaskState represents the current state of a task
type TaskState string

const (
	TaskStatePending    TaskState = "pending"
	TaskStateInProgress TaskState = "in_progress"
	TaskStateCompleted  TaskState = "completed"
)

// Task represents an executable task in the plan
type Task struct {
	Description string
	State       TaskState
	Result      string
}

// Plan represents the execution plan with tasks
type Plan struct {
	Goal           string
	Tasks          []Task
	DirectResponse string // Used when no plan is needed
}

// PlanExecuteHooks provides hook points for plan lifecycle events
type PlanExecuteHooks struct {
	OnPlanCreated func(ctx context.Context, plan *Plan) error
	OnPlanUpdated func(ctx context.Context, plan *Plan) error
}

// PlanExecuteStrategy implements the Strategy interface for plan-and-execute approach
type PlanExecuteStrategy struct {
	client        gollem.LLMClient
	middleware    []gollem.ContentBlockMiddleware
	hooks         PlanExecuteHooks
	maxIterations int

	// Runtime state
	plan           *Plan
	currentTask    *Task
	waitingForTask bool
}

// PlanExecuteOption is a functional option for configuring PlanExecuteStrategy
type PlanExecuteOption func(*PlanExecuteStrategy)
