// Package gollem provides a unified interface for interacting with various LLM services.
package gollem

import (
	"encoding/json"

	"cloud.google.com/go/vertexai/genai"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/m-mizutani/goerr/v2"
	"github.com/sashabaranov/go-openai"
)

// History represents a conversation history that can be used across different LLM sessions.
// It stores messages in a format specific to each LLM type (OpenAI, Claude, or Gemini).
//
// For detailed documentation, see doc/history.md
type LLMType string

const (
	llmTypeOpenAI LLMType = "OpenAI"
	llmTypeGemini LLMType = "gemini"
	llmTypeClaude LLMType = "claude"
)

const (
	HistoryVersion = 1
)

type History struct {
	LLType  LLMType `json:"type"`
	Version int     `json:"version"`

	Claude []claudeMessage                `json:"claude,omitempty"`
	OpenAI []openai.ChatCompletionMessage `json:"OpenAI,omitempty"`
	Gemini []geminiMessage                `json:"gemini,omitempty"`

	// Compaction related fields
	Summary     string `json:"summary,omitempty"`      // Summary information
	Compacted   bool   `json:"compacted,omitempty"`    // Compaction flag
	OriginalLen int    `json:"original_len,omitempty"` // Original length
}

func (x *History) ToCount() int {
	if x == nil {
		return 0
	}
	return len(x.Claude) + len(x.OpenAI) + len(x.Gemini)
}

func (x *History) Clone() *History {
	if x == nil {
		return nil
	}

	// Use JSON marshal/unmarshal for deep copy to avoid field-specific code
	// This ensures all fields are copied correctly even when structs are modified
	data, err := json.Marshal(x)
	if err != nil {
		// If marshaling fails, return a basic clone with empty messages
		// This should not happen in practice as History is designed to be JSON-serializable
		return &History{
			LLType:  x.LLType,
			Version: x.Version,
		}
	}

	var clone History
	if err := json.Unmarshal(data, &clone); err != nil {
		// If unmarshaling fails, return a basic clone with empty messages
		return &History{
			LLType:  x.LLType,
			Version: x.Version,
		}
	}

	return &clone
}

func (x *History) ToGemini() ([]*genai.Content, error) {
	if x.Version != HistoryVersion {
		return nil, goerr.Wrap(ErrHistoryVersionMismatch, "history version is not supported", goerr.V("expected", HistoryVersion), goerr.V("actual", x.Version))
	}
	if x.LLType != llmTypeGemini {
		return nil, goerr.Wrap(ErrLLMTypeMismatch, "history is not gemini", goerr.V("expected", llmTypeGemini), goerr.V("actual", x.LLType))
	}
	return toGeminiMessages(x.Gemini)
}

func (x *History) ToClaude() ([]anthropic.MessageParam, error) {
	if x.Version != HistoryVersion {
		return nil, goerr.Wrap(ErrHistoryVersionMismatch, "history version is not supported", goerr.V("expected", HistoryVersion), goerr.V("actual", x.Version))
	}
	if x.LLType != llmTypeClaude {
		return nil, goerr.Wrap(ErrLLMTypeMismatch, "history is not claude", goerr.V("expected", llmTypeClaude), goerr.V("actual", x.LLType))
	}
	return toClaudeMessages(x.Claude)
}

func (x *History) ToOpenAI() ([]openai.ChatCompletionMessage, error) {
	if x.Version != HistoryVersion {
		return nil, goerr.Wrap(ErrHistoryVersionMismatch, "history version is not supported", goerr.V("expected", HistoryVersion), goerr.V("actual", x.Version))
	}
	if x.LLType != llmTypeOpenAI {
		return nil, goerr.Wrap(ErrLLMTypeMismatch, "history is not OpenAI", goerr.V("expected", llmTypeOpenAI), goerr.V("actual", x.LLType))
	}
	return x.OpenAI, nil
}

type claudeMessage struct {
	Role    anthropic.MessageParamRole `json:"role"`
	Content []claudeContentBlock       `json:"content"`
}

type claudeContentBlock struct {
	Type       string             `json:"type"`
	Text       *string            `json:"text,omitempty"`
	Source     *claudeImageSource `json:"source,omitempty"`
	ToolUse    *claudeToolUse     `json:"tool_use,omitempty"`
	ToolResult *claudeToolResult  `json:"tool_result,omitempty"`
}

type claudeImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data"`
}

type claudeToolUse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input any    `json:"input"`
	Type  string `json:"type"`
}

type claudeToolResult struct {
	ToolUseID string          `json:"tool_use_id"`
	Content   string          `json:"content"`
	IsError   param.Opt[bool] `json:"is_error"`
}

type geminiMessage struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Type     string                 `json:"type"`
	Text     string                 `json:"text,omitempty"`
	MIMEType string                 `json:"mime_type,omitempty"`
	Data     []byte                 `json:"data,omitempty"`
	FileURI  string                 `json:"file_uri,omitempty"`
	Name     string                 `json:"name,omitempty"`
	Args     map[string]interface{} `json:"args,omitempty"`
	Response map[string]interface{} `json:"response,omitempty"`
}

func NewHistoryFromOpenAI(messages []openai.ChatCompletionMessage) *History {
	return &History{
		LLType:  llmTypeOpenAI,
		Version: HistoryVersion,
		OpenAI:  messages,
	}
}

