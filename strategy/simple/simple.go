package simple

import (
	"context"

	"github.com/m-mizutani/gollem"
)

// New creates a simple loop strategy that continues as long as there is input
func New() gollem.Strategy {
	return func(client gollem.LLMClient) gollem.StrategyHandler {
		return func(ctx context.Context, state *gollem.StrategyState) ([]gollem.Input, *gollem.ExecuteResponse, error) {
			if state.Iteration == 0 {
				return state.InitInput, nil, nil
			}
			return state.NextInput, nil, nil
		}
	}
}
