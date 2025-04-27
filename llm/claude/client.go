package claude

import (
	"context"
	"encoding/json"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollam"
)

// generationParameters represents the parameters for text generation.
type generationParameters struct {
	// Temperature controls randomness in the output.
	// Higher values make the output more random, lower values make it more focused.
	Temperature float64

	// TopP controls diversity via nucleus sampling.
	// Higher values allow more diverse outputs.
	TopP float64

	// MaxTokens limits the number of tokens to generate.
	MaxTokens int64
}

// Client is a client for the Claude API.
// It provides methods to interact with Anthropic's Claude models.
type Client struct {
	// client is the underlying Claude client.
	client *anthropic.Client

	// defaultModel is the model to use for chat completions.
	// It can be overridden using WithModel option.
	defaultModel string

	// generation parameters
	params generationParameters
}

// Option is a function that configures a Client.
type Option func(*Client)

// WithModel sets the default model to use for chat completions.
// The model name should be a valid Claude model identifier.
// Default: anthropic.ModelClaude3_5SonnetLatest
func WithModel(modelName string) Option {
	return func(c *Client) {
		c.defaultModel = modelName
	}
}

// WithTemperature sets the temperature parameter for text generation.
// Higher values make the output more random, lower values make it more focused.
// Range: 0.0 to 1.0
// Default: 0.7
func WithTemperature(temp float64) Option {
	return func(c *Client) {
		c.params.Temperature = temp
	}
}

// WithTopP sets the top_p parameter for text generation.
// Controls diversity via nucleus sampling.
// Range: 0.0 to 1.0
// Default: 1.0
func WithTopP(topP float64) Option {
	return func(c *Client) {
		c.params.TopP = topP
	}
}

// WithMaxTokens sets the maximum number of tokens to generate.
// Default: 4096
func WithMaxTokens(maxTokens int64) Option {
	return func(c *Client) {
		c.params.MaxTokens = maxTokens
	}
}

// New creates a new client for the Claude API.
// It requires an API key and can be configured with additional options.
func New(ctx context.Context, apiKey string, options ...Option) (*Client, error) {
	client := &Client{
		defaultModel: anthropic.ModelClaude3_5SonnetLatest,
		params: generationParameters{
			Temperature: 0.7,
			TopP:        1.0,
			MaxTokens:   4096,
		},
	}

	for _, option := range options {
		option(client)
	}

	newClient := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)
	client.client = &newClient

	return client, nil
}

// Session is a session for the Claude chat.
// It maintains the conversation state and handles message generation.
type Session struct {
	// client is the underlying Claude client.
	client *anthropic.Client

	// defaultModel is the model to use for chat completions.
	defaultModel string

	// tools are the available tools for the session.
	tools []anthropic.ToolUnionParam

	// messages stores the conversation history.
	messages []anthropic.MessageParam

	// generation parameters
	params generationParameters
}

// NewSession creates a new session for the Claude API.
// It converts the provided tools to Claude's tool format and initializes a new chat session.
func (c *Client) NewSession(ctx context.Context, tools []gollam.Tool) (gollam.Session, error) {
	// Convert gollam.Tool to anthropic.ToolUnionParam
	claudeTools := make([]anthropic.ToolUnionParam, len(tools))
	for i, tool := range tools {
		claudeTools[i] = convertTool(tool)
	}

	session := &Session{
		client:       c.client,
		defaultModel: c.defaultModel,
		tools:        claudeTools,
		params:       c.params,
	}

	return session, nil
}

// Generate processes the input and generates a response.
// It handles both text messages and function responses.
func (s *Session) Generate(ctx context.Context, input ...gollam.Input) (*gollam.Response, error) {
	var toolResults []anthropic.ContentBlockParamUnion
	// Convert input to messages
	for _, in := range input {
		switch v := in.(type) {
		case gollam.Text:
			s.messages = append(s.messages, anthropic.NewUserMessage(
				anthropic.NewTextBlock(string(v)),
			))

		case gollam.FunctionResponse:
			response, err := json.Marshal(v.Data)
			if err != nil {
				return nil, goerr.Wrap(err, "failed to marshal function response")
			}
			toolResults = append(toolResults, anthropic.NewToolResultBlock(v.ID, string(response), v.Error != nil))

		default:
			return nil, goerr.Wrap(gollam.ErrInvalidParameter, "invalid input")
		}
	}

	if len(toolResults) > 0 {
		s.messages = append(s.messages, anthropic.NewUserMessage(toolResults...))
	}

	params := anthropic.MessageNewParams{
		Model:       s.defaultModel,
		MaxTokens:   s.params.MaxTokens,
		Temperature: anthropic.Float(s.params.Temperature),
		TopP:        anthropic.Float(s.params.TopP),
		Tools:       s.tools,
		Messages:    s.messages,
	}

	resp, err := s.client.Messages.New(ctx, params)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create message")
	}
	s.messages = append(s.messages, resp.ToParam())

	if len(resp.Content) == 0 {
		return &gollam.Response{}, nil
	}

	response := &gollam.Response{
		Texts:         make([]string, 0),
		FunctionCalls: make([]*gollam.FunctionCall, 0),
	}

	for _, content := range resp.Content {
		textBlock := content.AsResponseTextBlock()
		if textBlock.Type == "text" {
			response.Texts = append(response.Texts, textBlock.Text)
		}

		toolUseBlock := content.AsResponseToolUseBlock()
		if toolUseBlock.Type == "tool_use" {
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolUseBlock.Input), &args); err != nil {
				return nil, goerr.Wrap(err, "failed to unmarshal function arguments")
			}

			response.FunctionCalls = append(response.FunctionCalls, &gollam.FunctionCall{
				ID:        toolUseBlock.ID,
				Name:      toolUseBlock.Name,
				Arguments: args,
			})
		}
	}

	return response, nil
}
