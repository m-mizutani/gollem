package gollem

import (
	"encoding/json"
	"time"
)

// Message represents a unified message format that can be converted between different LLM providers.
// All provider-specific messages are converted to this common format for cross-provider compatibility.
type Message struct {
	Role     MessageRole      `json:"role"`
	Contents []MessageContent `json:"contents"`

	// Optional fields for provider-specific information
	Name     string                 `json:"name,omitempty"`     // OpenAI's name field
	Metadata map[string]interface{} `json:"metadata,omitempty"` // Extension metadata
}

// MessageRole represents the role of a message in a conversation
type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"     // OpenAI tool response
	RoleFunction  MessageRole = "function" // OpenAI function response (legacy)
	RoleModel     MessageRole = "model"    // Gemini model role (maps to assistant)
)

// MessageContent represents the content of a message in a unified format
type MessageContent struct {
	Type MessageContentType `json:"type"`
	// Data contains type-specific content that should be unmarshaled based on Type
	Data json.RawMessage `json:"data"`
}

// MessageContentType represents the type of content in a message
type MessageContentType string

const (
	MessageContentTypeText             MessageContentType = "text"
	MessageContentTypeImage            MessageContentType = "image"
	MessageContentTypeToolCall         MessageContentType = "tool_call"
	MessageContentTypeToolResponse     MessageContentType = "tool_response"
	MessageContentTypeFunctionCall     MessageContentType = "function_call"     // OpenAI legacy
	MessageContentTypeFunctionResponse MessageContentType = "function_response" // OpenAI legacy
)

// TextContent represents text content in a message
type TextContent struct {
	Text string `json:"text"`
}

// ImageContent represents image content in a message
type ImageContent struct {
	MediaType string `json:"media_type,omitempty"` // e.g., "image/jpeg", "image/png"
	Data      []byte `json:"data,omitempty"`       // Image data (base64 encoded in JSON)
	URL       string `json:"url,omitempty"`        // Image URL (either Data or URL should be set)
	Detail    string `json:"detail,omitempty"`     // OpenAI: "high", "low", "auto"
}

// ToolCallContent represents a tool/function call request
type ToolCallContent struct {
	ID        string                 `json:"id"`        // Call ID for matching with response
	Name      string                 `json:"name"`      // Tool/function name
	Arguments map[string]interface{} `json:"arguments"` // Arguments as JSON object
}

// ToolResponseContent represents a tool/function response
type ToolResponseContent struct {
	ToolCallID string                 `json:"tool_call_id"`       // ID of the corresponding call
	Name       string                 `json:"name,omitempty"`     // Tool/function name (required for Gemini)
	Response   map[string]interface{} `json:"response"`           // Response content
	IsError    bool                   `json:"is_error,omitempty"` // Whether this is an error response (Claude)
}

// FunctionCallContent represents a legacy OpenAI function call
type FunctionCallContent struct {
	Name      string `json:"name"`      // Function name
	Arguments string `json:"arguments"` // Arguments as JSON string
}

// FunctionResponseContent represents a legacy OpenAI function response
type FunctionResponseContent struct {
	Name    string `json:"name"`    // Function name
	Content string `json:"content"` // Response content as string
}

