package gollem

import (
	"context"
	"fmt"
	"strings"

	"github.com/m-mizutani/goerr/v2"
	"github.com/sashabaranov/go-openai"
)

// HistoryCompressor is a function type that handles compression of conversation history
// It evaluates if compression is needed and performs it if necessary.
// Returns the compressed history if compression was performed, or the original history if no compression was needed.
type HistoryCompressor func(ctx context.Context, history *History, llmClient LLMClient) (*History, error)

// HistoryCompressionOptions contains simple configuration options for history compression.
// These options control basic compression behavior.
type HistoryCompressionOptions struct {
	// MaxMessages is the maximum number of messages allowed before compression is triggered.
	// When the conversation history exceeds this number, compression will be performed.
	// Default: 50
	MaxMessages int

	// PreserveRecent is the number of recent messages to preserve during compression.
	// These messages will never be compressed to maintain conversation context.
	// Default: 10
	PreserveRecent int
}

// DefaultHistoryCompressionOptions returns default compression options with sensible defaults.
func DefaultHistoryCompressionOptions() HistoryCompressionOptions {
	return HistoryCompressionOptions{
		MaxMessages:    50, // Allow up to 50 messages before compression
		PreserveRecent: 10, // Always keep last 10 messages
	}
}

// DefaultHistoryCompressor creates a history compressor that uses summarization to preserve context.
// summarizerLLM: LLM client used for generating summaries of old messages while preserving context.
// options: Configuration options for controlling compression behavior (optional, uses defaults if not provided).
func DefaultHistoryCompressor(summarizerLLM LLMClient, options ...HistoryCompressionOptions) HistoryCompressor {
	// Use provided options or defaults
	opts := DefaultHistoryCompressionOptions()
	if len(options) > 0 {
		opts = options[0]
	}

	// Latest LLM context window sizes (as of 2024)
	contextLimits := map[llmType]int{
		llmTypeOpenAI: 128000,  // GPT-4 Turbo/GPT-4o
		llmTypeClaude: 200000,  // Claude 3.5 Sonnet / Claude 4
		llmTypeGemini: 1000000, // Gemini 2.0 Flash
	}

	// Per-LLM token thresholds based on context window size
	targetTokens := map[llmType]int{
		llmTypeOpenAI: 100000, // ~78% of 128k
		llmTypeClaude: 150000, // ~75% of 200k
		llmTypeGemini: 800000, // ~80% of 1M
	}

	emergencyTokens := map[llmType]int{
		llmTypeOpenAI: 120000, // ~94% of 128k
		llmTypeClaude: 190000, // ~95% of 200k
		llmTypeGemini: 950000, // ~95% of 1M
	}

	return func(ctx context.Context, history *History, llmClient LLMClient) (*History, error) {
		if history == nil {
			return nil, goerr.New("history is nil")
		}

		// Check if compression is needed using unified logic
		if !shouldCompress(ctx, history, llmClient, opts, contextLimits, targetTokens, emergencyTokens) {
			// No compression needed, return the original history as-is
			return history, nil
		}

		// Compression is needed, perform it using summarization
		return summarizeCompress(ctx, history, summarizerLLM, opts.PreserveRecent)
	}
}

// shouldCompress checks if history compression is needed (unified emergency + normal logic)
func shouldCompress(ctx context.Context, history *History, llmClient LLMClient, options HistoryCompressionOptions, contextLimits, targetTokens, emergencyTokens map[llmType]int) bool {
	if history == nil {
		return false
	}

	// Emergency compression checks (prioritized)
	// 1. Emergency message count check (1.5x normal limit)
	emergencyMessageLimit := int(float64(options.MaxMessages) * 1.5)
	if history.ToCount() >= emergencyMessageLimit {
		return true
	}

	// 2. Emergency token count check
	emergencyThreshold, exists := emergencyTokens[history.LLType]
	if exists {
		if estimateTokens(ctx, history, llmClient) >= emergencyThreshold {
			return true
		}
	}

	// 3. Near context limit check (95% threshold for emergency)
	if isNearContextLimitEmergency(ctx, history, llmClient, history.LLType, contextLimits) {
		return true
	}

	// Normal compression checks
	// 4. Normal message count check
	if history.ToCount() >= options.MaxMessages {
		return true
	}

	// 5. Normal token count check
	targetThreshold, exists := targetTokens[history.LLType]
	if exists {
		if estimateTokens(ctx, history, llmClient) >= targetThreshold {
			return true
		}
	}

	// 6. Normal context limit proximity check (80% threshold)
	if isNearContextLimit(ctx, history, llmClient, history.LLType, contextLimits) {
		return true
	}

	return false
}

