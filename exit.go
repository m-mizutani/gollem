package gollem

import "context"

// DefaultExitTool is the tool to stop the session loop. This tool is used when the agent determines that the session should be ended. The tool name is "finalize_task"

type DefaultExitTool struct {
	isCompleted bool
}

func (t *DefaultExitTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "finalize_task",
		Description: "This tool is used to stop the task loop. When you call this tool, the task loop will be stopped and the session will be ended.",
		Parameters: map[string]*Parameter{
			"conclusion": {
				Type:        "string",
				Description: "The conclusion of the task.",
			},
		},
	}
}

func (t *DefaultExitTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	t.isCompleted = true
	return nil, nil
}

func (t *DefaultExitTool) IsCompleted() bool {
	return t.isCompleted
}
