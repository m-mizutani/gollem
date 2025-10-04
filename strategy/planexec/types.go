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

	// DefaultMaxIterations is the default maximum number of task execution iterations
	DefaultMaxIterations = 32
)

// Task represents an executable task in the plan
type Task struct {
	ID          string // Unique identifier for the task
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
type PlanExecuteHooks interface {
	OnCreated(ctx context.Context, plan *Plan) error
	OnUpdated(ctx context.Context, plan *Plan) error
}

// Strategy implements the gollem.Strategy interface for plan-and-execute approach
type Strategy struct {
	client        gollem.LLMClient
	middleware    []gollem.ContentBlockMiddleware
	hooks         PlanExecuteHooks
	maxIterations int

	// Runtime state
	plan               *Plan
	currentTask        *Task
	waitingForTask     bool
	taskIterationCount int // Counts completed tasks
}

// Option is a functional option for configuring Strategy
type Option func(*Strategy)
