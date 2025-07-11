package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/anthropics/anthropic-sdk-go/vertex"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
)

const (
	// Default Claude models available via Vertex AI using Anthropic SDK
	DefaultVertexClaudeModel = "claude-sonnet-4@20250514"
)

// VertexClient is a client for Claude models via Vertex AI using official Anthropic SDK.
type VertexClient struct {
	// client is the underlying Anthropic client configured for Vertex AI.
	client *anthropic.Client

	// defaultModel is the model to use for chat completions.
	defaultModel string

	// embeddingModel is the model to use for embeddings.
	embeddingModel string

	// generation parameters
	params generationParameters

	// systemPrompt is the system prompt to use for chat completions.
	systemPrompt string
}

// VertexOption is a function that configures a VertexClient.
type VertexOption func(*VertexClient)

// WithVertexModel sets the default model to use for chat completions.
func WithVertexModel(modelName string) VertexOption {
	return func(c *VertexClient) {
		c.defaultModel = modelName
	}
}

// WithVertexEmbeddingModel sets the embedding model to use for embeddings.
func WithVertexEmbeddingModel(modelName string) VertexOption {
	return func(c *VertexClient) {
		c.embeddingModel = modelName
	}
}

// WithVertexTemperature sets the temperature parameter for text generation.
func WithVertexTemperature(temp float64) VertexOption {
	return func(c *VertexClient) {
		c.params.Temperature = temp
	}
}

// WithVertexTopP sets the top_p parameter for text generation.
func WithVertexTopP(topP float64) VertexOption {
	return func(c *VertexClient) {
		c.params.TopP = topP
	}
}

// WithVertexMaxTokens sets the maximum number of tokens to generate.
func WithVertexMaxTokens(maxTokens int64) VertexOption {
	return func(c *VertexClient) {
		c.params.MaxTokens = maxTokens
	}
}

// WithVertexSystemPrompt sets the system prompt for the client.
func WithVertexSystemPrompt(prompt string) VertexOption {
	return func(c *VertexClient) {
		c.systemPrompt = prompt
	}
}

// NewWithVertex creates a new client for Claude models via Vertex AI using Anthropic's official SDK.
// This is the recommended approach as it uses Anthropic's native Vertex AI integration.
func NewWithVertex(ctx context.Context, region, projectID string, options ...VertexOption) (*VertexClient, error) {
	if region == "" {
		return nil, goerr.New("region is required")
	}
	if projectID == "" {
		return nil, goerr.New("projectID is required")
	}

	client := &VertexClient{
		defaultModel:   DefaultVertexClaudeModel,
		embeddingModel: DefaultEmbeddingModel,
		params: generationParameters{
			Temperature: 0.7,
			TopP:        1.0,
			MaxTokens:   4096,
		},
	}

	for _, opt := range options {
		opt(client)
	}

	// Create Anthropic client with Vertex AI integration
	anthropicClient := anthropic.NewClient(
		option.WithAPIKey("dummy"), // Not used for Vertex AI
		vertex.WithGoogleAuth(ctx, region, projectID),
	)

	client.client = &anthropicClient

	return client, nil
}

// VertexAnthropicSession is a session for Claude via Vertex AI using Anthropic SDK.
type VertexAnthropicSession struct {
	client       *anthropic.Client
	defaultModel string
	params       generationParameters
	cfg          gollem.SessionConfig
	messages     []anthropic.MessageParam
}

// NewSession creates a new session for Claude via Vertex AI using Anthropic SDK.
func (c *VertexClient) NewSession(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
	cfg := gollem.NewSessionConfig(options...)

	var messages []anthropic.MessageParam
	if cfg.History() != nil {
		history, err := cfg.History().ToClaude()
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert history to anthropic.MessageParam")
		}
		messages = append(messages, history...)
	}

	session := &VertexAnthropicSession{
		client:       c.client,
		defaultModel: c.defaultModel,
		params:       c.params,
		cfg:          cfg,
		messages:     messages,
	}

	return session, nil
}

// History returns the conversation history
func (s *VertexAnthropicSession) History() *gollem.History {
	return gollem.NewHistoryFromClaude(s.messages)
}

// convertInputs converts gollem.Input to Claude messages and tool results
func (s *VertexAnthropicSession) convertInputs(ctx context.Context, input ...gollem.Input) ([]anthropic.MessageParam, []anthropic.ContentBlockParamUnion, error) {
	logger := gollem.LoggerFromContext(ctx)
	var toolResults []anthropic.ContentBlockParamUnion
	var messages []anthropic.MessageParam

	for _, in := range input {
		switch v := in.(type) {
		case gollem.Text:
			messages = append(messages, anthropic.NewUserMessage(
				anthropic.NewTextBlock(string(v)),
			))

		case gollem.FunctionResponse:
			data, err := v.Data, v.Error
			var response string
			isError := err != nil

			if isError {
				response = fmt.Sprintf("Error: %v", err)
			} else {
				jsonData, marshalErr := json.Marshal(data)
				if marshalErr != nil {
					return nil, nil, goerr.Wrap(marshalErr, "failed to marshal function response")
				}
				response = string(jsonData)
			}

			logger.Debug("creating tool_result",
				"tool_use_id", v.ID,
				"tool_name", v.Name,
				"is_error", isError,
				"response_length", len(response))

			// Create tool result block with new API
			toolResult := anthropic.NewToolResultBlock(v.ID)
			
			// Set content
			if response != "" {
				toolResult.OfToolResult.Content = []anthropic.ToolResultBlockParamContentUnion{
					{OfText: &anthropic.TextBlockParam{Text: response}},
				}
			}
			
			// Set error flag
			if isError {
				toolResult.OfToolResult.IsError = param.NewOpt(true)
			}
			
			toolResults = append(toolResults, toolResult)

		default:
			return nil, nil, goerr.Wrap(gollem.ErrInvalidParameter, "invalid input")
		}
	}

	if len(toolResults) > 0 {
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}

	return messages, toolResults, nil
}

