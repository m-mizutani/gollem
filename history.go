package gollam

import (
	"encoding/json"
	"fmt"

	"cloud.google.com/go/vertexai/genai"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/m-mizutani/goerr/v2"
	"github.com/sashabaranov/go-openai"
)

type llmType string

const (
	llmTypeGPT    llmType = "gpt"
	llmTypeGemini llmType = "gemini"
	llmTypeClaude llmType = "claude"
)

type History struct {
	history historyData
}

type historyData struct {
	LLType   llmType `json:"type"`
	Messages any     `json:"messages"`

	gptMessages    []openai.ChatCompletionMessage
	claudeMessages []anthropic.MessageParam
	geminiMessages []*genai.Content
}

func (h *History) MarshalJSON() ([]byte, error) {
	return json.Marshal(h.history)
}

func (h *History) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &h.history)
}

func (x *History) ToGemini() ([]*genai.Content, error) {
	if x.history.LLType != llmTypeGemini {
		return nil, goerr.Wrap(ErrLLMTypeMismatch, "history is not gemini")
	}
	return x.history.geminiMessages, nil
}

func (x *History) ToClaude() ([]anthropic.MessageParam, error) {
	if x.history.LLType != llmTypeClaude {
		return nil, goerr.Wrap(ErrLLMTypeMismatch, "history is not claude")
	}
	return x.history.claudeMessages, nil
}

func (x *History) ToGPT() ([]openai.ChatCompletionMessage, error) {
	if x.history.LLType != llmTypeGPT {
		return nil, goerr.Wrap(ErrLLMTypeMismatch, "history is not gpt")
	}
	return x.history.gptMessages, nil
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
	Input string `json:"input"`
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

func NewHistoryFromGPT(messages []openai.ChatCompletionMessage) *History {
	return &History{
		history: historyData{
			LLType:      llmTypeGPT,
			Messages:    messages,
			gptMessages: messages,
		},
	}
}

func NewHistoryFromClaude(messages []anthropic.MessageParam) *History {
	claudeMessages := make([]claudeMessage, len(messages))
	for i, msg := range messages {
		content := make([]claudeContentBlock, len(msg.Content))
		for j, c := range msg.Content {
			if c.OfRequestTextBlock != nil {
				content[j] = claudeContentBlock{
					Type: "text",
					Text: &c.OfRequestTextBlock.Text,
				}
			} else if c.OfRequestImageBlock != nil {
				if c.OfRequestImageBlock.Source.OfBase64ImageSource != nil {
					content[j] = claudeContentBlock{
						Type: "image",
						Source: &claudeImageSource{
							Type:      string(c.OfRequestImageBlock.Source.OfBase64ImageSource.Type),
							MediaType: string(c.OfRequestImageBlock.Source.OfBase64ImageSource.MediaType),
							Data:      c.OfRequestImageBlock.Source.OfBase64ImageSource.Data,
						},
					}
				}
			} else if c.OfRequestToolUseBlock != nil {
				content[j] = claudeContentBlock{
					Type: "tool_use",
					ToolUse: &claudeToolUse{
						ID:    c.OfRequestToolUseBlock.ID,
						Name:  c.OfRequestToolUseBlock.Name,
						Input: fmt.Sprintf("%v", c.OfRequestToolUseBlock.Input),
						Type:  string(c.OfRequestToolUseBlock.Type),
					},
				}
			} else if c.OfRequestToolResultBlock != nil {
				content[j] = claudeContentBlock{
					Type: "tool_result",
					ToolResult: &claudeToolResult{
						ToolUseID: c.OfRequestToolResultBlock.ToolUseID,
						Content:   c.OfRequestToolResultBlock.Content[0].OfRequestTextBlock.Text,
						IsError:   c.OfRequestToolResultBlock.IsError,
					},
				}
			}
		}
		claudeMessages[i] = claudeMessage{
			Role:    msg.Role,
			Content: content,
		}
	}
	return &History{
		history: historyData{
			LLType:         llmTypeClaude,
			Messages:       claudeMessages,
			claudeMessages: messages,
		},
	}
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
			case *genai.Blob:
				parts[j] = geminiPart{
					Type:     "blob",
					MIMEType: v.MIMEType,
					Data:     v.Data,
				}
			case *genai.FileData:
				parts[j] = geminiPart{
					Type:     "file_data",
					MIMEType: v.MIMEType,
					FileURI:  v.FileURI,
				}
			case *genai.FunctionCall:
				parts[j] = geminiPart{
					Type: "function_call",
					Name: v.Name,
					Args: v.Args,
				}
			case *genai.FunctionResponse:
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
		history: historyData{
			LLType:         llmTypeGemini,
			Messages:       converted,
			geminiMessages: messages,
		},
	}
}