// estimateTokens estimates the number of tokens in the history using LLMClient
func estimateTokens(ctx context.Context, history *History, llmClient LLMClient) int {
	if history == nil {
		return 0
	}

	// Use LLMClient's CountTokens method for accurate counting if client is available
	if llmClient != nil {
		tokens, err := llmClient.CountTokens(ctx, history)
		if err == nil {
			return tokens
		}
		// If CountTokens fails, fall back to character estimation
	}

	// Fallback to simple character-based estimation if CountTokens fails or client is nil
	return fallbackEstimateTokens(history)
}

// fallbackEstimateTokens provides a fallback token estimation method
func fallbackEstimateTokens(history *History) int {
	if history == nil {
		return 0
	}

	totalChars := 0

	switch history.LLType {
	case llmTypeOpenAI:
		for _, msg := range history.OpenAI {
			totalChars += len(msg.Role) + len(msg.Content)
			// Include tool calls in estimation
			if msg.ToolCalls != nil {
				for _, call := range msg.ToolCalls {
					totalChars += len(call.Function.Name) + len(call.Function.Arguments)
				}
			}
		}

	case llmTypeClaude:
		for _, msg := range history.Claude {
			totalChars += len(string(msg.Role))
			for _, content := range msg.Content {
				if content.Text != nil {
					totalChars += len(*content.Text)
				}
			}
		}

	case llmTypeGemini:
		for _, msg := range history.Gemini {
			totalChars += len(msg.Role)
			for _, part := range msg.Parts {
				totalChars += len(part.Text)
			}
		}
	}

	// Rough estimation: 4 characters = 1 token (typical ratio for English)
	return totalChars / 4
}

// isNearContextLimit checks if history is approaching context window limits (80% threshold)
func isNearContextLimit(ctx context.Context, history *History, llmClient LLMClient, llmType llmType, contextLimits map[llmType]int) bool {
	limit, exists := contextLimits[llmType]
	if !exists {
		limit = 8000 // Default limit
	}

	estimatedTokens := estimateTokens(ctx, history, llmClient)
	// Recommend compression when reaching 80% of limit
	return estimatedTokens >= int(float64(limit)*0.8)
}

// isNearContextLimitEmergency checks if history is at emergency context window limits (95%)
func isNearContextLimitEmergency(ctx context.Context, history *History, llmClient LLMClient, llmType llmType, contextLimits map[llmType]int) bool {
	limit, exists := contextLimits[llmType]
	if !exists {
		limit = 8000 // Default limit
	}

	estimatedTokens := estimateTokens(ctx, history, llmClient)
	// Emergency compression when reaching 95% of limit
	return estimatedTokens >= int(float64(limit)*0.95)
}

// summarizeCompress performs compression by summarizing old messages
func summarizeCompress(ctx context.Context, history *History, llmClient LLMClient, preserveRecent int) (*History, error) {
	if history == nil {
		return nil, goerr.New("history is nil")
	}

	if llmClient == nil {
		return nil, goerr.New("LLM client is not set for summarization")
	}

	totalCount := history.ToCount()
	if totalCount <= preserveRecent {
		return history, nil
	}

	// Extract messages to be summarized
	oldHistory, recentHistory := extractMessages(history, preserveRecent)

	// Generate summary
	summary, err := generateSummary(ctx, llmClient, oldHistory)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to generate summary")
	}

	// Build new history with summary + recent messages
	compressed := buildCompressedHistory(history, summary, recentHistory)
	compressed.Compressed = true
	compressed.OriginalLen = totalCount

	return compressed, nil
}

// extractMessages separates old messages from recent messages
func extractMessages(history *History, preserveRecent int) (old, recent *History) {
	if history == nil {
		return nil, nil
	}

	// Create base History objects
	oldHistory := &History{
		LLType:  history.LLType,
		Version: history.Version,
	}
	recentHistory := &History{
		LLType:  history.LLType,
		Version: history.Version,
	}

	switch history.LLType {
	case llmTypeOpenAI:
		totalMsgs := len(history.OpenAI)
		oldCount := totalMsgs - preserveRecent
		if oldCount <= 0 {
			recentHistory.OpenAI = append([]openai.ChatCompletionMessage{}, history.OpenAI...)
			return nil, recentHistory
		}

		oldHistory.OpenAI = append([]openai.ChatCompletionMessage{}, history.OpenAI[:oldCount]...)
		recentHistory.OpenAI = append([]openai.ChatCompletionMessage{}, history.OpenAI[oldCount:]...)

	case llmTypeClaude:
		totalMsgs := len(history.Claude)
		oldCount := totalMsgs - preserveRecent
		if oldCount <= 0 {
			recentHistory.Claude = append([]claudeMessage{}, history.Claude...)
			return nil, recentHistory
		}

		oldHistory.Claude = append([]claudeMessage{}, history.Claude[:oldCount]...)
		recentHistory.Claude = append([]claudeMessage{}, history.Claude[oldCount:]...)

	case llmTypeGemini:
		totalMsgs := len(history.Gemini)
		oldCount := totalMsgs - preserveRecent
		if oldCount <= 0 {
			recentHistory.Gemini = append([]geminiMessage{}, history.Gemini...)
			return nil, recentHistory
		}

		oldHistory.Gemini = append([]geminiMessage{}, history.Gemini[:oldCount]...)
		recentHistory.Gemini = append([]geminiMessage{}, history.Gemini[oldCount:]...)
	}

	return oldHistory, recentHistory
}

