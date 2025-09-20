package gollem

import (
	"context"
)

// Strategy is a factory of actual execution strategy.
type Strategy func(client LLMClient) StrategyHandler

// StrategyHandler is a function that determines the next input for the LLM based on the current state.
// Returning nil indicates that the task is complete.
type StrategyHandler func(ctx context.Context, state *StrategyState) ([]Input, error)

// StrategyState contains the current state of the execution
type StrategyState struct {
	Session      Session   // Current LLM session
	InitInput    []Input   // Initial input (user input)
	LastResponse *Response // Last LLM response (nil on first call)
	NextInput    []Input   // Next input (same with InitInput in 1st iter, tool results in others)
	Iteration    int       // Current iteration count
}

// defaultStrategy returns the default simple loop strategy
func defaultStrategy() Strategy {
	return func(client LLMClient) StrategyHandler {
		return func(ctx context.Context, state *StrategyState) ([]Input, error) {
			if state.Iteration == 0 {
				return state.InitInput, nil
			}
			return state.NextInput, nil
		}
	}
}