func NewHistoryFromData(data []byte) (*History, error) {
	var check struct {
		LLType llmType `json:"type"`
	}
	if err := json.Unmarshal(data, &check); err != nil {
		return nil, goerr.Wrap(err, "invalid history data", goerr.V("data", string(data)))
	}

	switch check.LLType {
	case llmTypeGPT:
		var history struct {
			Messages []openai.ChatCompletionMessage `json:"messages"`
		}
		if err := json.Unmarshal(data, &history); err != nil {
			return nil, goerr.Wrap(err, "failed to unmarshal gpt history", goerr.V("data", string(data)))
		}
		return NewHistoryFromGPT(history.Messages), nil

	case llmTypeClaude:
		var history struct {
			Messages []claudeMessage `json:"messages"`
		}
		if err := json.Unmarshal(data, &history); err != nil {
			return nil, goerr.Wrap(err, "failed to unmarshal claude history", goerr.V("data", string(data)))
		}

		messages := make([]anthropic.MessageParam, len(history.Messages))
		for i, msg := range history.Messages {
			content := make([]anthropic.ContentBlockParamUnion, 0, len(msg.Content))
			for _, c := range msg.Content {
				switch c.Type {
				case "text":
					if c.Text == nil {
						return nil, goerr.New("text block has no text field")
					}
					content = append(content, anthropic.ContentBlockParamUnion{
						OfRequestTextBlock: &anthropic.TextBlockParam{
							Text: *c.Text,
							Type: "text",
						},
					})

				case "image":
					if c.Source == nil {
						return nil, goerr.New("image block has no source field")
					}
					if c.Source.Type == "base64" {
						content = append(content, anthropic.ContentBlockParamUnion{
							OfRequestImageBlock: &anthropic.ImageBlockParam{
								Source: anthropic.ImageBlockParamSourceUnion{
									OfBase64ImageSource: &anthropic.Base64ImageSourceParam{
										Type:      "base64",
										MediaType: anthropic.Base64ImageSourceMediaType(c.Source.MediaType),
										Data:      c.Source.Data,
									},
								},
								Type: "image",
							},
						})
					}

				case "tool_use":
					if c.ToolUse == nil {
						return nil, goerr.New("tool_use block has no tool_use field")
					}
					content = append(content, anthropic.ContentBlockParamUnion{
						OfRequestToolUseBlock: &anthropic.ToolUseBlockParam{
							ID:    c.ToolUse.ID,
							Name:  c.ToolUse.Name,
							Input: c.ToolUse.Input,
							Type:  "tool_use",
						},
					})

				case "tool_result":
					if c.ToolResult == nil {
						return nil, goerr.New("tool_result block has no tool_result field")
					}
					content = append(content, anthropic.ContentBlockParamUnion{
						OfRequestToolResultBlock: &anthropic.ToolResultBlockParam{
							ToolUseID: c.ToolResult.ToolUseID,
							Content: []anthropic.ToolResultBlockParamContentUnion{
								{
									OfRequestTextBlock: &anthropic.TextBlockParam{
										Text: c.ToolResult.Content,
										Type: "text",
									},
								},
							},
							IsError: param.NewOpt(c.ToolResult.IsError.Value),
						},
					})
				}
			}

			messages[i] = anthropic.MessageParam{
				Role:    msg.Role,
				Content: content,
			}
		}
		return NewHistoryFromClaude(messages), nil

	case llmTypeGemini:
		var history struct {
			Messages []geminiMessage `json:"messages"`
		}
		if err := json.Unmarshal(data, &history); err != nil {
			return nil, goerr.Wrap(err, "failed to unmarshal gemini history", goerr.V("data", string(data)))
		}

		messages := make([]*genai.Content, len(history.Messages))
		for i, msg := range history.Messages {
			parts := make([]genai.Part, len(msg.Parts))
			for j, p := range msg.Parts {
				switch p.Type {
				case "text":
					parts[j] = genai.Text(p.Text)
				case "blob":
					parts[j] = &genai.Blob{
						MIMEType: p.MIMEType,
						Data:     p.Data,
					}
				case "file_data":
					parts[j] = &genai.FileData{
						MIMEType: p.MIMEType,
						FileURI:  p.FileURI,
					}
				case "function_call":
					parts[j] = &genai.FunctionCall{
						Name: p.Name,
						Args: p.Args,
					}
				case "function_response":
					parts[j] = &genai.FunctionResponse{
						Name:     p.Name,
						Response: p.Response,
					}
				}
			}
			messages[i] = &genai.Content{
				Role:  msg.Role,
				Parts: parts,
			}
		}
		return NewHistoryFromGemini(messages), nil
	}

	return nil, goerr.Wrap(ErrInvalidHistoryData, "unsupported history data", goerr.V("data", string(data)))
}
