package gemini_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/gemini"
	"github.com/m-mizutani/gt"
)

// Create mock tools for testing
type respondToUserTool struct{}

func (t *respondToUserTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "respond_to_user",
		Description: "Call this tool when you have gathered all necessary information, completed all required actions, and already provided the final answer to the user's original request. This signals that your work on the current request is finished.",
		Parameters: map[string]*gollem.Parameter{
			"summary": {
				Type:        gollem.TypeString,
				Description: "Brief summary of what was accomplished",
			},
		},
		// Note: Required is empty, making summary optional
	}
}

func (t *respondToUserTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return nil, nil
}

type parameterlessTool struct{}

func (t *parameterlessTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "no_params_tool",
		Description: "A tool with no parameters",
		Parameters:  map[string]*gollem.Parameter{}, // Empty parameters
		Required:    []string{},                     // Empty required
	}
}

func (t *parameterlessTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return nil, nil
}

// Test the exact respond_to_user tool structure that's causing issues
func TestRespondToUserTool(t *testing.T) {
	tool := &respondToUserTool{}
	converted := gemini.ConvertTool(tool)

	t.Logf("Converted tool: %+v", converted)
	t.Logf("Parameters: %+v", converted.Parameters)
	t.Logf("Properties: %+v", converted.Parameters.Properties)
	t.Logf("Required: %+v", converted.Parameters.Required)

	// Verify the structure
	gt.Value(t, converted.Name).Equal("respond_to_user")
	gt.Value(t, len(converted.Parameters.Properties)).Equal(1)
	
	// Critical finding: Required is nil, not empty slice!
	if converted.Parameters.Required == nil {
		t.Logf("CRITICAL: Required field is nil, not empty slice!")
	} else {
		t.Logf("Required field is empty slice: %v", converted.Parameters.Required)
	}
	
	summary := converted.Parameters.Properties["summary"]
	gt.Value(t, summary).NotEqual(nil)
	t.Logf("Summary type: %v (String representation: %s)", summary.Type, summary.Type.String())
}

// Test a completely parameter-less tool
func TestParameterlessTool(t *testing.T) {
	tool := &parameterlessTool{}
	converted := gemini.ConvertTool(tool)

	t.Logf("Parameterless tool: %+v", converted)
	t.Logf("Parameters: %+v", converted.Parameters)

	gt.Value(t, converted.Name).Equal("no_params_tool")
	gt.Value(t, len(converted.Parameters.Properties)).Equal(0)
	gt.Value(t, converted.Parameters.Required).Equal([]string{})
}