func NewHistoryFromClaude(messages []anthropic.MessageParam) *History {
	claudeMessages := make([]claudeMessage, len(messages))
	for i, msg := range messages {
		content := make([]claudeContentBlock, len(msg.Content))
		for j, c := range msg.Content {
			if c.OfText != nil {
				content[j] = claudeContentBlock{
					Type: "text",
					Text: &c.OfText.Text,
				}
			} else if c.OfImage != nil {
				if c.OfImage.Source.OfBase64 != nil {
					content[j] = claudeContentBlock{
						Type: "image",
						Source: &claudeImageSource{
							Type:      "base64",
							MediaType: string(c.OfImage.Source.OfBase64.MediaType),
							Data:      c.OfImage.Source.OfBase64.Data,
						},
					}
				}
			} else if c.OfToolUse != nil {
				content[j] = claudeContentBlock{
					Type: "tool_use",
					ToolUse: &claudeToolUse{
						ID:    c.OfToolUse.ID,
						Name:  c.OfToolUse.Name,
						Input: c.OfToolUse.Input,
						Type:  string(c.OfToolUse.Type),
					},
				}
			} else if c.OfToolResult != nil {
				content[j] = claudeContentBlock{
					Type: "tool_result",
					ToolResult: &claudeToolResult{
						ToolUseID: c.OfToolResult.ToolUseID,
						Content:   c.OfToolResult.Content[0].OfText.Text,
						IsError:   c.OfToolResult.IsError,
					},
				}
			} else {
				// This else clause will catch unhandled content types
				// and create an empty content block
				content[j] = claudeContentBlock{}
			}
		}
		claudeMessages[i] = claudeMessage{
			Role:    msg.Role,
			Content: content,
		}
	}

	return &History{
		LLType:  llmTypeClaude,
		Version: HistoryVersion,
		Claude:  claudeMessages,
	}
}

func toClaudeMessages(messages []claudeMessage) ([]anthropic.MessageParam, error) {
	converted := make([]anthropic.MessageParam, len(messages))

	for i, msg := range messages {
		content := make([]anthropic.ContentBlockParamUnion, 0, len(msg.Content))
		for _, c := range msg.Content {
			switch c.Type {
			case "text":
				if c.Text == nil {
					return nil, goerr.New("text block has no text field")
				}
				content = append(content, anthropic.NewTextBlock(*c.Text))

			case "image":
				if c.Source == nil {
					return nil, goerr.New("image block has no source field")
				}
				if c.Source.Type == "base64" || c.Source.Type == "" {
					content = append(content, anthropic.NewImageBlockBase64(c.Source.MediaType, c.Source.Data))
				}

			case "tool_use":
				if c.ToolUse == nil {
					return nil, goerr.New("tool_use block has no tool_use field")
				}
				content = append(content, anthropic.NewToolUseBlock(c.ToolUse.ID, c.ToolUse.Input, c.ToolUse.Name))

			case "tool_result":
				if c.ToolResult == nil {
					return nil, goerr.New("tool_result block has no tool_result field")
				}
				toolResult := anthropic.NewToolResultBlock(c.ToolResult.ToolUseID)

				// Set content
				if c.ToolResult.Content != "" {
					toolResult.OfToolResult.Content = []anthropic.ToolResultBlockParamContentUnion{
						{OfText: &anthropic.TextBlockParam{Text: c.ToolResult.Content}},
					}
				}

				// Set error flag
				if c.ToolResult.IsError.Valid() {
					toolResult.OfToolResult.IsError = param.NewOpt(c.ToolResult.IsError.Value)
				}

				content = append(content, toolResult)
			case "":
				// Skip empty content blocks
				continue
			default:
				return nil, goerr.New("unknown content block type", goerr.V("type", c.Type))
			}
		}
		converted[i] = anthropic.MessageParam{
			Role:    msg.Role,
			Content: content,
		}
	}

	return converted, nil
}

func NewHistoryFromGemini(messages []*genai.Content) *History {
	converted := make([]geminiMessage, len(messages))
	for i, msg := range messages {
		parts := make([]geminiPart, len(msg.Parts))
		for j, p := range msg.Parts {
			switch v := p.(type) {
			case genai.Text:
				parts[j] = geminiPart{
					Type: "text",
					Text: string(v),
				}
			case genai.Blob:
				parts[j] = geminiPart{
					Type:     "blob",
					MIMEType: v.MIMEType,
					Data:     v.Data,
				}
			case genai.FileData:
				parts[j] = geminiPart{
					Type:     "file_data",
					MIMEType: v.MIMEType,
					FileURI:  v.FileURI,
				}
			case genai.FunctionCall:
				parts[j] = geminiPart{
					Type: "function_call",
					Name: v.Name,
					Args: v.Args,
				}
			case genai.FunctionResponse:
				parts[j] = geminiPart{
					Type:     "function_response",
					Name:     v.Name,
					Response: v.Response,
				}
			}
		}
		converted[i] = geminiMessage{
			Role:  msg.Role,
			Parts: parts,
		}
	}
	return &History{
		LLType:  llmTypeGemini,
		Version: HistoryVersion,
		Gemini:  converted,
	}
}

func toGeminiMessages(messages []geminiMessage) ([]*genai.Content, error) {
	converted := make([]*genai.Content, len(messages))
	for i, msg := range messages {
		parts := make([]genai.Part, len(msg.Parts))
		for j, p := range msg.Parts {
			switch p.Type {
			case "text":
				parts[j] = genai.Text(p.Text)
			case "blob":
				parts[j] = genai.Blob{
					MIMEType: p.MIMEType,
					Data:     p.Data,
				}
			case "file_data":
				parts[j] = genai.FileData{
					MIMEType: p.MIMEType,
					FileURI:  p.FileURI,
				}
			case "function_call":
				parts[j] = genai.FunctionCall{
					Name: p.Name,
					Args: p.Args,
				}
			case "function_response":
				parts[j] = genai.FunctionResponse{
					Name:     p.Name,
					Response: p.Response,
				}
			}
		}
		converted[i] = &genai.Content{
			Role:  msg.Role,
			Parts: parts,
		}
	}
	return converted, nil
}
