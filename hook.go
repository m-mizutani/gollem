package gollem

import "context"

type (
	MessageHook      func(ctx context.Context, msg string) error
	ToolRequestHook  func(ctx context.Context, tool FunctionCall) error
	ToolResponseHook func(ctx context.Context, tool FunctionCall, response map[string]any) error
	ToolErrorHook    func(ctx context.Context, err error, tool FunctionCall) error
)

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
