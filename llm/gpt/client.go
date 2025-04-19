package gpt

import (
	"context"
	"encoding/json"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/servantic"
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
func (c *Client) NewSession(ctx context.Context, tools []servantic.Tool) (servantic.Session, error) {
	// Convert servantic.Tool to openai.FunctionDefinition
	openaiTools := make([]openai.FunctionDefinition, len(tools))
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
	tools        []openai.FunctionDefinition
	messages     []openai.ChatCompletionMessage
}

func (s *Session) Generate(ctx context.Context, input ...servantic.Input) (*servantic.Response, error) {
	// Convert input to messages
	for _, in := range input {
		switch v := in.(type) {
		case servantic.Text:
			s.messages = append(s.messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: string(v),
			})
		case servantic.FunctionResponse:
			response, err := json.Marshal(v.Data)
			if err != nil {
				return nil, goerr.Wrap(err, "failed to marshal function response")
			}
			s.messages = append(s.messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleFunction,
				Name:    v.Name,
				Content: string(response),
			})
		default:
			return nil, goerr.Wrap(servantic.ErrInvalidParameter, "invalid input")
		}
	}

	resp, err := s.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:        s.defaultModel,
			Messages:     s.messages,
			Functions:    s.tools,
			FunctionCall: "auto",
		},
	)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create chat completion")
	}

	if len(resp.Choices) == 0 {
		return &servantic.Response{}, nil
	}

	response := &servantic.Response{
		Texts:         make([]string, 0),
		FunctionCalls: make([]*servantic.FunctionCall, 0),
	}

	message := resp.Choices[0].Message
	if message.Content != "" {
		response.Texts = append(response.Texts, message.Content)
	}

	if message.FunctionCall != nil {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(message.FunctionCall.Arguments), &args); err != nil {
			return nil, goerr.Wrap(err, "failed to unmarshal function arguments")
		}

		response.FunctionCalls = append(response.FunctionCalls, &servantic.FunctionCall{
			Name:      message.FunctionCall.Name,
			Arguments: args,
		})
	}

	return response, nil
}
