package trace

// LLMCallData holds data specific to an LLM call span.
type LLMCallData struct {
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	Model        string `json:"model,omitempty"`

	// Prompt-cache token breakdown. Omitted when zero for backward compatibility.
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`

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

	// Thoughts is the thinking text returned by the model. It is the same
	// value as the Thoughts of the gollem.Response produced by the call; for a
	// streamed call, it holds the thinking text of all responses of the stream,
	// which some providers record joined instead of as one element per delta.
	// A thinking block whose text the provider omitted, as Claude does when
	// thinking display is "omitted", adds no element.
	Thoughts []string `json:"thoughts,omitempty"`

	// OtherBlockTypes is the types of the content blocks other than text and
	// tool_use, in the order received. It is the same value as the
	// OtherBlockTypes of the gollem.Response produced by the call; for a
	// streamed call, it holds the types of all responses of the stream. Only
	// Claude reports it. A thinking block whose text was omitted adds no
	// element to Thoughts but is listed here as "thinking", so a response that
	// holds only such a block can be told apart from a response without
	// content.
	OtherBlockTypes []string `json:"other_block_types,omitempty"`

	// FinishReason is the provider's reason for ending the generation, recorded
	// as returned by the provider. It is the same value as the FinishReason of
	// the gollem.Response produced by the call.
	FinishReason string `json:"finish_reason,omitempty"`

	// Refusal is the details of a refusal or a block, recorded as returned by
	// the provider. It holds the same values as the Refusal of the
	// gollem.Response produced by the call, and is nil when there is none.
	Refusal *Refusal `json:"refusal,omitempty"`
}

// Refusal is the details a provider reported for a refusal or a block. See
// gollem.Refusal for the source of each value.
type Refusal struct {
	Reason      string   `json:"reason,omitempty"`
	Categories  []string `json:"categories,omitempty"`
	Explanation string   `json:"explanation,omitempty"`
}

// Message represents a message in the trace with structured content blocks.
type Message struct {
	Role     string           `json:"role"`
	Contents []MessageContent `json:"contents"`
}

// MessageContent represents a single content block within a trace message.
type MessageContent struct {
	Type string `json:"type"`

	// Text content (type: "text")
	Text string `json:"text,omitempty"`

	// Tool call content (type: "tool_call")
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`

	// Tool response content (type: "tool_response")
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Result     map[string]any `json:"result,omitempty"`

	// Media content (type: "image", "pdf", "document", "file")
	MediaType string `json:"media_type,omitempty"`
	URL       string `json:"url,omitempty"`
	Title     string `json:"title,omitempty"`
}

// NewTextContent creates a text content block for trace messages.
func NewTextContent(text string) MessageContent {
	return MessageContent{Type: "text", Text: text}
}

// NewToolCallContent creates a tool call content block for trace messages.
func NewToolCallContent(id, name string, args map[string]any) MessageContent {
	return MessageContent{Type: "tool_call", ID: id, Name: name, Arguments: args}
}

// NewToolResponseContent creates a tool response content block for trace messages.
func NewToolResponseContent(toolCallID, name string, result map[string]any) MessageContent {
	return MessageContent{Type: "tool_response", ToolCallID: toolCallID, Name: name, Result: result}
}

// NewMediaContent creates a media content block (image, pdf, etc.) for trace messages.
func NewMediaContent(contentType, mediaType string) MessageContent {
	return MessageContent{Type: contentType, MediaType: mediaType}
}

// NewThinkingContent creates a reasoning content block for trace messages.
func NewThinkingContent(reasoning string) MessageContent {
	return MessageContent{Type: "reasoning", Text: reasoning}
}

// NewRedactedThinkingContent creates a redacted reasoning content block for trace messages.
func NewRedactedThinkingContent() MessageContent {
	return MessageContent{Type: "redacted_reasoning"}
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
