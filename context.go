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
