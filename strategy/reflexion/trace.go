package reflexion

// TrialStartEvent is recorded when a trial begins.
type TrialStartEvent struct {
	TrialNumber int `json:"trial_number"`
}

// TrialEndEvent is recorded when a trial ends.
type TrialEndEvent struct {
	TrialNumber int    `json:"trial_number"`
	Success     bool   `json:"success"`
	Feedback    string `json:"feedback,omitempty"`
}

// ReflectionGeneratedEvent is recorded when a reflection is generated.
type ReflectionGeneratedEvent struct {
	TrialNumber int    `json:"trial_number"`
	Reflection  string `json:"reflection"`
}
