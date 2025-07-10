package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
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

// Client is a client for the OpenAI API.
// It provides methods to interact with OpenAI's OpenAI models.
type Client struct {
	// client is the underlying OpenAI client.
	client *openai.Client

	// defaultModel is the model to use for chat completions.
	// It can be overridden using WithModel option.
	defaultModel string

	// embeddingModel is the model to use for embeddings.
	// It can be overridden using WithEmbeddingModel option.
	embeddingModel string

	// generation parameters
	params generationParameters

	// systemPrompt is the system prompt to use for chat completions.
	systemPrompt string

	// contentType is the type of content to be generated.
	contentType gollem.ContentType
}

const (
	DefaultModel          = "gpt-4.1"
	DefaultEmbeddingModel = "text-embedding-3-small"
)

// Option is a function that configures a Client.
type Option func(*Client)

// WithModel sets the default model to use for chat completions.
// The model name should be a valid OpenAI model identifier.
// See default model in [DefaultModel].
func WithModel(modelName string) Option {
	return func(c *Client) {
		c.defaultModel = modelName
	}
}

// WithEmbeddingModel sets the embedding model to use for embeddings.
// The model name should be a valid OpenAI model identifier.
// See default embedding model in [DefaultEmbeddingModel].
// Model list is at https://platform.openai.com/docs/guides/embeddings#embedding-models
func WithEmbeddingModel(modelName string) Option {
	return func(c *Client) {
		c.embeddingModel = modelName
	}
}

// WithTemperature sets the temperature parameter for text generation.
// Higher values make the output more random, lower values make it more focused.
// Range: 0.0 to 1.0
// Default: 0.7
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

// WithContentType sets the content type for text generation.
// This determines the format of the generated content.
func WithContentType(contentType gollem.ContentType) Option {
	return func(c *Client) {
		c.contentType = contentType
	}
}

// New creates a new client for the OpenAI API.
// It requires an API key and can be configured with additional options.
func New(ctx context.Context, apiKey string, options ...Option) (*Client, error) {
	client := &Client{
		defaultModel:   DefaultModel,
		embeddingModel: DefaultEmbeddingModel,
		params:         generationParameters{},
		contentType:    gollem.ContentTypeText,
	}

	for _, option := range options {
		option(client)
	}

	openaiClient := openai.NewClient(apiKey)
	client.client = openaiClient

	return client, nil
}

// Session is a session for the OpenAI chat.
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

	cfg gollem.SessionConfig
}

// NewSession creates a new session for the OpenAI API.
// It converts the provided tools to OpenAI's tool format and initializes a new chat session.
func (c *Client) NewSession(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
	cfg := gollem.NewSessionConfig(options...)

	// Convert gollem.Tool to openai.Tool
	openaiTools := make([]openai.Tool, len(cfg.Tools()))
	for i, tool := range cfg.Tools() {
		openaiTools[i] = convertTool(tool)
	}

	var messages []openai.ChatCompletionMessage
	if c.systemPrompt != "" {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: cfg.SystemPrompt(),
		})
	}
	if cfg.History() != nil {
		history, err := cfg.History().ToOpenAI()
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
		cfg:          cfg,
	}

	return session, nil
}

func (s *Session) History() *gollem.History {
	return gollem.NewHistoryFromOpenAI(s.messages)
}

// convertInputs converts gollem.Input to OpenAI messages
func (s *Session) convertInputs(input ...gollem.Input) error {
	for _, in := range input {
		switch v := in.(type) {
		case gollem.Text:
			s.messages = append(s.messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: string(v),
			})

		case gollem.FunctionResponse:
			data, err := json.Marshal(v.Data)
			if err != nil {
				return goerr.Wrap(err, "failed to marshal function response")
			}
			response := string(data)
			if v.Error != nil {
				response = fmt.Sprintf(`Error message: %+v`, v.Error)
			}
			s.messages = append(s.messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    response,
				ToolCallID: v.ID,
			})
		default:
			return goerr.Wrap(gollem.ErrInvalidParameter, "invalid input")
		}
	}
	return nil
}

// createRequest creates a chat completion request with the current session state
func (s *Session) createRequest(stream bool) openai.ChatCompletionRequest {
	req := openai.ChatCompletionRequest{
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

	// Add content type to the request
	if s.cfg.ContentType() == gollem.ContentTypeJSON {
		req.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		}
	}

	return req
}

