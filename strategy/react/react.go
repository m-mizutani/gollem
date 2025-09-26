package react

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/m-mizutani/gollem"
)

// New creates a ReAct (Reasoning + Acting) strategy
// This strategy encourages the LLM to think step-by-step before taking actions
func New(options ...Option) gollem.Strategy {
	return func(client gollem.LLMClient) gollem.StrategyHandler {
		impl := &reactImpl{
			llm:                 client,
			thoughtPrompt:       defaultThoughtPrompt,
			reflectionPrompt:    defaultReflectionPrompt,
			finishPrompt:        defaultFinishCheckPrompt,
			conversationEntries: []ConversationEntry{}, // Initialize structured conversation log
		}

		for _, opt := range options {
			opt(impl)
		}

		return impl.Handle
	}
}

const (
	defaultThoughtPrompt = `Let me approach this step-by-step:
1. First, I need to understand what's being asked
2. Then determine the best approach
3. Execute the necessary steps with reasoning

Thought: `

	defaultReflectionPrompt = `Based on the previous result, let me think about what to do next.

Observation: The tool returned %s
Thought: `

	defaultFinishCheckPrompt = `Analyze the conversation and determine task completion status. 
Respond with a JSON object with the following structure:
{
  "is_complete": boolean,
  "reason": "string explaining the decision",
  "next_action": "string describing what to do next (only if not complete)"
}`
)

type reactImpl struct {
	llm              gollem.LLMClient
	thoughtPrompt    string
	reflectionPrompt string
	finishPrompt     string

	// Internal state to track conversation with structured data
	conversationEntries []ConversationEntry // Store structured conversation history
}

func (x *reactImpl) Handle(ctx context.Context, state *gollem.StrategyState) ([]gollem.Input, *gollem.ExecuteResponse, error) {
	// First iteration: Add thought prompt
	if state.Iteration == 0 {
		x.recordInitialInput(state.InitInput)
		thought := gollem.Text(x.thoughtPrompt)
		return append([]gollem.Input{thought}, state.InitInput...), nil, nil
	}

	// Update conversation log with latest response and tool results
	x.updateConversationLog(state)

	// Process tool results with reflection
	if toolInput := x.processToolResults(state.NextInput); toolInput != nil {
		return toolInput, nil, nil
	}

	// ReAct core: Always evaluate next step when no tools are pending
	// This is the essence of ReAct - continuous reasoning about what to do next
	if len(state.NextInput) == 0 && state.LastResponse != nil {
		return x.evaluateNextStep(ctx, state)
	}

	// Continue with pending input
	return state.NextInput, nil, nil
}

func (x *reactImpl) recordInitialInput(inputs []gollem.Input) {
	for _, input := range inputs {
		entry := ConversationEntry{
			Type:    EntryTypeUser,
			Content: input.String(),
		}
		x.conversationEntries = append(x.conversationEntries, entry)
	}
}

func (x *reactImpl) updateConversationLog(state *gollem.StrategyState) {
	// Record last LLM response
	if state.LastResponse != nil {
		if len(state.LastResponse.Texts) > 0 {
			entry := ConversationEntry{
				Type:    EntryTypeAssistant,
				Content: strings.Join(state.LastResponse.Texts, " "),
			}
			x.conversationEntries = append(x.conversationEntries, entry)
		}

		// Record tool calls
		for _, fc := range state.LastResponse.FunctionCalls {
			entry := ConversationEntry{
				Type:     EntryTypeToolCall,
				Content:  fmt.Sprintf("Calling %s", fc.Name),
				ToolName: fc.Name,
			}
			x.conversationEntries = append(x.conversationEntries, entry)
		}
	}

	// Record tool results
	for _, input := range state.NextInput {
		if fr, ok := input.(gollem.FunctionResponse); ok {
			success := fr.Error == nil
			entry := ConversationEntry{
				Type:     EntryTypeToolResult,
				ToolName: fr.Name,
				Success:  &success,
			}

			if fr.Error != nil {
				entry.Content = fmt.Sprintf("Tool %s failed", fr.Name)
				entry.Error = fr.Error.Error()
			} else {
				entry.Content = fmt.Sprintf("Tool %s succeeded", fr.Name)
			}

			x.conversationEntries = append(x.conversationEntries, entry)
		}
	}
}

func (x *reactImpl) processToolResults(inputs []gollem.Input) []gollem.Input {
	var toolSummaries []string
	hasToolResponse := false

	for _, input := range inputs {
		if fr, ok := input.(gollem.FunctionResponse); ok {
			hasToolResponse = true
			if fr.Error != nil {
				toolSummaries = append(toolSummaries, fmt.Sprintf("error from %s", fr.Name))
			} else {
				toolSummaries = append(toolSummaries, fmt.Sprintf("result from %s", fr.Name))
			}
		}
	}

	if !hasToolResponse {
		return nil
	}

	// Create reflection prompt
	summary := toolSummaries[0]
	if len(toolSummaries) > 1 {
		summary = fmt.Sprintf("%d tool results", len(toolSummaries))
	}
	reflection := gollem.Text(fmt.Sprintf(x.reflectionPrompt, summary))
	return append([]gollem.Input{reflection}, inputs...)
}

