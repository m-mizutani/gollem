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

type LLMType string

const (
	LLMTypeGPT    LLMType = "gpt"
	LLMTypeGemini LLMType = "gemini"
	LLMTypeClaude LLMType = "claude"
)

type History struct {
	historyData
}

type claudeMessage struct {
	Role    string               `json:"role"`
	Content []claudeContentBlock `json:"content"`
}

type claudeContentBlock struct {
	Type       string           `json:"type"`
	Text       *string          `json:"text,omitempty"`
	Source     *imageBlock      `json:"source,omitempty"`
	ToolUse    *toolUseBlock    `json:"tool_use,omitempty"`
	ToolResult *toolResultBlock `json:"tool_result,omitempty"`
}

type imageBlock struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type toolUseBlock struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"input"`
	Type  string `json:"type"`
}

type toolResultBlock struct {
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
		historyData: historyData{
			LLType:   LLMTypeGPT,
			Messages: messages,
		},
	}
}

func NewHistoryFromClaude(messages []anthropic.MessageParam) *History {
	converted := make([]claudeMessage, len(messages))
	for i, msg := range messages {
		content := make([]claudeContentBlock, len(msg.Content))
		for j, c := range msg.Content {
			if c.OfRequestTextBlock != nil {
				text := c.OfRequestTextBlock.Text
				content[j] = claudeContentBlock{
					Type: "text",
					Text: &text,
				}
			} else if c.OfRequestImageBlock != nil {
				if c.OfRequestImageBlock.Source.OfBase64ImageSource != nil {
					content[j] = claudeContentBlock{
						Type: "image",
						Source: &imageBlock{
							Type:      string(c.OfRequestImageBlock.Source.OfBase64ImageSource.Type),
							MediaType: string(c.OfRequestImageBlock.Source.OfBase64ImageSource.MediaType),
							Data:      c.OfRequestImageBlock.Source.OfBase64ImageSource.Data,
						},
					}
				} else if c.OfRequestImageBlock.Source.OfURLImageSource != nil {
					content[j] = claudeContentBlock{
						Type: "image",
						Source: &imageBlock{
							Type: string(c.OfRequestImageBlock.Source.OfURLImageSource.Type),
							Data: c.OfRequestImageBlock.Source.OfURLImageSource.URL,
						},
					}
				}
			} else if c.OfRequestToolUseBlock != nil {
				content[j] = claudeContentBlock{
					Type: "tool_use",
					ToolUse: &toolUseBlock{
						ID:    c.OfRequestToolUseBlock.ID,
						Name:  c.OfRequestToolUseBlock.Name,
						Input: fmt.Sprintf("%v", c.OfRequestToolUseBlock.Input),
						Type:  string(c.OfRequestToolUseBlock.Type),
					},
				}
			} else if c.OfRequestToolResultBlock != nil {
				content[j] = claudeContentBlock{
					Type: "tool_result",
					ToolResult: &toolResultBlock{
						ToolUseID: c.OfRequestToolResultBlock.ToolUseID,
						Content:   c.OfRequestToolResultBlock.Content[0].OfRequestTextBlock.Text,
						IsError:   c.OfRequestToolResultBlock.IsError,
					},
				}
			}
		}
		converted[i] = claudeMessage{
			Role:    string(msg.Role),
			Content: content,
		}
	}
	return &History{
		historyData: historyData{
			LLType:   LLMTypeClaude,
			Messages: converted,
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
		historyData: historyData{
			LLType:   LLMTypeGemini,
			Messages: converted,
		},
	}
}

func NewHistoryFromData(data []byte) (*History, error) {
	var check struct {
		LLType LLMType `json:"type"`
	}
	if err := json.Unmarshal(data, &check); err != nil {
		return nil, goerr.Wrap(err, "invalid history data", goerr.V("data", string(data)))
	}

	switch check.LLType {
	case LLMTypeGPT:
		var history struct {
			Messages []openai.ChatCompletionMessage `json:"messages"`
		}
		if err := json.Unmarshal(data, &history); err != nil {
			return nil, goerr.Wrap(err, "failed to unmarshal gpt history", goerr.V("data", string(data)))
		}
		return NewHistoryFromGPT(history.Messages), nil

	case LLMTypeClaude:
		var history struct {
			Messages []claudeMessage `json:"messages"`
		}
		if err := json.Unmarshal(data, &history); err != nil {
			return nil, goerr.Wrap(err, "failed to unmarshal claude history", goerr.V("data", string(data)))
		}

		messages := make([]anthropic.MessageParam, len(history.Messages))
		for i, msg := range history.Messages {
			content := make([]anthropic.ContentBlockParamUnion, len(msg.Content))
			for j, c := range msg.Content {
				if c.Type == "text" && c.Text != nil {
					content[j] = anthropic.ContentBlockParamUnion{
						OfRequestTextBlock: &anthropic.TextBlockParam{
							Text: *c.Text,
						},
					}
				} else if c.Type == "image" && c.Source != nil {
					if c.Source.Type == "base64" {
						content[j] = anthropic.ContentBlockParamUnion{
							OfRequestImageBlock: &anthropic.ImageBlockParam{
								Source: anthropic.ImageBlockParamSourceUnion{
									OfBase64ImageSource: &anthropic.Base64ImageSourceParam{
										Type:      "base64",
										MediaType: anthropic.Base64ImageSourceMediaType(c.Source.MediaType),
										Data:      c.Source.Data,
									},
								},
							},
						}
					} else if c.Source.Type == "url" {
						content[j] = anthropic.ContentBlockParamUnion{
							OfRequestImageBlock: &anthropic.ImageBlockParam{
								Source: anthropic.ImageBlockParamSourceUnion{
									OfURLImageSource: &anthropic.URLImageSourceParam{
										Type: "url",
										URL:  c.Source.Data,
									},
								},
							},
						}
					}
				} else if c.Type == "tool_use" && c.ToolUse != nil {
					content[j] = anthropic.ContentBlockParamUnion{
						OfRequestToolUseBlock: &anthropic.ToolUseBlockParam{
							ID:    c.ToolUse.ID,
							Name:  c.ToolUse.Name,
							Input: c.ToolUse.Input,
							Type:  "tool_use",
						},
					}
				} else if c.Type == "tool_result" && c.ToolResult != nil {
					content[j] = anthropic.ContentBlockParamUnion{
						OfRequestToolResultBlock: &anthropic.ToolResultBlockParam{
							ToolUseID: c.ToolResult.ToolUseID,
							Content: []anthropic.ToolResultBlockParamContentUnion{
								{
									OfRequestTextBlock: &anthropic.TextBlockParam{
										Text: c.ToolResult.Content,
									},
								},
							},
							IsError: c.ToolResult.IsError,
						},
					}
				}
			}
			messages[i] = anthropic.MessageParam{
				Role:    anthropic.MessageParamRole(msg.Role),
				Content: content,
			}
		}
		return &History{
			historyData: historyData{
				LLType:   LLMTypeClaude,
				Messages: messages,
			},
		}, nil

	case LLMTypeGemini:
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
		return &History{
			historyData: historyData{
				LLType:   LLMTypeGemini,
				Messages: messages,
			},
		}, nil
	}

	return nil, goerr.Wrap(ErrInvalidHistoryData, "unsupported history data", goerr.V("data", string(data)))
}

type historyData struct {
	LLType   LLMType `json:"type"`
	Messages any     `json:"messages"`
}

func (h *History) MarshalJSON() ([]byte, error) {
	return json.Marshal(h.historyData)
}

func (h *History) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &h.historyData)
}
