package gollem

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"text/template"

	"github.com/m-mizutani/goerr/v2"
	"github.com/sashabaranov/go-openai"
)

// HistoryCompactor is a function type that handles compaction of conversation history
// It evaluates if compaction is needed and performs it if necessary.
// Returns the compacted history if compaction was performed, or the original history if no compaction was needed.
type HistoryCompactor func(ctx context.Context, history *History, llmClient LLMClient) (*History, error)

// DefaultCompactionSystemPrompt is the default system prompt used for conversation summarization
const DefaultCompactionSystemPrompt = `You are an expert at summarizing conversations. Your task is to create comprehensive summaries that will replace the original messages to reduce memory usage while maintaining conversation continuity.

SUMMARIZATION OBJECTIVES:
1. Preserve the context needed for the assistant to continue the conversation naturally
2. Maintain the logical flow and progression of topics discussed
3. Enable the assistant to understand what has been accomplished and what remains to be done

CRITICAL INFORMATION TO PRESERVE:
- User's main goals, requests, and preferences mentioned throughout the conversation
- Key decisions made and their rationale
- Important facts, data, or specific values discussed (names, numbers, dates, locations, etc.)
- Problems encountered and their solutions
- Current state of any ongoing tasks or processes
- All tool/function calls made and their results (these often contain critical data)
- Any commitments, promises, or action items agreed upon
- User's personal information or context shared (if any)
- Technical specifications, requirements, or constraints mentioned
- Error messages or issues that arose and how they were resolved

SUMMARIZATION GUIDELINES:
- Maintain chronological order of significant events
- Use clear, concise language while preserving technical accuracy
- Include enough detail that someone reading only the summary could understand the conversation's progress
- Pay special attention to the most recent topics as they're likely most relevant
- Preserve the tone and relationship dynamics established in the conversation

TARGET SUMMARY LENGTH:
- Aim to reduce the conversation to approximately 20-30% of its original length
- For very long conversations (50+ messages), aim for 10-20% of original length
- The summary should be substantial enough to maintain context but concise enough to significantly reduce memory usage
- Prioritize quality over strict length limits - it's better to be slightly longer if it preserves critical information

Please provide a summary that captures all essential information while achieving significant compaction.`

// DefaultCompactionPromptTemplate is the default prompt template used for conversation summarization.
// This template simply presents the conversation history to the LLM for summarization.
// The detailed instructions about how to summarize are provided in the DefaultCompactionSystemPrompt.
//
// This template uses Go's text/template syntax and receives a TemplateData struct with the following fields:
// - Messages: an array of message objects, each containing:
//   - Role: the role of the speaker (e.g., "user", "assistant", "system", "tool")
//   - Content: the message content
//   - ToolCalls: array of function/tool calls made by the assistant
//   - ToolResponses: array of tool execution results
//
// - MessageCount: the total number of messages
//
// Example usage in custom templates:
//
//	{{range .Messages}}
//	{{.Role}}: {{.Content}}
//	{{range .ToolCalls}}  [Called {{.Name}} with {{.Arguments}}]{{end}}
//	{{range .ToolResponses}}  [{{.Name}} returned: {{.Content}}]{{end}}
//	{{end}}
const DefaultCompactionPromptTemplate = `Here is the conversation history to summarize ({{.MessageCount}} messages):

{{range .Messages}}{{.Role}}: {{.Content}}{{if .ToolCalls}}
Tool calls:{{range .ToolCalls}}
  - {{.Name}}: {{.Arguments}}{{end}}{{end}}{{if .ToolResponses}}
Tool responses:{{range .ToolResponses}}
  - {{.Name}}: {{.Content}}{{end}}{{end}}
{{end}}`

// TemplateMessage represents a single message in the conversation history for template rendering
type TemplateMessage struct {
	Role    string
	Content string
	// ToolCalls contains function/tool calls made by the assistant
	ToolCalls []TemplateToolCall
	// ToolResponses contains tool execution results
	ToolResponses []TemplateToolResponse
}

