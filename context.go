package gollem

import (
	"context"
	"log/slog"

	"github.com/m-mizutani/ctxlog"
)

// LoggerFromContext extracts logger from context.
// This function is kept for backward compatibility.
// New code should use ctxlog.From(ctx) directly.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	return ctxlog.From(ctx)
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
