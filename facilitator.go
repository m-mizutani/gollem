package gollem

import (
	"bytes"
	"context"
	"html/template"

	"github.com/m-mizutani/goerr/v2"
)

const (
	DefaultProceedPrompt   = "What is the next action needed to advance the task? If no further actions are required and you are ready to switch to the requested user, no need more message response and please use the `{{ .facilitator_tool_name }}` function to indicate completion immediately."
	DefaultFacilitatorName = "respond_to_user"
)

// Facilitator is a tool that can be used to control the session loop and provide proceed prompts.
// IsCompleted() is called before calling a method to generate content every loop. If IsCompleted() returns true, the session will be ended.
// ProceedPrompt() provides the prompt text that will be used when the LLM needs guidance on what to do next in the loop.
// This allows the Facilitator to control both when to exit the session and how to guide the LLM through the conversation flow.
type Facilitator interface {
	Tool
	IsCompleted() bool
	ProceedPrompt() string
}

// DefaultFacilitator is the tool to stop the session loop and provide proceed prompts.
// This tool is used when the agent determines that the session should be ended. The tool name is "respond_to_user".
// It provides a default proceed prompt that guides the LLM to continue working or use the facilitator tool when the task is completed.
type defaultFacilitator struct {
	prompt      string
	isCompleted bool
}

func newDefaultFacilitator() Facilitator {
	tmpl := template.Must(template.New("proceed_prompt").Parse(DefaultProceedPrompt))

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]any{
		"facilitator_tool_name": DefaultFacilitatorName,
	}); err != nil {
		panic(goerr.Wrap(err, "failed to execute proceed prompt template"))
	}

	return &defaultFacilitator{prompt: buf.String()}
}

var _ Facilitator = &defaultFacilitator{}

func (t *defaultFacilitator) Spec() ToolSpec {
	return ToolSpec{
		Name:        DefaultFacilitatorName,
		Description: "Call this tool when you have gathered all necessary information, completed all required actions, and already provided the final answer to the user's original request. This signals that your work on the current request is finished.",
	}
}

func (t *defaultFacilitator) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	t.isCompleted = true
	return map[string]any{
		"message": "The session is completed.",
	}, nil
}

func (t *defaultFacilitator) IsCompleted() bool {
	return t.isCompleted
}

func (t *defaultFacilitator) ProceedPrompt() string {
	return t.prompt
}
