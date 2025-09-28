package gollem

import (
	"context"
)

// Strategy defines the interface for execution strategies
type Strategy interface {
	// Init initializes the strategy with initial inputs
	// Called once when Execute is invoked, before the execution loop begins
	Init(ctx context.Context, inputs []Input) error

	// Handle determines the next input for the LLM based on the current state
	// Returns ([]Input, *ExecuteResponse, error) where:
	// - []Input: next input for LLM (nil means no LLM call)
	// - *ExecuteResponse: strategy's conclusion (nil means let LLM decide)
	// - error: execution error
	Handle(ctx context.Context, state *StrategyState) ([]Input, *ExecuteResponse, error)

	// Tools returns the tools that this strategy provides
	// This allows strategies to offer their own specialized tools
	Tools(ctx context.Context) ([]Tool, error)
}

// StrategyState contains the current state of the execution
type StrategyState struct {
	Session      Session   // Current LLM session
	InitInput    []Input   // Initial input (user input)
	LastResponse *Response // Last LLM response (nil on first call)
	NextInput    []Input   // Next input (same with InitInput in 1st iter, tool results in others)
	Iteration    int       // Current iteration count
}

// defaultStrategy implements the default simple loop strategy
type defaultStrategy struct{}

// newDefaultStrategy creates a new default strategy instance
func newDefaultStrategy() *defaultStrategy {
	return &defaultStrategy{}
}

func (s *defaultStrategy) Init(ctx context.Context, inputs []Input) error {
	// Default strategy needs no special initialization
	return nil
}

func (s *defaultStrategy) Handle(ctx context.Context, state *StrategyState) ([]Input, *ExecuteResponse, error) {
	// Initial iteration: send user input to LLM
	if state.Iteration == 0 {
		return state.InitInput, nil, nil
	}

	// Check LLM's last response
	if state.LastResponse != nil && len(state.LastResponse.FunctionCalls) == 0 {
		// No tool calls = final response, use as conclusion
		executeResponse := &ExecuteResponse{
			Texts: state.LastResponse.Texts,
		}
		return nil, executeResponse, nil
	}

	// Tool calls exist, continue with next input
	return state.NextInput, nil, nil
}

func (s *defaultStrategy) Tools(ctx context.Context) ([]Tool, error) {
	// Default strategy provides no additional tools
	return []Tool{}, nil
}
