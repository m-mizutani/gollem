package gollem

import (
	"context"
	"log/slog"
)

type ctxLoggerKey struct{}

func ctxWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxLoggerKey{}, logger)
}

func LoggerFromContext(ctx context.Context) *slog.Logger {
	return ctx.Value(ctxLoggerKey{}).(*slog.Logger)
}