func (x *reactImpl) evaluateNextStep(ctx context.Context, _ *gollem.StrategyState) ([]gollem.Input, *gollem.ExecuteResponse, error) {
	session, err := x.llm.NewSession(ctx,
		gollem.WithSessionSystemPrompt("You are a task completion analyzer. Analyze if a task is complete based on the conversation history and respond in JSON format."),
		gollem.WithSessionContentType(gollem.ContentTypeJSON))
	if err != nil {
		return nil, nil, err
	}

	contextPrompt := x.buildCompletionPrompt()
	response, err := session.GenerateContent(ctx, gollem.Text(contextPrompt))
	if err != nil {
		return nil, nil, err
	}

	return x.parseCompletionResponse(response)
}

func (x *reactImpl) buildCompletionPrompt() string {
	var prompt strings.Builder
	prompt.WriteString("Conversation history:\n")

	// Include recent conversation (last 5 entries)
	start := 0
	if len(x.conversationEntries) > 5 {
		start = len(x.conversationEntries) - 5
	}

	for i := start; i < len(x.conversationEntries); i++ {
		entry := x.conversationEntries[i]
		prompt.WriteString(fmt.Sprintf("%s: %s\n", entry.Type, entry.Content))
	}

	prompt.WriteString("\n")
	prompt.WriteString(x.finishPrompt)

	return prompt.String()
}

func (x *reactImpl) parseCompletionResponse(response *gollem.Response) ([]gollem.Input, *gollem.ExecuteResponse, error) {
	if len(response.Texts) == 0 {
		// No response, continue by default
		return []gollem.Input{gollem.Text("Continuing...")}, nil, nil
	}

	type CompletionCheck struct {
		IsComplete bool   `json:"is_complete"`
		Reason     string `json:"reason"`
		NextAction string `json:"next_action,omitempty"`
	}

	var result CompletionCheck
	if err := json.Unmarshal([]byte(response.Texts[0]), &result); err != nil {
		// JSON parsing failed - continue task without assumptions
		return []gollem.Input{gollem.Text("Continuing with task...")}, nil, nil
	}

	if result.IsComplete {
		// Record completion in structured format
		completionEntry := ConversationEntry{
			Type:    EntryTypeCompletion,
			Content: result.Reason,
		}
		x.conversationEntries = append(x.conversationEntries, completionEntry)

		// Generate conclusion based on reason and conversation
		conclusionText := fmt.Sprintf("Task completed: %s", result.Reason)
		executeResponse := &gollem.ExecuteResponse{
			Texts: []string{conclusionText},
		}
		return nil, executeResponse, nil
	}

	if result.NextAction != "" {
		// Record guidance in structured format
		guidanceEntry := ConversationEntry{
			Type:    EntryTypeGuidance,
			Content: result.NextAction,
		}
		x.conversationEntries = append(x.conversationEntries, guidanceEntry)
		return []gollem.Input{gollem.Text("Next: " + result.NextAction)}, nil, nil
	}

	return []gollem.Input{gollem.Text("Continuing...")}, nil, nil
}

// ConversationEntryType represents the type of conversation entry
type ConversationEntryType string

const (
	EntryTypeUser       ConversationEntryType = "USER"
	EntryTypeAssistant  ConversationEntryType = "ASSISTANT"
	EntryTypeToolCall   ConversationEntryType = "TOOL_CALL"
	EntryTypeToolResult ConversationEntryType = "TOOL_RESULT"
	EntryTypeCompletion ConversationEntryType = "COMPLETION"
	EntryTypeGuidance   ConversationEntryType = "GUIDANCE"
)

// ConversationEntry represents a structured conversation log entry
type ConversationEntry struct {
	Type     ConversationEntryType `json:"type"`
	Content  string                `json:"content"`
	ToolName string                `json:"tool_name,omitempty"`
	Success  *bool                 `json:"success,omitempty"`
	Error    string                `json:"error,omitempty"`
}


// Option is an option for configuring the ReAct strategy
type Option func(*reactImpl)

// WithThoughtPrompt sets a custom thought prompt for the ReAct strategy
func WithThoughtPrompt(prompt string) Option {
	return func(impl *reactImpl) {
		impl.thoughtPrompt = prompt
	}
}

// WithReflectionPrompt sets a custom reflection prompt for the ReAct strategy
// The prompt should contain one %s placeholder for the tool result summary
func WithReflectionPrompt(prompt string) Option {
	return func(impl *reactImpl) {
		impl.reflectionPrompt = prompt
	}
}

// WithFinishPrompt sets a custom prompt for checking task completion
func WithFinishPrompt(prompt string) Option {
	return func(impl *reactImpl) {
		impl.finishPrompt = prompt
	}
}