// TemplateToolCall represents a tool/function call in the template
type TemplateToolCall struct {
	Name      string
	Arguments string
}

// TemplateToolResponse represents a tool execution result in the template
type TemplateToolResponse struct {
	Name    string
	Content string
}

// TemplateData contains the data passed to the prompt template
type TemplateData struct {
	// Messages is an array of conversation messages
	Messages []TemplateMessage
	// MessageCount is the total number of messages
	MessageCount int
}

// historyCompactionOptions contains internal configuration options for history compaction
type historyCompactionOptions struct {
	maxTokens            int // Maximum tokens before compaction is triggered
	preserveRecentTokens int // Number of recent tokens to preserve
	systemPrompt         string
	promptTemplate       string
}

// HistoryCompactionOption is a functional option for configuring history compaction
type HistoryCompactionOption func(*historyCompactionOptions)

// WithMaxTokens sets the maximum number of tokens allowed before compaction is triggered
// Default: 50000 (suitable for most use cases while leaving room for responses)
func WithMaxTokens(tokens int) HistoryCompactionOption {
	return func(opts *historyCompactionOptions) {
		opts.maxTokens = tokens
	}
}

// WithPreserveRecentTokens sets the number of recent tokens to preserve during compaction
// These tokens will never be compacted to maintain conversation context
// Default: 10000 (preserves substantial recent context)
func WithPreserveRecentTokens(tokens int) HistoryCompactionOption {
	return func(opts *historyCompactionOptions) {
		opts.preserveRecentTokens = tokens
	}
}

// WithCompactionSystemPrompt sets a custom system prompt for the summarization LLM
func WithCompactionSystemPrompt(prompt string) HistoryCompactionOption {
	return func(opts *historyCompactionOptions) {
		opts.systemPrompt = prompt
	}
}

// WithCompactionPromptTemplate sets a custom prompt template for summarization.
// The template should use Go's text/template syntax and will receive a TemplateData struct.
// See DefaultCompactionPromptTemplate for the available fields and example usage.
func WithCompactionPromptTemplate(tmpl string) HistoryCompactionOption {
	return func(opts *historyCompactionOptions) {
		opts.promptTemplate = tmpl
	}
}

// NewHistoryCompactor creates a history compactor that uses summarization to preserve context
// summarizerLLM: LLM client used for generating summaries of old messages while preserving context
// options: Functional options for configuring compaction behavior
func NewHistoryCompactor(summarizerLLM LLMClient, options ...HistoryCompactionOption) HistoryCompactor {
	// Apply default options
	opts := &historyCompactionOptions{
		maxTokens:            50000, // 50k tokens - reasonable default for modern LLMs
		preserveRecentTokens: 10000, // 10k tokens - preserves substantial recent context
		systemPrompt:         DefaultCompactionSystemPrompt,
		promptTemplate:       DefaultCompactionPromptTemplate,
	}

	// Apply provided options
	for _, opt := range options {
		opt(opts)
	}

	// Latest LLM context window sizes (as of 2024)
	contextLimits := map[llmType]int{
		llmTypeOpenAI: 128000,  // GPT-4 Turbo/GPT-4o
		llmTypeClaude: 200000,  // Claude 3.5 Sonnet / Claude 4
		llmTypeGemini: 1000000, // Gemini 2.0 Flash and newer
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

		// Check if compaction is needed using unified logic
		if !shouldCompact(ctx, history, llmClient, opts, contextLimits, targetTokens, emergencyTokens) {
			// No compaction needed, return the original history as-is
			return history, nil
		}

		// Compaction is needed, perform it using summarization
		return summarizeCompact(ctx, history, summarizerLLM, opts)
	}
}