// GenerateContent processes the input and generates a response.
// It handles both text messages and function responses.
func (s *Session) GenerateContent(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
	if err := s.convertInputs(input...); err != nil {
		return nil, err
	}

	req := s.createRequest(false)
	resp, err := s.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create chat completion")
	}

	if len(resp.Choices) == 0 {
		return &gollem.Response{}, nil
	}

	response := &gollem.Response{
		Texts:         make([]string, 0),
		FunctionCalls: make([]*gollem.FunctionCall, 0),
	}

	message := resp.Choices[0].Message
	if message.Content != "" {
		response.Texts = append(response.Texts, message.Content)
	}

	if message.ToolCalls != nil {
		for _, toolCall := range message.ToolCalls {
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				return nil, goerr.Wrap(err, "failed to unmarshal tool arguments")
			}

			response.FunctionCalls = append(response.FunctionCalls, &gollem.FunctionCall{
				ID:        toolCall.ID,
				Name:      toolCall.Function.Name,
				Arguments: args,
			})
		}

		// Add a single assistant message with all tool calls
		s.messages = append(s.messages, openai.ChatCompletionMessage{
			Role:      openai.ChatMessageRoleAssistant,
			Content:   message.Content,
			ToolCalls: message.ToolCalls,
		})
	} else if message.Content != "" {
		// Add assistant message only if there are no tool calls
		s.messages = append(s.messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: message.Content,
		})
	}

	return response, nil
}

// GenerateStream processes the input and generates a response stream.
// It handles both text messages and function responses, and returns a channel for streaming responses.
func (s *Session) GenerateStream(ctx context.Context, input ...gollem.Input) (<-chan *gollem.Response, error) {
	if err := s.convertInputs(input...); err != nil {
		return nil, err
	}

	req := s.createRequest(true)
	stream, err := s.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create chat completion stream")
	}

	responseChan := make(chan *gollem.Response)

	go func() {
		defer close(responseChan)
		defer stream.Close()

		var textContent string
		var toolCalls []openai.ToolCall

		// Process streaming chunks
		for {
			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				responseChan <- &gollem.Response{
					Error: goerr.Wrap(err, "failed to receive chat completion stream"),
				}
				return
			}

			if len(resp.Choices) == 0 {
				continue
			}

			choice := resp.Choices[0]
			delta := choice.Delta

			// Handle text content
			if delta.Content != "" {
				textContent += delta.Content
				responseChan <- &gollem.Response{
					Texts: []string{delta.Content},
				}
			}

			// Handle tool calls - accumulate them
			if delta.ToolCalls != nil {
				for _, toolCall := range delta.ToolCalls {
					// Get the index, defaulting to 0 if nil
					index := 0
					if toolCall.Index != nil {
						index = *toolCall.Index
					}

					// Ensure we have enough space in the slice
					for len(toolCalls) <= index {
						toolCalls = append(toolCalls, openai.ToolCall{
							Function: openai.FunctionCall{},
						})
					}

					tc := &toolCalls[index]

					if toolCall.ID != "" {
						tc.ID = toolCall.ID
					}
					if toolCall.Type != "" {
						tc.Type = toolCall.Type
					}
					if toolCall.Function.Name != "" {
						tc.Function.Name = toolCall.Function.Name
					}
					if toolCall.Function.Arguments != "" {
						tc.Function.Arguments += toolCall.Function.Arguments
					}
				}
			}

			// Check if we're done
			if choice.FinishReason == openai.FinishReasonToolCalls {
				break
			}
			if choice.FinishReason == openai.FinishReasonStop {
				break
			}
		}

		// Process accumulated tool calls
		if len(toolCalls) > 0 {
			var functionCalls []*gollem.FunctionCall
			for _, toolCall := range toolCalls {
				if toolCall.ID != "" && toolCall.Function.Name != "" && toolCall.Function.Arguments != "" {
					var args map[string]any
					if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
						responseChan <- &gollem.Response{
							Error: goerr.Wrap(err, "failed to unmarshal function call arguments"),
						}
						return
					}

					functionCalls = append(functionCalls, &gollem.FunctionCall{
						ID:        toolCall.ID,
						Name:      toolCall.Function.Name,
						Arguments: args,
					})
				}
			}

			if len(functionCalls) > 0 {
				responseChan <- &gollem.Response{
					FunctionCalls: functionCalls,
				}
			}

			// Add tool calls to message history
			s.messages = append(s.messages, openai.ChatCompletionMessage{
				Role:      openai.ChatMessageRoleAssistant,
				ToolCalls: toolCalls,
			})
		} else if textContent != "" {
			// Add text content to message history
			s.messages = append(s.messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: textContent,
			})
		}
	}()

	return responseChan, nil
}