// HistoryMetadata contains metadata about the conversation history
type HistoryMetadata struct {
	OriginalProvider LLMType   `json:"original_provider,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	// Compaction related fields (preserved from existing functionality)
	Compacted   bool   `json:"compacted,omitempty"`
	Summary     string `json:"summary,omitempty"`
	OriginalLen int    `json:"original_len,omitempty"`
}

// Helper methods for creating MessageContent

// NewTextContent creates a new text message content
func NewTextContent(text string) (MessageContent, error) {
	data, err := json.Marshal(TextContent{Text: text})
	if err != nil {
		return MessageContent{}, err
	}
	return MessageContent{
		Type: MessageContentTypeText,
		Data: data,
	}, nil
}

// NewImageContent creates a new image message content
func NewImageContent(mediaType string, imageData []byte, url string, detail string) (MessageContent, error) {
	data, err := json.Marshal(ImageContent{
		MediaType: mediaType,
		Data:      imageData,
		URL:       url,
		Detail:    detail,
	})
	if err != nil {
		return MessageContent{}, err
	}
	return MessageContent{
		Type: MessageContentTypeImage,
		Data: data,
	}, nil
}

// NewToolCallContent creates a new tool call message content
func NewToolCallContent(id, name string, args map[string]interface{}) (MessageContent, error) {
	data, err := json.Marshal(ToolCallContent{
		ID:        id,
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return MessageContent{}, err
	}
	return MessageContent{
		Type: MessageContentTypeToolCall,
		Data: data,
	}, nil
}

// NewToolResponseContent creates a new tool response message content
func NewToolResponseContent(toolCallID, name string, response map[string]interface{}, isError bool) (MessageContent, error) {
	data, err := json.Marshal(ToolResponseContent{
		ToolCallID: toolCallID,
		Name:       name,
		Response:   response,
		IsError:    isError,
	})
	if err != nil {
		return MessageContent{}, err
	}
	return MessageContent{
		Type: MessageContentTypeToolResponse,
		Data: data,
	}, nil
}

// NewFunctionCallContent creates a new function call message content (legacy OpenAI)
func NewFunctionCallContent(name, arguments string) (MessageContent, error) {
	data, err := json.Marshal(FunctionCallContent{
		Name:      name,
		Arguments: arguments,
	})
	if err != nil {
		return MessageContent{}, err
	}
	return MessageContent{
		Type: MessageContentTypeFunctionCall,
		Data: data,
	}, nil
}

// NewFunctionResponseContent creates a new function response message content (legacy OpenAI)
func NewFunctionResponseContent(name, content string) (MessageContent, error) {
	data, err := json.Marshal(FunctionResponseContent{
		Name:    name,
		Content: content,
	})
	if err != nil {
		return MessageContent{}, err
	}
	return MessageContent{
		Type: MessageContentTypeFunctionResponse,
		Data: data,
	}, nil
}

// Helper methods for extracting content from MessageContent

// GetTextContent extracts text content from a MessageContent
func (mc *MessageContent) GetTextContent() (*TextContent, error) {
	if mc.Type != MessageContentTypeText {
		return nil, ErrInvalidHistoryData
	}
	var content TextContent
	if err := json.Unmarshal(mc.Data, &content); err != nil {
		return nil, err
	}
	return &content, nil
}

// GetImageContent extracts image content from a MessageContent
func (mc *MessageContent) GetImageContent() (*ImageContent, error) {
	if mc.Type != MessageContentTypeImage {
		return nil, ErrInvalidHistoryData
	}
	var content ImageContent
	if err := json.Unmarshal(mc.Data, &content); err != nil {
		return nil, err
	}
	return &content, nil
}

// GetToolCallContent extracts tool call content from a MessageContent
func (mc *MessageContent) GetToolCallContent() (*ToolCallContent, error) {
	if mc.Type != MessageContentTypeToolCall {
		return nil, ErrInvalidHistoryData
	}
	var content ToolCallContent
	if err := json.Unmarshal(mc.Data, &content); err != nil {
		return nil, err
	}
	return &content, nil
}

// GetToolResponseContent extracts tool response content from a MessageContent
func (mc *MessageContent) GetToolResponseContent() (*ToolResponseContent, error) {
	if mc.Type != MessageContentTypeToolResponse {
		return nil, ErrInvalidHistoryData
	}
	var content ToolResponseContent
	if err := json.Unmarshal(mc.Data, &content); err != nil {
		return nil, err
	}
	return &content, nil
}

// GetFunctionCallContent extracts function call content from a MessageContent
func (mc *MessageContent) GetFunctionCallContent() (*FunctionCallContent, error) {
	if mc.Type != MessageContentTypeFunctionCall {
		return nil, ErrInvalidHistoryData
	}
	var content FunctionCallContent
	if err := json.Unmarshal(mc.Data, &content); err != nil {
		return nil, err
	}
	return &content, nil
}

// GetFunctionResponseContent extracts function response content from a MessageContent
func (mc *MessageContent) GetFunctionResponseContent() (*FunctionResponseContent, error) {
	if mc.Type != MessageContentTypeFunctionResponse {
		return nil, ErrInvalidHistoryData
	}
	var content FunctionResponseContent
	if err := json.Unmarshal(mc.Data, &content); err != nil {
		return nil, err
	}
	return &content, nil
}
