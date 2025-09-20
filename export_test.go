package gollem

import (
	"log/slog"
	"os"
)

var debugLogger *slog.Logger

func init() {
	debugLogger = slog.New(slog.DiscardHandler)
	if _, ok := os.LookupEnv("GOLLEM_DEBUG"); ok {
		debugLogger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		}))
	}
}

func DebugLogger() *slog.Logger { return debugLogger }

// Export buildCompactedHistory for testing
var BuildCompactedHistory = buildCompactedHistory

// Export message types for testing
type (
	ClaudeMessage      = claudeMessage
	GeminiMessage      = geminiMessage
	ClaudeContentBlock = claudeContentBlock
	ClaudeToolUse      = claudeToolUse
	GeminiPart         = geminiPart
)

// Export conversion functions for testing
var (
	ClaudeToTemplateMessages = claudeToTemplateMessages
	GeminiToTemplateMessages = geminiToTemplateMessages
)