// shouldCompact checks if history compaction is needed
func shouldCompact(ctx context.Context, history *History, llmClient LLMClient, options *historyCompactionOptions, contextLimits, targetTokens, emergencyTokens map[llmType]int) bool {
	if history == nil {
		return false
	}

	currentTokens := estimateTokens(ctx, history, llmClient)

	// 1. Check against configured max tokens
	if options.maxTokens > 0 {
		// Emergency compaction at 1.5x the limit
		if currentTokens >= int(float64(options.maxTokens)*1.5) {
			return true
		}
		// Normal compaction at the limit
		if currentTokens >= options.maxTokens {
			return true
		}
	}

	// 2. Emergency token count check based on LLM type
	emergencyThreshold, exists := emergencyTokens[history.LLType]
	if exists && currentTokens >= emergencyThreshold {
		return true
	}

	// 3. Near context limit check (95% threshold for emergency)
	if isNearContextLimitEmergency(ctx, history, llmClient, history.LLType, contextLimits) {
		return true
	}

	// 4. Normal token count check based on LLM type
	targetThreshold, exists := targetTokens[history.LLType]
	if exists && currentTokens >= targetThreshold {
		return true
	}

	// 5. Normal context limit proximity check (80% threshold)
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
	// Recommend compaction when reaching 80% of limit
	return estimatedTokens >= int(float64(limit)*0.8)
}

// isNearContextLimitEmergency checks if history is at emergency context window limits (95%)
func isNearContextLimitEmergency(ctx context.Context, history *History, llmClient LLMClient, llmType llmType, contextLimits map[llmType]int) bool {
	limit, exists := contextLimits[llmType]
	if !exists {
		limit = 8000 // Default limit
	}

	estimatedTokens := estimateTokens(ctx, history, llmClient)
	// Emergency compaction when reaching 95% of limit
	return estimatedTokens >= int(float64(limit)*0.95)
}

// summarizeCompact performs compaction by summarizing old messages
func summarizeCompact(ctx context.Context, history *History, llmClient LLMClient, options *historyCompactionOptions) (*History, error) {
	if history == nil {
		return nil, goerr.New("history is nil")
	}

	if llmClient == nil {
		return nil, goerr.New("LLM client is not set for summarization")
	}

	// Check if we have enough tokens to compact
	currentTokens := estimateTokens(ctx, history, llmClient)
	if currentTokens <= options.preserveRecentTokens {
		return history, nil
	}

	// Extract messages to be summarized
	oldHistory, recentHistory := extractMessages(ctx, history, options.preserveRecentTokens, llmClient)

	// Generate summary
	summary, err := generateSummary(ctx, llmClient, oldHistory, options.systemPrompt, options.promptTemplate)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to generate summary")
	}

	// Build new history with summary + recent messages
	compacted := buildCompactedHistory(history, summary, recentHistory)
	compacted.Compacted = true
	compacted.OriginalLen = history.ToCount()

	return compacted, nil
}

