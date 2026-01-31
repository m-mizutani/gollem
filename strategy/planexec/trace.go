package planexec

// PlanCreatedEvent is recorded when a plan is created.
type PlanCreatedEvent struct {
	Goal  string         `json:"goal"`
	Tasks []PlanTaskInfo `json:"tasks"`
}

// PlanTaskInfo represents a task in a trace event.
type PlanTaskInfo struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	State       string `json:"state"`
}

// TaskStartedEvent is recorded when a task execution begins.
type TaskStartedEvent struct {
	TaskID      string `json:"task_id"`
	Description string `json:"description"`
}

// TaskCompletedEvent is recorded when a task execution completes.
type TaskCompletedEvent struct {
	TaskID      string `json:"task_id"`
	Description string `json:"description"`
	State       string `json:"state"`
}

// PlanUpdatedEvent is recorded when a plan is updated after reflection.
type PlanUpdatedEvent struct {
	UpdatedTasks []PlanTaskInfo `json:"updated_tasks,omitempty"`
	NewTasks     []PlanTaskInfo `json:"new_tasks,omitempty"`
}

// AllTasksCompletedEvent is recorded when all tasks are completed.
type AllTasksCompletedEvent struct {
	TotalTasks int `json:"total_tasks"`
}
