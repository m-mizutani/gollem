package gollem

import (
	"encoding/json"

	"github.com/gollem-dev/gollem/internal/jsonutil"
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
	// Provider holds data that only its issuer can interpret, such as a Claude
	// thinking signature or a Gemini thought signature. It is nil when the
	// content carries no such data.
	Provider *ProviderData `json:"provider,omitempty"`
}

// Issuer identifies who issued provider-bound data. Two issuers are the same
// only when every field is equal.
//
// Built-in clients set Provider to one of the LLMType constants. A custom
// client should use a Provider value different from those constants, unless
// it intends to exchange provider-bound data with the built-in client of that
// provider.
type Issuer struct {
	Provider LLMType `json:"provider"`
	Model    string  `json:"model"`
	Scope    string  `json:"scope,omitempty"`
}

// Equal reports whether x and y are the same issuer.
func (x Issuer) Equal(y Issuer) bool {
	return x.Provider == y.Provider && x.Model == y.Model && x.Scope == y.Scope
}

// ProviderData is data that only its issuer can interpret. Data is opaque to
// gollem and may be empty when only the issuer is recorded.
type ProviderData struct {
	Issuer Issuer          `json:"issuer"`
	Data   json.RawMessage `json:"data,omitempty"`
}

// FilterProviderData returns the messages to send to the provider identified
// by dest. A thinking content is kept only when its ProviderData was issued by
// dest. Any other content is kept, without its ProviderData when that was
// issued by someone else. A message left with no content is removed. The
// input is not modified.
//
// Every built-in client calls this before converting a History to its API
// format. A custom Session implementation should do the same, and record its
// own Issuer on the ProviderData it creates from API responses. Data that
// belongs to no text or tool call should be stored on a thinking content with
// empty text, so that it is removed for other issuers. Content types not
// defined by gollem are not supported: they are kept here and passed to the
// destination client, which may skip them or return an error.
func FilterProviderData(messages []Message, dest Issuer) []Message {
	if messages == nil {
		return nil
	}

	filtered := make([]Message, 0, len(messages))
	for _, msg := range messages {
		// A message that had no content to begin with is passed through, so
		// each converter keeps handling it as it did before.
		if len(msg.Contents) == 0 {
			filtered = append(filtered, msg)
			continue
		}

		contents := make([]MessageContent, 0, len(msg.Contents))
		for _, c := range msg.Contents {
			issuedByDest := c.Provider != nil && c.Provider.Issuer.Equal(dest)
			if c.Type == MessageContentTypeThinking {
				if issuedByDest {
					contents = append(contents, c)
				}
				continue
			}
			if c.Provider != nil && !issuedByDest {
				c.Provider = nil
			}
			contents = append(contents, c)
		}
		if len(contents) == 0 {
			continue
		}

		msg.Contents = contents
		filtered = append(filtered, msg)
	}
	return filtered
}

// MessageContentType represents the type of content in a message
type MessageContentType string

const (
	MessageContentTypeText         MessageContentType = "text"
	MessageContentTypeImage        MessageContentType = "image"
	MessageContentTypePDF          MessageContentType = "pdf"
	MessageContentTypeToolCall     MessageContentType = "tool_call"
	MessageContentTypeToolResponse MessageContentType = "tool_response"
	MessageContentTypeThinking     MessageContentType = "thinking"
)

// TextContent represents text content in a message
type TextContent struct {
	Text string `json:"text"`
}

// ThinkingContent represents thinking/reasoning content
type ThinkingContent struct {
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

// makeContent marshals v into JSON and wraps it in a MessageContent with the given type.
func makeContent[T any](t MessageContentType, v T) (MessageContent, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return MessageContent{}, err
	}
	return MessageContent{Type: t, Data: data}, nil
}

// decodeContent checks that mc has the expected type, then decodes its Data into T.
// The decode preserves numbers that a float64 cannot represent exactly, so a tool
// argument or result stored in a History is replayed to the provider unchanged.
func decodeContent[T any](t MessageContentType, mc *MessageContent) (*T, error) {
	if mc.Type != t {
		return nil, ErrInvalidHistoryData
	}
	var content T
	if err := jsonutil.Decode(mc.Data, &content); err != nil {
		return nil, err
	}
	return &content, nil
}

// Helper methods for creating MessageContent

// NewTextContent creates a new text message content
func NewTextContent(text string) (MessageContent, error) {
	return makeContent(MessageContentTypeText, TextContent{Text: text})
}

// NewThinkingContent creates a new thinking message content
func NewThinkingContent(text string) (MessageContent, error) {
	return makeContent(MessageContentTypeThinking, ThinkingContent{Text: text})
}

// NewImageContent creates a new image message content
func NewImageContent(mediaType string, imageData []byte, url string, detail string) (MessageContent, error) {
	return makeContent(MessageContentTypeImage, ImageContent{
		MediaType: mediaType,
		Data:      imageData,
		URL:       url,
		Detail:    detail,
	})
}

// NewPDFContent creates a new PDF message content
func NewPDFContent(pdfData []byte, url string) (MessageContent, error) {
	return makeContent(MessageContentTypePDF, PDFContent{Data: pdfData, URL: url})
}

// NewToolCallContent creates a new tool call message content
func NewToolCallContent(id, name string, args map[string]interface{}) (MessageContent, error) {
	return makeContent(MessageContentTypeToolCall, ToolCallContent{
		ID:        id,
		Name:      name,
		Arguments: args,
	})
}

// NewToolResponseContent creates a new tool response message content
func NewToolResponseContent(toolCallID, name string, response map[string]interface{}, isError bool) (MessageContent, error) {
	return makeContent(MessageContentTypeToolResponse, ToolResponseContent{
		ToolCallID: toolCallID,
		Name:       name,
		Response:   response,
		IsError:    isError,
	})
}

// Helper methods for extracting content from MessageContent

// GetTextContent extracts text content from a MessageContent
func (mc *MessageContent) GetTextContent() (*TextContent, error) {
	return decodeContent[TextContent](MessageContentTypeText, mc)
}

// GetImageContent extracts image content from a MessageContent
func (mc *MessageContent) GetImageContent() (*ImageContent, error) {
	return decodeContent[ImageContent](MessageContentTypeImage, mc)
}

// GetPDFContent extracts PDF content from a MessageContent
func (mc *MessageContent) GetPDFContent() (*PDFContent, error) {
	return decodeContent[PDFContent](MessageContentTypePDF, mc)
}

// GetToolCallContent extracts tool call content from a MessageContent
func (mc *MessageContent) GetToolCallContent() (*ToolCallContent, error) {
	return decodeContent[ToolCallContent](MessageContentTypeToolCall, mc)
}

// GetToolResponseContent extracts tool response content from a MessageContent
func (mc *MessageContent) GetToolResponseContent() (*ToolResponseContent, error) {
	return decodeContent[ToolResponseContent](MessageContentTypeToolResponse, mc)
}

// GetThinkingContent extracts thinking content from a MessageContent
func (mc *MessageContent) GetThinkingContent() (*ThinkingContent, error) {
	return decodeContent[ThinkingContent](MessageContentTypeThinking, mc)
}
