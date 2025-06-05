package gollem

import "context"

type (
	// LoopHook is a hook for the session loop. "loop" is the loop count, it's 0-indexed. "input" is the current input of the loop. If you want to abort the session loop, you can return an error.
	LoopHook func(ctx context.Context, loop int, input []Input) error

	// MessageHook is a hook for the message. If you want to display or record the message, you can use this hook.
	MessageHook func(ctx context.Context, msg string) error

	// ToolRequestHook is a hook for the tool request. If you want to display or record the tool request, you can use this hook. If you want to abort the tool execution, you can return an error.
	ToolRequestHook func(ctx context.Context, tool FunctionCall) error

	// ToolResponseHook is a hook for the tool response. If you want to display or record the tool response, you can use this hook. If you want to abort the tool execution, you can return an error.
	ToolResponseHook func(ctx context.Context, tool FunctionCall, response map[string]any) error

	// ToolErrorHook is a hook for the tool error. If you want to record the tool error, you can use this hook.
	ToolErrorHook func(ctx context.Context, err error, tool FunctionCall) error
)

func defaultLoopHook(ctx context.Context, loop int, input []Input) error {
	return nil
}

func defaultMessageHook(ctx context.Context, msg string) error {
	return nil
}

func defaultToolRequestHook(ctx context.Context, tool FunctionCall) error {
	return nil
}

func defaultToolResponseHook(ctx context.Context, tool FunctionCall, response map[string]any) error {
	return nil
}

func defaultToolErrorHook(ctx context.Context, err error, tool FunctionCall) error {
	return nil
}
