package gollem

import (
	"context"
	"log/slog"
)

type ctxLoggerKey struct{}

var defaultLogger = slog.New(slog.DiscardHandler)

func ctxWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxLoggerKey{}, logger)
}

func LoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(ctxLoggerKey{}).(*slog.Logger); ok {
		return logger
	}
	return defaultLogger
}

type ctxPlanKey struct{}

func ctxWithPlan(ctx context.Context, plan *Plan) context.Context {
	return context.WithValue(ctx, ctxPlanKey{}, plan)
}

func PlanFromContext(ctx context.Context) (*Plan, bool) {
	plan, ok := ctx.Value(ctxPlanKey{}).(*Plan)
	return plan, ok
}

type ctxPlanToDoKey struct{}

func ctxWithPlanToDo(ctx context.Context, todo *planToDo) context.Context {
	expose := todo.toPlanToDo()
	return context.WithValue(ctx, ctxPlanToDoKey{}, &expose)
}

func PlanToDoFromContext(ctx context.Context) (*PlanToDo, bool) {
	todo, ok := ctx.Value(ctxPlanToDoKey{}).(*PlanToDo)
	return todo, ok
}
