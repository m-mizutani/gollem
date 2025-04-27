package gpt

import (
	"context"
	"encoding/json"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollam"
	"github.com/sashabaranov/go-openai"
)

// generationParameters represents the parameters for text generation.
type generationParameters struct {
	// Temperature controls randomness in the output.
	// Higher values make the output more random, lower values make it more focused.
	Temperature float32

	// TopP controls diversity via nucleus sampling.
	// Higher values allow more diverse outputs.
	TopP float32

	// MaxTokens limits the number of tokens to generate.
	MaxTokens int

	// PresencePenalty increases the model's likelihood to talk about new topics.
	// Range: -2.0 to 2.0
	PresencePenalty float32

	// FrequencyPenalty decreases the model's likelihood to repeat the same line verbatim.
	// Range: -2.0 to 2.0
	FrequencyPenalty float32
}

// Client is a client for the GPT API.
// It provides methods to interact with OpenAI's GPT models.
type Client struct {
	// client is the underlying OpenAI client.
	client *openai.Client

	// defaultModel is the model to use for chat completions.
	// It can be overridden using WithModel option.
	defaultModel string

	// generation parameters
	params generationParameters
}

// Option is a function that configures a Client.
type Option func(*Client)

// WithModel sets the default model to use for chat completions.
// The model name should be a valid OpenAI model identifier.
func WithModel(modelName string) Option {
	return func(c *Client) {
		c.defaultModel = modelName
	}
}

// WithTemperature sets the temperature parameter for text generation.
// Higher values make the output more random, lower values make it more focused.
func WithTemperature(temp float32) Option {
	return func(c *Client) {
		c.params.Temperature = temp
	}
}

// WithTopP sets the top_p parameter for text generation.
// Controls diversity via nucleus sampling.
func WithTopP(topP float32) Option {
	return func(c *Client) {
		c.params.TopP = topP
	}
}

// WithMaxTokens sets the maximum number of tokens to generate.
func WithMaxTokens(maxTokens int) Option {
	return func(c *Client) {
		c.params.MaxTokens = maxTokens
	}
}

// WithPresencePenalty sets the presence penalty parameter.
// Increases the model's likelihood to talk about new topics.
func WithPresencePenalty(penalty float32) Option {
	return func(c *Client) {
		c.params.PresencePenalty = penalty
	}
}

// WithFrequencyPenalty sets the frequency penalty parameter.
// Decreases the model's likelihood to repeat the same line verbatim.
func WithFrequencyPenalty(penalty float32) Option {
	return func(c *Client) {
		c.params.FrequencyPenalty = penalty
	}
}

// New creates a new client for the GPT API.
// It requires an API key and can be configured with additional options.
func New(ctx context.Context, apiKey string, options ...Option) (*Client, error) {
	client := &Client{
		defaultModel: "gpt-4-turbo-preview",
		params: generationParameters{
			Temperature: 1.0,
			TopP:        1.0,
		},
	}

	for _, option := range options {
		option(client)
	}

	openaiClient := openai.NewClient(apiKey)
	client.client = openaiClient

	return client, nil
}

// Session is a session for the GPT chat.
// It maintains the conversation state and handles message generation.
type Session struct {
	// client is the underlying OpenAI client.
	client *openai.Client

	// defaultModel is the model to use for chat completions.
	defaultModel string

	// tools are the available tools for the session.
	tools []openai.Tool

	// messages stores the conversation history.
	messages []openai.ChatCompletionMessage

	// generation parameters
	params generationParameters
}

// NewSession creates a new session for the GPT API.
// It converts the provided tools to OpenAI's tool format and initializes a new chat session.
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
		params:       c.params,
	}

	return session, nil
}

// Generate processes the input and generates a response.
// It handles both text messages and function responses.
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
		Model:            s.defaultModel,
		Messages:         s.messages,
		Tools:            s.tools,
		Temperature:      s.params.Temperature,
		TopP:             s.params.TopP,
		MaxTokens:        s.params.MaxTokens,
		PresencePenalty:  s.params.PresencePenalty,
		FrequencyPenalty: s.params.FrequencyPenalty,
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
