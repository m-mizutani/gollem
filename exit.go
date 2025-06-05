package gollem

import "context"

// ExitTool is a tool that can be used to exit the session. IsCompleted() is called before calling a method to generate content every loop. If IsCompleted() returns true, the session will be ended.
type ExitTool interface {
	Tool
	IsCompleted() bool
	Response() string
}

// DefaultExitTool is the tool to stop the session loop. This tool is used when the agent determines that the session should be ended. The tool name is "respond_to_user".
type DefaultExitTool struct {
	isCompleted bool
	response    string
}

func (t *DefaultExitTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "respond_to_user",
		Description: "Call this tool when you have gathered all necessary information, completed all required actions, and already provided the final answer to the user's original request. This signals that your work on the current request is finished.",
		Parameters:  map[string]*Parameter{
			/*
				"final_answer": {
					Type:        "string",
					Description: "The comprehensive final answer or result for the user's request. If you already provided the final answer, you MUST omit this parameter.",
				},
			*/
		},
	}

}

func (t *DefaultExitTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	t.isCompleted = true

	if response, ok := args["final_answer"].(string); ok {
		t.response = response
	}

	return nil, nil
}

func (t *DefaultExitTool) IsCompleted() bool {
	return t.isCompleted
}

func (t *DefaultExitTool) Response() string {
	return t.response
}