// GenerateContent processes the input and generates a response.
func (s *VertexAnthropicSession) GenerateContent(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
	logger := gollem.LoggerFromContext(ctx)
	messages, _, err := s.convertInputs(ctx, input...)
	if err != nil {
		return nil, err
	}

	s.messages = append(s.messages, messages...)

	// Convert gollem tools to anthropic tools
	var tools []anthropic.ToolUnionParam
	if len(s.cfg.Tools()) > 0 {
		tools = make([]anthropic.ToolUnionParam, len(s.cfg.Tools()))
		for i, tool := range s.cfg.Tools() {
			tools[i] = convertTool(tool)
		}
	}

	// Prepare message parameters
	msgParams := anthropic.MessageNewParams{
		Model:       anthropic.Model(s.defaultModel),
		MaxTokens:   s.params.MaxTokens,
		Temperature: anthropic.Float(s.params.Temperature),
		TopP:        anthropic.Float(s.params.TopP),
		Messages:    s.messages,
	}

	if len(tools) > 0 {
		msgParams.Tools = tools
	}

	// Add system prompt if available
	if systemPrompt := s.createSystemPrompt(); len(systemPrompt) > 0 {
		msgParams.System = systemPrompt
	}

	logger.Debug("Claude Vertex API calling",
		"model", s.defaultModel,
		"message_count", len(s.messages),
		"tools_count", len(tools))

	resp, err := s.client.Messages.New(ctx, msgParams)
	if err != nil {
		logger.Debug("Claude Vertex API request failed", "error", err)
		return nil, goerr.Wrap(err, "failed to create message via Vertex AI")
	}

	logger.Debug("Claude Vertex API response received",
		"content_blocks", len(resp.Content),
		"stop_reason", resp.StopReason)

	// Add assistant's response to message history
	s.messages = append(s.messages, resp.ToParam())

	return processResponse(resp), nil
}

// createSystemPrompt creates system prompt with content type handling
func (s *VertexAnthropicSession) createSystemPrompt() []anthropic.TextBlockParam {
	var systemPrompt []anthropic.TextBlockParam
	if s.cfg.SystemPrompt() != "" {
		systemPrompt = []anthropic.TextBlockParam{
			{Text: s.cfg.SystemPrompt()},
		}
	}

	// Add content type instruction to system prompt
	if s.cfg.ContentType() == gollem.ContentTypeJSON {
		if len(systemPrompt) > 0 {
			systemPrompt[0].Text += "\nPlease format your response as valid JSON."
		} else {
			systemPrompt = []anthropic.TextBlockParam{
				{Text: "Please format your response as valid JSON."},
			}
		}
	}

	return systemPrompt
}

// GenerateStream processes the input and generates a response stream.
func (s *VertexAnthropicSession) GenerateStream(ctx context.Context, input ...gollem.Input) (<-chan *gollem.Response, error) {
	messages, _, err := s.convertInputs(ctx, input...)
	if err != nil {
		return nil, err
	}

	s.messages = append(s.messages, messages...)

	// Convert gollem tools to anthropic tools
	var tools []anthropic.ToolUnionParam
	if len(s.cfg.Tools()) > 0 {
		tools = make([]anthropic.ToolUnionParam, len(s.cfg.Tools()))
		for i, tool := range s.cfg.Tools() {
			tools[i] = convertTool(tool)
		}
	}

	// Prepare message parameters
	msgParams := anthropic.MessageNewParams{
		Model:       anthropic.Model(s.defaultModel),
		MaxTokens:   s.params.MaxTokens,
		Temperature: anthropic.Float(s.params.Temperature),
		TopP:        anthropic.Float(s.params.TopP),
		Messages:    s.messages,
	}

	if len(tools) > 0 {
		msgParams.Tools = tools
	}

	// Add system prompt if available
	if systemPrompt := s.createSystemPrompt(); len(systemPrompt) > 0 {
		msgParams.System = systemPrompt
	}

	stream := s.client.Messages.NewStreaming(ctx, msgParams)
	if stream == nil {
		return nil, goerr.New("failed to create message stream")
	}

	responseChan := make(chan *gollem.Response)

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
			response := &gollem.Response{
				Texts:         make([]string, 0),
				FunctionCalls: make([]*gollem.FunctionCall, 0),
			}

			switch event.Type {
			case "content_block_delta":
				deltaEvent := event.AsContentBlockDelta()
				switch deltaEvent.Delta.Type {
				case "text_delta":
					textDelta := deltaEvent.Delta.AsTextDelta()
					response.Texts = append(response.Texts, textDelta.Text)
					textContent.WriteString(textDelta.Text)
				case "input_json_delta":
					jsonDelta := deltaEvent.Delta.AsInputJSONDelta()
					if jsonDelta.PartialJSON != "" {
						acc.Arguments += jsonDelta.PartialJSON
					}
				}
			case "content_block_start":
				startEvent := event.AsContentBlockStart()
				if startEvent.ContentBlock.Type == "tool_use" {
					toolUseBlock := startEvent.ContentBlock.AsToolUse()
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
					toolCalls = append(toolCalls, anthropic.NewToolUseBlock(funcCall.ID, funcCall.Arguments, funcCall.Name))
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

// GenerateEmbedding generates embeddings for the given input texts.
func (c *VertexClient) GenerateEmbedding(ctx context.Context, dimension int, input []string) ([][]float64, error) {
	return nil, goerr.New("embedding generation not supported for Claude models via Vertex AI")
}
