package react

// ThoughtEvent is recorded when the LLM generates a thought.
type ThoughtEvent struct {
	Iteration int    `json:"iteration"`
	Content   string `json:"content"`
}

// ActionEvent is recorded when an action is taken.
type ActionEvent struct {
	Iteration  int      `json:"iteration"`
	ActionType string   `json:"action_type"`
	ToolNames  []string `json:"tool_names,omitempty"`
}

// ObservationEvent is recorded when tool results are observed.
type ObservationEvent struct {
	Iteration int  `json:"iteration"`
	Success   bool `json:"success"`
}
