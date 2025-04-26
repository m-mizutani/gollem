package gpt

import (
	"context"
	"encoding/json"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollam"
	"github.com/sashabaranov/go-openai"
)

// Client is a client for the GPT API.
type Client struct {
	client       *openai.Client
	defaultModel string
}

type Option func(*Client)

func WithDefaultModel(modelName string) Option {
	return func(c *Client) {
		c.defaultModel = modelName
	}
}

// New creates a new client for the GPT API.
func New(ctx context.Context, apiKey string, options ...Option) (*Client, error) {
	client := &Client{
		defaultModel: "gpt-4-turbo-preview",
	}

	for _, option := range options {
		option(client)
	}

	openaiClient := openai.NewClient(apiKey)
	client.client = openaiClient

	return client, nil
}

// NewSession creates a new session for the GPT API.
func (c *Client) NewSession(ctx context.Context, tools []gollam.Tool) (gollam.Session, error) {
	// Convert gollam.Tool to openai.Tool
	openaiTools := make([]openai.Tool, len(tools))
	for i, tool := range tools {
		openaiTools[i] = convertTool(tool)
	}

	session := &Session{
		client:       c.client,
		defaultModel: c.defaultModel,
		tools:        openaiTools,
	}

	return session, nil
}

// Session is a session for the GPT chat.
type Session struct {
	client       *openai.Client
	defaultModel string
	tools        []openai.Tool
	messages     []openai.ChatCompletionMessage
}

func (s *Session) Generate(ctx context.Context, input ...gollam.Input) (*gollam.Response, error) {
	// Convert input to messages
	for _, in := range input {
		switch v := in.(type) {
		case gollam.Text:
			s.messages = append(s.messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: string(v),
			})
		case gollam.FunctionResponse:
			response, err := json.Marshal(v.Data)
			if err != nil {
				return nil, goerr.Wrap(err, "failed to marshal function response")
			}
			s.messages = append(s.messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    string(response),
				ToolCallID: v.ID,
			})

		default:
			return nil, goerr.Wrap(gollam.ErrInvalidParameter, "invalid input")
		}
	}

	req := openai.ChatCompletionRequest{
		Model:    s.defaultModel,
		Messages: s.messages,
		Tools:    s.tools,
	}

	resp, err := s.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create chat completion")
	}

	if len(resp.Choices) == 0 {
		return &gollam.Response{}, nil
	}

	response := &gollam.Response{
		Texts:         make([]string, 0),
		FunctionCalls: make([]*gollam.FunctionCall, 0),
	}

	message := resp.Choices[0].Message
	if message.Content != "" {
		response.Texts = append(response.Texts, message.Content)
		s.messages = append(s.messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: message.Content,
		})
	}

	if message.ToolCalls != nil {
		for _, toolCall := range message.ToolCalls {
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				return nil, goerr.Wrap(err, "failed to unmarshal tool arguments")
			}

			response.FunctionCalls = append(response.FunctionCalls, &gollam.FunctionCall{
				ID:        toolCall.ID,
				Name:      toolCall.Function.Name,
				Arguments: args,
			})
			s.messages = append(s.messages, openai.ChatCompletionMessage{
				Role:      openai.ChatMessageRoleAssistant,
				Content:   message.Content,
				ToolCalls: []openai.ToolCall{toolCall},
			})
		}
	}

	return response, nil
}
