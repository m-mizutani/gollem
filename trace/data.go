package trace

// LLMCallData holds data specific to an LLM call span.
type LLMCallData struct {
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	Model        string `json:"model,omitempty"`

	Request  *LLMRequest  `json:"request"`
	Response *LLMResponse `json:"response"`
}

// LLMRequest represents the request sent to an LLM.
type LLMRequest struct {
	SystemPrompt string     `json:"system_prompt,omitempty"`
	Messages     []Message  `json:"messages"`
	Tools        []ToolSpec `json:"tools,omitempty"`
}

// LLMResponse represents the response from an LLM.
type LLMResponse struct {
	Texts         []string        `json:"texts,omitempty"`
	FunctionCalls []*FunctionCall `json:"function_calls,omitempty"`
}

// Message represents a message in the trace (simplified from gollem.Message).
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToolSpec represents a tool specification in the trace (simplified from gollem.ToolSpec).
type ToolSpec struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// FunctionCall represents a function call in the trace (simplified from gollem.FunctionCall).
type FunctionCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// ToolExecData holds data specific to a tool execution span.
type ToolExecData struct {
	ToolName string         `json:"tool_name"`
	Args     map[string]any `json:"args"`
	Result   map[string]any `json:"result,omitempty"`
	Error    string         `json:"error,omitempty"`
}

// EventData holds data specific to a strategy event span.
// Kind is a string defined by each Strategy implementation.
// Data is any JSON-serializable value defined by the Strategy.
type EventData struct {
	Kind string `json:"kind"`
	Data any    `json:"data"`
}
