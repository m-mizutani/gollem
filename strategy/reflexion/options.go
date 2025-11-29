package reflexion

// Option is a function that configures a Strategy.
type Option func(*Strategy)

// WithMaxTrials sets the maximum number of trials.
// Default is 3.
func WithMaxTrials(n int) Option {
	return func(s *Strategy) {
		s.maxTrials = n
	}
}

// WithMemorySize sets the maximum size of episodic memory.
// Default is 3.
func WithMemorySize(n int) Option {
	return func(s *Strategy) {
		s.memorySize = n
	}
}

// WithEvaluator sets a custom evaluator.
// If not set, LLMEvaluator is used by default.
func WithEvaluator(evaluator Evaluator) Option {
	return func(s *Strategy) {
		s.evaluator = evaluator
	}
}

// WithHooks sets lifecycle hooks.
func WithHooks(hooks Hooks) Option {
	return func(s *Strategy) {
		s.hooks = hooks
	}
}
