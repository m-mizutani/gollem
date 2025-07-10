package internal

import (
	"io"
	"log/slog"
	"os"
)

var testLogger *slog.Logger

func init() {
	testLogger = slog.New(slog.NewJSONHandler(io.Discard, nil))
	if os.Getenv("GOLLEM_TEST_LOG") == "1" {
		testLogger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
	}
}

func TestLogger() *slog.Logger {
	return testLogger
}
