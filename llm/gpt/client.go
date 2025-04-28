package gpt

import (
	"context"
	"encoding/json"
	"io"
	"strings"

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

	// systemPrompt is the system prompt to use for chat completions.
	systemPrompt string
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

// WithSystemPrompt sets the system prompt to use for chat completions.
func WithSystemPrompt(prompt string) Option {
	return func(c *Client) {
		c.systemPrompt = prompt
	}
}

// New creates a new client for the GPT API.
// It requires an API key and can be configured with additional options.
func New(ctx context.Context, apiKey string, options ...Option) (*Client, error) {
	client := &Client{
		defaultModel: "gpt-4-turbo-preview",
		params:       generationParameters{},
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
func (c *Client) NewSession(ctx context.Context, tools []gollam.Tool, histories ...*gollam.History) (gollam.Session, error) {
	// Convert gollam.Tool to openai.Tool
	openaiTools := make([]openai.Tool, len(tools))
	for i, tool := range tools {
		openaiTools[i] = convertTool(tool)
	}

	var messages []openai.ChatCompletionMessage
	if c.systemPrompt != "" {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: c.systemPrompt,
		})
	}
	for _, history := range histories {
		history, err := history.ToGPT()
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert history to openai.ChatCompletionMessage")
		}
		messages = append(messages, history...)
	}

	session := &Session{
		client:       c.client,
		defaultModel: c.defaultModel,
		tools:        openaiTools,
		params:       c.params,
		messages:     messages,
	}

	return session, nil
}

func (s *Session) History() *gollam.History {
	return gollam.NewHistoryFromGPT(s.messages)
}

// convertInputs converts gollam.Input to OpenAI messages
func (s *Session) convertInputs(input ...gollam.Input) error {
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
				return goerr.Wrap(err, "failed to marshal function response")
			}
			s.messages = append(s.messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    string(response),
				ToolCallID: v.ID,
			})
		default:
			return goerr.Wrap(gollam.ErrInvalidParameter, "invalid input")
		}
	}
	return nil
}

// createRequest creates a chat completion request with the current session state
func (s *Session) createRequest(stream bool) openai.ChatCompletionRequest {
	return openai.ChatCompletionRequest{
		Model:            s.defaultModel,
		Messages:         s.messages,
		Tools:            s.tools,
		Temperature:      s.params.Temperature,
		TopP:             s.params.TopP,
		MaxTokens:        s.params.MaxTokens,
		PresencePenalty:  s.params.PresencePenalty,
		FrequencyPenalty: s.params.FrequencyPenalty,
		Stream:           stream,
	}
}

// GenerateContent processes the input and generates a response.
// It handles both text messages and function responses.
func (s *Session) GenerateContent(ctx context.Context, input ...gollam.Input) (*gollam.Response, error) {
	if err := s.convertInputs(input...); err != nil {
		return nil, err
	}

	req := s.createRequest(false)
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

// accumulator accumulates streaming responses for function calls
type accumulator struct {
	ID   string
	Name string
	Args string
}

func newAccumulator() *accumulator {
	return &accumulator{}
}

func (a *accumulator) addFunctionCall(toolCall *openai.ToolCall) {
	if toolCall.ID != "" {
		a.ID = toolCall.ID
	}
	if toolCall.Function.Name != "" {
		a.Name = toolCall.Function.Name
	}
	if toolCall.Function.Arguments != "" {
		a.Args += toolCall.Function.Arguments
	}
}

func (a *accumulator) accumulate() (*openai.ToolCall, *gollam.FunctionCall, error) {
	if a.ID == "" || a.Name == "" || a.Args == "" {
		return nil, nil, goerr.Wrap(gollam.ErrInvalidParameter, "function call is not complete")
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(a.Args), &args); err != nil {
		return nil, nil, goerr.Wrap(err, "failed to unmarshal function call arguments", goerr.V("accumulator", a))
	}

	return &openai.ToolCall{
			ID:   a.ID,
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name:      a.Name,
				Arguments: a.Args,
			},
		}, &gollam.FunctionCall{
			ID:        a.ID,
			Name:      a.Name,
			Arguments: args,
		}, nil
}

// GenerateStream processes the input and generates a response stream.
// It handles both text messages and function responses, and returns a channel for streaming responses.
func (s *Session) GenerateStream(ctx context.Context, input ...gollam.Input) (<-chan *gollam.Response, error) {
	if err := s.convertInputs(input...); err != nil {
		return nil, err
	}

	req := s.createRequest(true)
	stream, err := s.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create chat completion stream")
	}

	responseChan := make(chan *gollam.Response)
	acc := newAccumulator()

	callHistory := make([]openai.ToolCall, 0)
	textHistory := make([]string, 0)

	go func() {
		defer close(responseChan)
		defer stream.Close()

		defer func() {
			if len(textHistory) > 0 {
				s.messages = append(s.messages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleAssistant,
					Content: strings.Join(textHistory, ""),
				})
			}
			if len(callHistory) > 0 {
				s.messages = append(s.messages, openai.ChatCompletionMessage{
					Role:      openai.ChatMessageRoleAssistant,
					ToolCalls: callHistory,
				})
			}
		}()

		for {
			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					return
				}
				responseChan <- &gollam.Response{
					Error: goerr.Wrap(err, "failed to receive chat completion stream"),
				}
				return
			}

			if len(resp.Choices) == 0 {
				return
			}

			for _, choice := range resp.Choices {
				if choice.Delta.Content != "" {
					// Send text immediately for each delta.Content
					responseChan <- &gollam.Response{
						Texts: []string{choice.Delta.Content},
					}
				}

				if choice.Delta.ToolCalls != nil {
					for _, toolCall := range choice.Delta.ToolCalls {
						acc.addFunctionCall(&toolCall)
					}
				}

				if choice.FinishReason == openai.FinishReasonToolCalls {
					openaiCall, funcCall, err := acc.accumulate()
					if err != nil {
						responseChan <- &gollam.Response{
							Error: err,
						}
						return
					}

					callHistory = append(callHistory, *openaiCall)
					responseChan <- &gollam.Response{
						FunctionCalls: []*gollam.FunctionCall{funcCall},
					}

					acc = newAccumulator()
				}
			}
		}
	}()

	return responseChan, nil
}
