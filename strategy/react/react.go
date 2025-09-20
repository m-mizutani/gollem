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
			llm:              client,
			thoughtPrompt:    defaultThoughtPrompt,
			reflectionPrompt: defaultReflectionPrompt,
			finishPrompt:     defaultFinishCheckPrompt,
			conversationLog:  []string{}, // Initialize conversation log
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

	// Internal state to track conversation
	conversationLog []string // Store conversation history for context
}

func (x *reactImpl) Handle(ctx context.Context, state *gollem.StrategyState) ([]gollem.Input, error) {
	// First iteration: Add thought prompt
	if state.Iteration == 0 {
		x.recordInitialInput(state.InitInput)
		thought := gollem.Text(x.thoughtPrompt)
		return append([]gollem.Input{thought}, state.InitInput...), nil
	}

	// Update conversation log with latest response and tool results
	x.updateConversationLog(state)

	// Process tool results with reflection
	if toolInput := x.processToolResults(state.NextInput); toolInput != nil {
		return toolInput, nil
	}

	// ReAct core: Always evaluate next step when no tools are pending
	// This is the essence of ReAct - continuous reasoning about what to do next
	if len(state.NextInput) == 0 && state.LastResponse != nil {
		return x.evaluateNextStep(ctx, state)
	}

	// Continue with pending input
	return state.NextInput, nil
}

func (x *reactImpl) recordInitialInput(inputs []gollem.Input) {
	for _, input := range inputs {
		x.conversationLog = append(x.conversationLog, "USER: "+input.String())
	}
}

func (x *reactImpl) updateConversationLog(state *gollem.StrategyState) {
	// Record last LLM response
	if state.LastResponse != nil {
		if len(state.LastResponse.Texts) > 0 {
			x.conversationLog = append(x.conversationLog, "ASSISTANT: "+strings.Join(state.LastResponse.Texts, " "))
		}
		for _, fc := range state.LastResponse.FunctionCalls {
			x.conversationLog = append(x.conversationLog, fmt.Sprintf("TOOL_CALL: %s", fc.Name))
		}
	}

	// Record tool results
	for _, input := range state.NextInput {
		if fr, ok := input.(gollem.FunctionResponse); ok {
			if fr.Error != nil {
				x.conversationLog = append(x.conversationLog, fmt.Sprintf("TOOL_RESULT: %s failed - %v", fr.Name, fr.Error))
			} else {
				x.conversationLog = append(x.conversationLog, fmt.Sprintf("TOOL_RESULT: %s succeeded", fr.Name))
			}
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

func (x *reactImpl) evaluateNextStep(ctx context.Context, state *gollem.StrategyState) ([]gollem.Input, error) {
	session, err := x.llm.NewSession(ctx,
		gollem.WithSessionSystemPrompt("You are a task completion analyzer. Analyze if a task is complete based on the conversation history and respond in JSON format."),
		gollem.WithSessionContentType(gollem.ContentTypeJSON))
	if err != nil {
		return nil, err
	}

	contextPrompt := x.buildCompletionPrompt()
	response, err := session.GenerateContent(ctx, gollem.Text(contextPrompt))
	if err != nil {
		return nil, err
	}

	return x.parseCompletionResponse(response)
}

func (x *reactImpl) buildCompletionPrompt() string {
	var prompt strings.Builder
	prompt.WriteString("Conversation history:\n")
	prompt.WriteString("================\n")

	// Include recent conversation (last 10 entries)
	start := 0
	if len(x.conversationLog) > 10 {
		start = len(x.conversationLog) - 10
	}

	for i := start; i < len(x.conversationLog); i++ {
		prompt.WriteString(x.conversationLog[i])
		prompt.WriteString("\n")
	}

	prompt.WriteString("================\n\n")
	prompt.WriteString(x.finishPrompt)

	return prompt.String()
}

func (x *reactImpl) parseCompletionResponse(response *gollem.Response) ([]gollem.Input, error) {
	if len(response.Texts) == 0 {
		// No response, continue by default
		return []gollem.Input{gollem.Text("Continuing...")}, nil
	}

	type CompletionCheck struct {
		IsComplete bool   `json:"is_complete"`
		Reason     string `json:"reason"`
		NextAction string `json:"next_action,omitempty"`
	}

	var result CompletionCheck
	if err := json.Unmarshal([]byte(response.Texts[0]), &result); err != nil {
		// Fallback to simple string matching if JSON parsing fails
		decision := strings.ToUpper(response.Texts[0])
		if strings.Contains(decision, "COMPLETE") || strings.Contains(decision, "TRUE") {
			return nil, nil
		}
		return []gollem.Input{gollem.Text("Continuing with task...")}, nil
	}

	if result.IsComplete {
		x.conversationLog = append(x.conversationLog, "COMPLETION: "+result.Reason)
		return nil, nil
	}

	if result.NextAction != "" {
		x.conversationLog = append(x.conversationLog, "GUIDANCE: "+result.NextAction)
		return []gollem.Input{gollem.Text("Next: " + result.NextAction)}, nil
	}

	return []gollem.Input{gollem.Text("Continuing...")}, nil
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
