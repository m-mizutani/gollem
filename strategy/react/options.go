package react

// Option is a function that configures the Strategy
type Option func(*Strategy)

// WithMaxIterations sets the maximum number of iterations
// Default is 20 if not specified
func WithMaxIterations(max int) Option {
	return func(s *Strategy) {
		s.maxIterations = max
	}
}

// WithMaxRepeatedActions sets the maximum number of times the same action can be repeated
// This helps detect infinite loops. Default is 3 if not specified
func WithMaxRepeatedActions(max int) Option {
	return func(s *Strategy) {
		s.maxRepeatedActions = max
	}
}

// WithSystemPrompt sets a custom system prompt
// If not set, DefaultSystemPrompt will be used
func WithSystemPrompt(prompt string) Option {
	return func(s *Strategy) {
		s.systemPrompt = prompt
	}
}

// WithThoughtPrompt sets a custom thought prompt
// If not set, DefaultThoughtPrompt will be used
func WithThoughtPrompt(prompt string) Option {
	return func(s *Strategy) {
		s.thoughtPrompt = prompt
	}
}

// WithObservationPrompt sets a custom observation prompt template
// The template should contain two %s placeholders for tool name and result
// Example: "Tool %s result: %s\nNext thought:"
func WithObservationPrompt(prompt string) Option {
	return func(s *Strategy) {
		s.observationPrompt = prompt
	}
}

// WithFewShotExamples enables few-shot learning with provided examples
func WithFewShotExamples(examples []FewShotExample) Option {
	return func(s *Strategy) {
		s.enableFewShot = true
		s.fewShotExamples = examples
	}
}
