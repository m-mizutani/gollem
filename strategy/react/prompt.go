package react

import (
	"fmt"
	"strings"
)

// Default prompt templates based on ReAct paper
const (
	// DefaultSystemPrompt is the default system prompt for ReAct strategy
	DefaultSystemPrompt = `Solve tasks using your available tools through the ReAct (Reasoning + Acting) pattern.

When you respond:
1. First, think step by step about what information you need
2. Then, call the appropriate function/tool to get that information (do not write "Action:" as text - use the actual function calling mechanism)
3. After receiving results, continue reasoning and calling more functions as needed
4. Only when you have gathered all necessary information, provide your final answer

CRITICAL RULES:
- You MUST use the function calling feature to execute tools. Do not simulate tool calls in your text response.
- You MUST gather information through actual function calls before providing your answer.
- NEVER guess or assume information - always use tools to discover it.
- If a task requires exploring or finding information, you MUST use the available tools to explore and gather data.
- Do NOT provide a final answer until you have used tools to collect all necessary information.`

	// DefaultThoughtPrompt is the default prompt to encourage reasoning
	DefaultThoughtPrompt = `Thought:`

	// DefaultObservationPromptTemplate is the template for observation prompts
	// Use with fmt.Sprintf(DefaultObservationPromptTemplate, toolName, result)
	DefaultObservationPromptTemplate = `Observation: %s returned:
%s

Thought:`
)

// buildThoughtPrompt returns the thought prompt
func (s *Strategy) buildThoughtPrompt() string {
	if s.thoughtPrompt != "" {
		return s.thoughtPrompt
	}
	return DefaultThoughtPrompt
}

// buildObservationPrompt constructs observation prompt from tool results
func (s *Strategy) buildObservationPrompt(toolResults []ToolResult) string {
	if len(toolResults) == 0 {
		return ""
	}

	// Single tool result
	if len(toolResults) == 1 {
		result := toolResults[0]
		resultText := result.Output
		if !result.Success {
			resultText = fmt.Sprintf("Error: %s", result.Error)
		}

		if s.observationPrompt != "" {
			return fmt.Sprintf(s.observationPrompt, result.ToolName, resultText)
		}
		return fmt.Sprintf(DefaultObservationPromptTemplate, result.ToolName, resultText)
	}

	// Multiple tool results
	var sb strings.Builder
	sb.WriteString("Observation:\n")
	for _, result := range toolResults {
		if result.Success {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", result.ToolName, result.Output))
		} else {
			sb.WriteString(fmt.Sprintf("- %s: Error: %s\n", result.ToolName, result.Error))
		}
	}
	sb.WriteString("\nThought:")

	return sb.String()
}