// openAIToStrings converts OpenAI messages to string array
func openAIToStrings(msgs []openai.ChatCompletionMessage) []string {
	var result []string
	for _, msg := range msgs {
		if msg.Role != "system" { // Exclude system messages from summarization
			result = append(result, fmt.Sprintf("%s: %s", msg.Role, msg.Content))
		}
	}
	return result
}

// claudeToStrings converts Claude messages to string array
func claudeToStrings(msgs []claudeMessage) []string {
	var result []string
	for _, msg := range msgs {
		var content strings.Builder
		for _, c := range msg.Content {
			if c.Text != nil {
				content.WriteString(*c.Text)
			}
		}
		result = append(result, fmt.Sprintf("%s: %s", msg.Role, content.String()))
	}
	return result
}

// geminiToStrings converts Gemini messages to string array
func geminiToStrings(msgs []geminiMessage) []string {
	var result []string
	for _, msg := range msgs {
		var content strings.Builder
		for _, part := range msg.Parts {
			content.WriteString(part.Text)
		}
		result = append(result, fmt.Sprintf("%s: %s", msg.Role, content.String()))
	}
	return result
}

// generateSummary generates a summary from messages
func generateSummary(ctx context.Context, llmClient LLMClient, history *History) (string, error) {
	if history == nil || history.ToCount() == 0 {
		return "", nil
	}

	// Convert history to string format for summarization
	var messages []string
	switch history.LLType {
	case llmTypeOpenAI:
		messages = openAIToStrings(history.OpenAI)
	case llmTypeClaude:
		messages = claudeToStrings(history.Claude)
	case llmTypeGemini:
		messages = geminiToStrings(history.Gemini)
	}

	if len(messages) == 0 {
		return "", nil
	}

	conversationText := strings.Join(messages, "\n")

	prompt := fmt.Sprintf(`Please summarize the following conversation history concisely. Preserve important points and context, and always include specific facts and decisions made.

Conversation History:
%s

Summary:`, conversationText)

	// Create a simple session for summary generation
	session, err := llmClient.NewSession(ctx,
		WithSessionSystemPrompt("You are an expert at summarizing conversations. Please summarize concisely without losing important information."))
	if err != nil {
		return "", goerr.Wrap(err, "failed to create summarization session")
	}

	response, err := session.GenerateContent(ctx, Text(prompt))
	if err != nil {
		return "", goerr.Wrap(err, "failed to generate summary")
	}

	if len(response.Texts) == 0 {
		return "", goerr.New("no summary text generated")
	}

	return response.Texts[0], nil
}

// buildCompressedHistory builds compressed history from summary and recent messages
func buildCompressedHistory(original *History, summary string, recentHistory *History) *History {
	compressed := &History{
		LLType:  original.LLType,
		Version: original.Version,
		Summary: summary,
	}

	if recentHistory == nil {
		return compressed
	}

	switch original.LLType {
	case llmTypeOpenAI:
		// Add summary as a system message
		msgs := []openai.ChatCompletionMessage{
			{
				Role:    "system",
				Content: "Conversation history summary: " + summary,
			},
		}

		// Preserve original system messages
		for _, msg := range original.OpenAI {
			if msg.Role == "system" {
				msgs = append(msgs, msg)
			}
		}

		// Add recent messages directly (no string conversion needed)
		msgs = append(msgs, recentHistory.OpenAI...)
		compressed.OpenAI = msgs

	case llmTypeClaude:
		// For Claude, store in Summary field and keep only recent messages
		compressed.Claude = append([]claudeMessage{}, recentHistory.Claude...)

	case llmTypeGemini:
		// For Gemini, same approach
		compressed.Gemini = append([]geminiMessage{}, recentHistory.Gemini...)
	}

	return compressed
}
