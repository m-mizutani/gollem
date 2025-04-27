package claude

import (
	"context"
	"encoding/json"
	"strings"

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

// convertInputs converts gollam.Input to Claude messages and tool results
func (s *Session) convertInputs(input ...gollam.Input) ([]anthropic.MessageParam, []anthropic.ContentBlockParamUnion, error) {
	var toolResults []anthropic.ContentBlockParamUnion
	var messages []anthropic.MessageParam

	for _, in := range input {
		switch v := in.(type) {
		case gollam.Text:
			messages = append(messages, anthropic.NewUserMessage(
				anthropic.NewTextBlock(string(v)),
			))

		case gollam.FunctionResponse:
			response, err := json.Marshal(v.Data)
			if err != nil {
				return nil, nil, goerr.Wrap(err, "failed to marshal function response")
			}
			toolResults = append(toolResults, anthropic.NewToolResultBlock(v.ID, string(response), v.Error != nil))

		default:
			return nil, nil, goerr.Wrap(gollam.ErrInvalidParameter, "invalid input")
		}
	}

	if len(toolResults) > 0 {
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}

	return messages, toolResults, nil
}

// createRequest creates a message request with the current session state
func (s *Session) createRequest(messages []anthropic.MessageParam) anthropic.MessageNewParams {
	return anthropic.MessageNewParams{
		Model:       s.defaultModel,
		MaxTokens:   s.params.MaxTokens,
		Temperature: anthropic.Float(s.params.Temperature),
		TopP:        anthropic.Float(s.params.TopP),
		Tools:       s.tools,
		Messages:    messages,
	}
}

// processResponse converts Claude response to gollam.Response
func processResponse(resp *anthropic.Message) *gollam.Response {
	if len(resp.Content) == 0 {
		return &gollam.Response{}
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
				response.Error = goerr.Wrap(err, "failed to unmarshal function arguments")
				return response
			}

			response.FunctionCalls = append(response.FunctionCalls, &gollam.FunctionCall{
				ID:        toolUseBlock.ID,
				Name:      toolUseBlock.Name,
				Arguments: args,
			})
		}
	}

	return response
}

// GenerateContent processes the input and generates a response.
// It handles both text messages and function responses.
func (s *Session) GenerateContent(ctx context.Context, input ...gollam.Input) (*gollam.Response, error) {
	messages, _, err := s.convertInputs(input...)
	if err != nil {
		return nil, err
	}

	s.messages = append(s.messages, messages...)
	params := s.createRequest(s.messages)

	resp, err := s.client.Messages.New(ctx, params)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create message")
	}

	// Add assistant's response to message history
	s.messages = append(s.messages, resp.ToParam())

	return processResponse(resp), nil
}

// FunctionCallAccumulator accumulates function call information from stream
type FunctionCallAccumulator struct {
	ID        string
	Name      string
	Arguments string
}

func newFunctionCallAccumulator() *FunctionCallAccumulator {
	return &FunctionCallAccumulator{
		Arguments: "",
	}
}

func (a *FunctionCallAccumulator) addFunctionCall(delta *anthropic.ContentBlockDeltaEventDeltaUnion) {
	if delta == nil {
		return
	}

	if delta.Type == "tool_use" {
		textDelta := delta.AsTextContentBlockDelta()
		if textDelta.Text != "" {
			a.Arguments += textDelta.Text
		}
	}
}

func (a *FunctionCallAccumulator) accumulate() (*gollam.FunctionCall, error) {
	if a.ID == "" || a.Name == "" {
		return nil, goerr.Wrap(gollam.ErrInvalidParameter, "function call is not complete")
	}

	var args map[string]any
	if a.Arguments != "" {
		if err := json.Unmarshal([]byte(a.Arguments), &args); err != nil {
			return nil, goerr.Wrap(err, "failed to unmarshal function call arguments", goerr.V("accumulator", a))
		}
	}

	return &gollam.FunctionCall{
		ID:        a.ID,
		Name:      a.Name,
		Arguments: args,
	}, nil
}

// GenerateStream processes the input and generates a response stream.
// It handles both text messages and function responses, and returns a channel for streaming responses.
func (s *Session) GenerateStream(ctx context.Context, input ...gollam.Input) (<-chan *gollam.Response, error) {
	messages, _, err := s.convertInputs(input...)
	if err != nil {
		return nil, err
	}

	s.messages = append(s.messages, messages...)
	params := s.createRequest(s.messages)

	stream := s.client.Messages.NewStreaming(ctx, params)
	if stream == nil {
		return nil, goerr.New("failed to create message stream")
	}

	responseChan := make(chan *gollam.Response)

	// Accumulate text and tool calls for message history
	var textContent strings.Builder
	var toolCalls []anthropic.ContentBlockParamUnion
	acc := newFunctionCallAccumulator()

	go func() {
		defer close(responseChan)

		for {
			if !stream.Next() {
				// Add accumulated message to history when stream ends
				if textContent.Len() > 0 || len(toolCalls) > 0 {
					var content []anthropic.ContentBlockParamUnion
					if textContent.Len() > 0 {
						content = append(content, anthropic.NewTextBlock(textContent.String()))
					}
					content = append(content, toolCalls...)
					s.messages = append(s.messages, anthropic.NewAssistantMessage(content...))
				}
				return
			}

			event := stream.Current()
			response := &gollam.Response{
				Texts:         make([]string, 0),
				FunctionCalls: make([]*gollam.FunctionCall, 0),
			}

			switch event.Type {
			case "content_block_delta":
				deltaEvent := event.AsContentBlockDeltaEvent()
				switch deltaEvent.Delta.Type {
				case "text_delta":
					textDelta := deltaEvent.Delta.AsTextContentBlockDelta()
					response.Texts = append(response.Texts, textDelta.Text)
					textContent.WriteString(textDelta.Text)
				case "input_json_delta":
					jsonDelta := deltaEvent.Delta.AsInputJSONContentBlockDelta()
					if jsonDelta.PartialJSON != "" {
						acc.Arguments += jsonDelta.PartialJSON
					}
				}
			case "content_block_start":
				startEvent := event.AsContentBlockStartEvent()
				if startEvent.ContentBlock.Type == "tool_use" {
					toolUseBlock := startEvent.ContentBlock.AsResponseToolUseBlock()
					acc.ID = toolUseBlock.ID
					acc.Name = toolUseBlock.Name
				}
			case "content_block_stop":
				if acc.ID != "" && acc.Name != "" {
					funcCall, err := acc.accumulate()
					if err != nil {
						response.Error = err
						responseChan <- response
						return
					}
					response.FunctionCalls = append(response.FunctionCalls, funcCall)
					toolCalls = append(toolCalls, anthropic.ContentBlockParamUnion{
						OfRequestToolUseBlock: &anthropic.ToolUseBlockParam{
							ID:    funcCall.ID,
							Name:  funcCall.Name,
							Input: funcCall.Arguments,
							Type:  "tool_use",
						},
					})
					acc = newFunctionCallAccumulator()
				}
			}

			if response.HasData() {
				responseChan <- response
			}
		}
	}()

	return responseChan, nil
}
