package gollem

import (
	"context"
)

// Strategy is a factory of actual execution strategy.
type Strategy func(client LLMClient) StrategyHandler

// StrategyHandler is a function that determines the next input for the LLM based on the current state.
// Returns ([]Input, *ExecuteResponse, error) where:
// - []Input: next input for LLM (nil means no LLM call)
// - *ExecuteResponse: strategy's conclusion (nil means let LLM decide)
// - error: execution error
type StrategyHandler func(ctx context.Context, state *StrategyState) ([]Input, *ExecuteResponse, error)

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
		return func(ctx context.Context, state *StrategyState) ([]Input, *ExecuteResponse, error) {
			// Initial iteration: send user input to LLM
			if state.Iteration == 0 {
				return state.InitInput, nil, nil
			}

			// Check LLM's last response
			if state.LastResponse != nil {
				if len(state.LastResponse.FunctionCalls) == 0 {
					// No tool calls = final response, use as conclusion
					executeResponse := &ExecuteResponse{
						Texts: state.LastResponse.Texts,
					}
					return nil, executeResponse, nil
				}
			}

			// Tool calls exist, continue with next input
			return state.NextInput, nil, nil
		}
	}
}
