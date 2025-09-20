package gollem_test

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
)

// Sample tool implementation for testing
type randomNumberTool struct{}

func (t *randomNumberTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "random_number",
		Description: "A tool for generating random numbers within a specified range",
		Parameters: map[string]*gollem.Parameter{
			"min": {
				Type:        gollem.TypeNumber,
				Description: "Minimum value of the random number",
			},
			"max": {
				Type:        gollem.TypeNumber,
				Description: "Maximum value of the random number",
			},
		},
		Required: []string{"min", "max"},
	}
}

func (t *randomNumberTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	min, ok := args["min"].(float64)
	if !ok {
		return nil, goerr.New("min is required")
	}

	max, ok := args["max"].(float64)
	if !ok {
		return nil, goerr.New("max is required")
	}

	if min >= max {
		return nil, goerr.New("min must be less than max")
	}

	// Note: In real implementation, you would use a proper random number generator
	// This is just for testing purposes
	result := (min + max) / 2

	return map[string]any{"result": result}, nil
}
