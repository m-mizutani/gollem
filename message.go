package gollem

import (
	"encoding/json"
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
	RoleTool      MessageRole = "tool" // Tool response (unified across all providers)
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
	MessageContentTypeText         MessageContentType = "text"
	MessageContentTypeImage        MessageContentType = "image"
	MessageContentTypePDF          MessageContentType = "pdf"
	MessageContentTypeToolCall     MessageContentType = "tool_call"
	MessageContentTypeToolResponse MessageContentType = "tool_response"
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

// PDFContent represents PDF document content in a message
type PDFContent struct {
	Data []byte `json:"data,omitempty"` // PDF data (base64 encoded in JSON)
	URL  string `json:"url,omitempty"`  // PDF URL (for future URL source support)
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

// NewPDFContent creates a new PDF message content
func NewPDFContent(pdfData []byte, url string) (MessageContent, error) {
	data, err := json.Marshal(PDFContent{
		Data: pdfData,
		URL:  url,
	})
	if err != nil {
		return MessageContent{}, err
	}
	return MessageContent{
		Type: MessageContentTypePDF,
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

// GetPDFContent extracts PDF content from a MessageContent
func (mc *MessageContent) GetPDFContent() (*PDFContent, error) {
	if mc.Type != MessageContentTypePDF {
		return nil, ErrInvalidHistoryData
	}
	var content PDFContent
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
