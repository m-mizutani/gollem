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
	Tools        []Tool    // Available tools for this execution

	// System prompt and history from Agent configuration
	// These are available for strategies that need context-aware planning
	SystemPrompt string   // User's system prompt from gollem.WithSystemPrompt
	History      *History // Conversation history from gollem.WithHistory
}

// defaultStrategy implements the default simple loop strategy
// Note: This implementation should be kept in sync with strategy/simple/simple.go
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
	if state.Iteration == 0 {
		return state.InitInput, nil, nil
	}

	// Check LLM's last response for termination condition
	if state.LastResponse != nil {
		if len(state.LastResponse.FunctionCalls) == 0 {
			// No tool calls = final response, use as conclusion
			executeResponse := &ExecuteResponse{
				Texts: state.LastResponse.Texts,
			}
			return nil, executeResponse, nil
		}
	}

	return state.NextInput, nil, nil
}

func (s *defaultStrategy) Tools(ctx context.Context) ([]Tool, error) {
	// Default strategy provides no additional tools
	return []Tool{}, nil
}