// extractMessages separates old messages from recent messages based on token count
func extractMessages(ctx context.Context, history *History, preserveRecentTokens int, llmClient LLMClient) (old, recent *History) {
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

	// Find the split point based on token count
	// We'll iterate from the end and accumulate tokens until we reach preserveRecentTokens
	switch history.LLType {
	case llmTypeOpenAI:
		msgs := history.OpenAI
		if len(msgs) == 0 {
			return nil, recentHistory
		}

		// Find split point by counting tokens from the end
		splitIdx := len(msgs)
		accumulatedTokens := 0

		for i := len(msgs) - 1; i >= 0; i-- {
			// Create temporary history with just this message to estimate its tokens
			tempHistory := &History{LLType: llmTypeOpenAI, OpenAI: []openai.ChatCompletionMessage{msgs[i]}}
			msgTokens := estimateTokens(ctx, tempHistory, llmClient)

			if accumulatedTokens+msgTokens > preserveRecentTokens && i < len(msgs)-1 {
				// We've exceeded the limit, use previous index as split
				splitIdx = i + 1
				break
			}
			accumulatedTokens += msgTokens
			splitIdx = i
		}

		if splitIdx == 0 {
			// All messages fit in recent history
			recentHistory.OpenAI = append([]openai.ChatCompletionMessage{}, msgs...)
			return nil, recentHistory
		}

		oldHistory.OpenAI = append([]openai.ChatCompletionMessage{}, msgs[:splitIdx]...)
		recentHistory.OpenAI = append([]openai.ChatCompletionMessage{}, msgs[splitIdx:]...)

	case llmTypeClaude:
		msgs := history.Claude
		if len(msgs) == 0 {
			return nil, recentHistory
		}

		// Find split point by counting tokens from the end
		splitIdx := len(msgs)
		accumulatedTokens := 0

		for i := len(msgs) - 1; i >= 0; i-- {
			// Create temporary history with just this message to estimate its tokens
			tempHistory := &History{LLType: llmTypeClaude, Claude: []claudeMessage{msgs[i]}}
			msgTokens := estimateTokens(ctx, tempHistory, llmClient)

			if accumulatedTokens+msgTokens > preserveRecentTokens && i < len(msgs)-1 {
				// We've exceeded the limit, use previous index as split
				splitIdx = i + 1
				break
			}
			accumulatedTokens += msgTokens
			splitIdx = i
		}

		if splitIdx == 0 {
			// All messages fit in recent history
			recentHistory.Claude = append([]claudeMessage{}, msgs...)
			return nil, recentHistory
		}

		oldHistory.Claude = append([]claudeMessage{}, msgs[:splitIdx]...)
		recentHistory.Claude = append([]claudeMessage{}, msgs[splitIdx:]...)

	case llmTypeGemini:
		msgs := history.Gemini
		if len(msgs) == 0 {
			return nil, recentHistory
		}

		// Find split point by counting tokens from the end
		splitIdx := len(msgs)
		accumulatedTokens := 0

		for i := len(msgs) - 1; i >= 0; i-- {
			// Create temporary history with just this message to estimate its tokens
			tempHistory := &History{LLType: llmTypeGemini, Gemini: []geminiMessage{msgs[i]}}
			msgTokens := estimateTokens(ctx, tempHistory, llmClient)

			if accumulatedTokens+msgTokens > preserveRecentTokens && i < len(msgs)-1 {
				// We've exceeded the limit, use previous index as split
				splitIdx = i + 1
				break
			}
			accumulatedTokens += msgTokens
			splitIdx = i
		}

		if splitIdx == 0 {
			// All messages fit in recent history
			recentHistory.Gemini = append([]geminiMessage{}, msgs...)
			return nil, recentHistory
		}

		oldHistory.Gemini = append([]geminiMessage{}, msgs[:splitIdx]...)
		recentHistory.Gemini = append([]geminiMessage{}, msgs[splitIdx:]...)
	}

	return oldHistory, recentHistory
}

// openAIToTemplateMessages converts OpenAI messages to TemplateMessage array
func openAIToTemplateMessages(msgs []openai.ChatCompletionMessage) []TemplateMessage {
	var result []TemplateMessage
	for _, msg := range msgs {
		if msg.Role != "system" { // Exclude system messages from summarization
			tmplMsg := TemplateMessage{
				Role:    msg.Role,
				Content: msg.Content,
			}

			// Handle tool calls from assistant
			if len(msg.ToolCalls) > 0 {
				for _, toolCall := range msg.ToolCalls {
					tmplMsg.ToolCalls = append(tmplMsg.ToolCalls, TemplateToolCall{
						Name:      toolCall.Function.Name,
						Arguments: toolCall.Function.Arguments,
					})
				}
			}

			// Handle tool responses (tool role)
			if msg.Role == "tool" && msg.ToolCallID != "" {
				// For tool responses, we'll add them as ToolResponses
				// Note: We might need to match this with the previous assistant message
				tmplMsg.ToolResponses = append(tmplMsg.ToolResponses, TemplateToolResponse{
					Name:    msg.Name,
					Content: msg.Content,
				})
			}

			result = append(result, tmplMsg)
		}
	}
	return result
}

