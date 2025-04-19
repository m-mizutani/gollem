package servantic

import "context"

type (
	MsgCallback  func(ctx context.Context, msg string) error
	ToolCallback func(ctx context.Context, tool FunctionCall) error
	ErrCallback  func(ctx context.Context, err error, tool FunctionCall) error
)

func defaultMsgCallback(ctx context.Context, msg string) error {
	return nil
}

func defaultToolCallback(ctx context.Context, tool FunctionCall) error {
	return nil
}

func defaultErrCallback(ctx context.Context, err error, tool FunctionCall) error {
	return nil
}