// claudeToTemplateMessages converts Claude messages to TemplateMessage array
func claudeToTemplateMessages(msgs []claudeMessage) []TemplateMessage {
	var result []TemplateMessage
	for _, msg := range msgs {
		var content strings.Builder
		var toolCalls []TemplateToolCall
		var toolResponses []TemplateToolResponse

		for _, c := range msg.Content {
			if c.Text != nil {
				content.WriteString(*c.Text)
			}

			// Handle tool use (Claude's way of making tool calls)
			if c.ToolUse != nil {
				// Convert Input (interface{}) to JSON string
				argsJSON, _ := json.Marshal(c.ToolUse.Input)
				toolCalls = append(toolCalls, TemplateToolCall{
					Name:      c.ToolUse.Name,
					Arguments: string(argsJSON),
				})
			}

			// Handle tool results
			if c.ToolResult != nil {
				toolResponses = append(toolResponses, TemplateToolResponse{
					Name:    c.ToolResult.ToolUseID, // Using ID as name proxy
					Content: c.ToolResult.Content,
				})
			}
		}

		result = append(result, TemplateMessage{
			Role:          string(msg.Role),
			Content:       content.String(),
			ToolCalls:     toolCalls,
			ToolResponses: toolResponses,
		})
	}
	return result
}

// geminiToTemplateMessages converts Gemini messages to TemplateMessage array
func geminiToTemplateMessages(msgs []geminiMessage) []TemplateMessage {
	var result []TemplateMessage
	for _, msg := range msgs {
		var content strings.Builder
		var toolCalls []TemplateToolCall
		var toolResponses []TemplateToolResponse

		for _, part := range msg.Parts {
			if part.Text != "" {
				content.WriteString(part.Text)
			}

			// Handle function calls (when Type is "function_call")
			if part.Type == "function_call" && part.Name != "" {
				argsJSON, _ := json.Marshal(part.Args)
				toolCalls = append(toolCalls, TemplateToolCall{
					Name:      part.Name,
					Arguments: string(argsJSON),
				})
			}

			// Handle function responses (when Type is "function_response")
			if part.Type == "function_response" && part.Name != "" {
				respJSON, _ := json.Marshal(part.Response)
				toolResponses = append(toolResponses, TemplateToolResponse{
					Name:    part.Name,
					Content: string(respJSON),
				})
			}
		}

		result = append(result, TemplateMessage{
			Role:          msg.Role,
			Content:       content.String(),
			ToolCalls:     toolCalls,
			ToolResponses: toolResponses,
		})
	}
	return result
}

// generateSummary generates a summary from messages
func generateSummary(ctx context.Context, llmClient LLMClient, history *History, systemPrompt, promptTemplate string) (string, error) {
	if history == nil || history.ToCount() == 0 {
		return "", nil
	}

	// Convert history to template messages
	var messages []TemplateMessage
	switch history.LLType {
	case llmTypeOpenAI:
		messages = openAIToTemplateMessages(history.OpenAI)
	case llmTypeClaude:
		messages = claudeToTemplateMessages(history.Claude)
	case llmTypeGemini:
		messages = geminiToTemplateMessages(history.Gemini)
	}

	if len(messages) == 0 {
		return "", nil
	}

	// Create template data
	data := TemplateData{
		Messages:     messages,
		MessageCount: len(messages),
	}

	// Parse and execute the template
	tmpl, err := template.New("prompt").Parse(promptTemplate)
	if err != nil {
		return "", goerr.Wrap(err, "failed to parse prompt template")
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", goerr.Wrap(err, "failed to execute prompt template")
	}

	prompt := buf.String()

	// Create a simple session for summary generation
	session, err := llmClient.NewSession(ctx, WithSessionSystemPrompt(systemPrompt))
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

// buildCompactedHistory builds compacted history from summary and recent messages
func buildCompactedHistory(original *History, summary string, recentHistory *History) *History {
	compacted := &History{
		LLType:  original.LLType,
		Version: original.Version,
		Summary: summary,
	}

	if recentHistory == nil {
		return compacted
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
		compacted.OpenAI = msgs

	case llmTypeClaude:
		// For Claude, store in Summary field and keep only recent messages
		compacted.Claude = append([]claudeMessage{}, recentHistory.Claude...)

	case llmTypeGemini:
		// For Gemini, same approach
		compacted.Gemini = append([]geminiMessage{}, recentHistory.Gemini...)
	}

	return compacted
}
