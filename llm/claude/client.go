package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
)

const (
	DefaultEmbeddingModel = "claude-3-sonnet-20240229"
)

var (
	// codeBlockRegex is a compiled regular expression for extracting JSON from markdown code blocks
	// This is compiled once at package initialization for performance
	codeBlockRegex = regexp.MustCompile(`(?s)` + "```" + `(?:json)?\n?(.*?)\n?` + "```" + ``)
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

	// embeddingModel is the model to use for embeddings.
	// It can be overridden using WithEmbeddingModel option.
	embeddingModel string

	// apiKey is the API key for authentication.
	apiKey string

	// generation parameters
	params generationParameters

	// systemPrompt is the system prompt to use for chat completions.
	systemPrompt string

	// timeout for API requests
	timeout time.Duration
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

// WithEmbeddingModel sets the embedding model to use for embeddings.
// The model name should be a valid Claude model identifier.
// Default: DefaultEmbeddingModel
func WithEmbeddingModel(modelName string) Option {
	return func(c *Client) {
		c.embeddingModel = modelName
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

// WithTimeout sets the timeout for API requests
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.timeout = timeout
	}
}

// WithSystemPrompt sets the system prompt for the client
func WithSystemPrompt(prompt string) Option {
	return func(c *Client) {
		c.systemPrompt = prompt
	}
}

// New creates a new client for the Claude API.
// It requires an API key and can be configured with additional options.
func New(ctx context.Context, apiKey string, options ...Option) (*Client, error) {
	client := &Client{
		defaultModel:   string(anthropic.ModelClaude3_5SonnetLatest),
		embeddingModel: DefaultEmbeddingModel,
		apiKey:         apiKey,
		params: generationParameters{
			Temperature: 0.7,
			TopP:        1.0,
			MaxTokens:   4096,
		},
		timeout: 30 * time.Second, // Default timeout
	}

	for _, option := range options {
		option(client)
	}

	clientOptions := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}

	// Add timeout if specified
	if client.timeout > 0 {
		httpClient := &http.Client{
			Timeout: client.timeout,
		}
		clientOptions = append(clientOptions, option.WithHTTPClient(httpClient))
	}

	newClient := anthropic.NewClient(clientOptions...)
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

	cfg gollem.SessionConfig
}

// NewSession creates a new session for the Claude API.
// It converts the provided tools to Claude's tool format and initializes a new chat session.
func (c *Client) NewSession(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
	cfg := gollem.NewSessionConfig(options...)

	// Convert gollem.Tool to anthropic.ToolUnionParam
	claudeTools := make([]anthropic.ToolUnionParam, len(cfg.Tools()))
	for i, tool := range cfg.Tools() {
		claudeTools[i] = convertTool(tool)
	}

	var messages []anthropic.MessageParam
	if cfg.History() != nil {
		history, err := cfg.History().ToClaude()
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert history to anthropic.MessageParam")
		}
		messages = append(messages, history...)
	}

	session := &Session{
		client:       c.client,
		defaultModel: c.defaultModel,
		tools:        claudeTools,
		params:       c.params,
		messages:     messages,
		cfg:          cfg,
	}

	return session, nil
}

func (s *Session) History() *gollem.History {
	return gollem.NewHistoryFromClaude(s.messages)
}

// convertInputs converts gollem.Input to Claude messages and tool results
func (s *Session) convertInputs(ctx context.Context, input ...gollem.Input) ([]anthropic.MessageParam, []anthropic.ContentBlockParamUnion, error) {
	return convertGollemInputsToClaude(ctx, input...)
}

// convertGollemInputsToClaude is a shared helper function that converts gollem.Input to Claude messages and tool results
// This function is used by both the standard Claude client and the Vertex AI Claude client to avoid code duplication.
func convertGollemInputsToClaude(ctx context.Context, input ...gollem.Input) ([]anthropic.MessageParam, []anthropic.ContentBlockParamUnion, error) {
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
			// Handle error cases first
			isError := v.Error != nil
			var response string

			if isError {
				response = fmt.Sprintf("Error: %v", v.Error)
			} else {
				data, err := json.Marshal(v.Data)
				if err != nil {
					return nil, nil, goerr.Wrap(err, "failed to marshal function response")
				}
				response = string(data)
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

// createSystemPrompt creates system prompt with content type handling
// This is a shared helper function used by both standard Claude client and Vertex AI Claude client.
func createSystemPrompt(cfg gollem.SessionConfig) []anthropic.TextBlockParam {
	var systemPrompt []anthropic.TextBlockParam
	if cfg.SystemPrompt() != "" {
		systemPrompt = []anthropic.TextBlockParam{
			{Text: cfg.SystemPrompt()},
		}
	}

	// Add content type instruction to system prompt
	if cfg.ContentType() == gollem.ContentTypeJSON {
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

// generateClaudeContent is a shared helper function that handles the core logic for generating content
// This function is used by both the standard Claude client and the Vertex AI Claude client.
func generateClaudeContent(
	ctx context.Context,
	client *anthropic.Client,
	messages []anthropic.MessageParam,
	model string,
	params generationParameters,
	tools []anthropic.ToolUnionParam,
	cfg gollem.SessionConfig,
	apiName string,
) (*anthropic.Message, error) {
	logger := gollem.LoggerFromContext(ctx)

	// Prepare message parameters
	msgParams := anthropic.MessageNewParams{
		Model:       anthropic.Model(model),
		MaxTokens:   params.MaxTokens,
		Temperature: anthropic.Float(params.Temperature),
		TopP:        anthropic.Float(params.TopP),
		Messages:    messages,
	}

	if len(tools) > 0 {
		msgParams.Tools = tools
	}

	// Add system prompt if available
	if systemPrompt := createSystemPrompt(cfg); len(systemPrompt) > 0 {
		msgParams.System = systemPrompt
	}

	logger.Debug(apiName+" API calling",
		"model", model,
		"message_count", len(messages),
		"tools_count", len(tools))

	resp, err := client.Messages.New(ctx, msgParams)
	if err != nil {
		logger.Debug(apiName+" API request failed", "error", err)
		return nil, goerr.Wrap(err, "failed to create message via "+apiName)
	}

	logger.Debug(apiName+" API response received",
		"content_blocks", len(resp.Content),
		"stop_reason", resp.StopReason)

	return resp, nil
}

// generateClaudeStream is a shared helper function that handles the core logic for generating streaming content
// This function is used by both the standard Claude client and the Vertex AI Claude client.
func generateClaudeStream(
	ctx context.Context,
	client *anthropic.Client,
	messages []anthropic.MessageParam,
	model string,
	params generationParameters,
	tools []anthropic.ToolUnionParam,
	cfg gollem.SessionConfig,
	messageHistory *[]anthropic.MessageParam,
) (<-chan *gollem.Response, error) {
	// Prepare message parameters
	msgParams := anthropic.MessageNewParams{
		Model:       anthropic.Model(model),
		MaxTokens:   params.MaxTokens,
		Temperature: anthropic.Float(params.Temperature),
		TopP:        anthropic.Float(params.TopP),
		Messages:    messages,
	}

	if len(tools) > 0 {
		msgParams.Tools = tools
	}

	// Add system prompt if available
	if systemPrompt := createSystemPrompt(cfg); len(systemPrompt) > 0 {
		msgParams.System = systemPrompt
	}

	stream := client.Messages.NewStreaming(ctx, msgParams)
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
						finalText := textContent.String()
						// Apply JSON extraction for Claude when ContentTypeJSON is specified
						if cfg.ContentType() == gollem.ContentTypeJSON {
							finalText = extractJSONFromResponse(finalText)
						}
						content = append(content, anthropic.NewTextBlock(finalText))
					}
					content = append(content, toolCalls...)
					*messageHistory = append(*messageHistory, anthropic.NewAssistantMessage(content...))
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

// extractJSONFromResponse cleans the response text to extract valid JSON
// This is necessary because Claude returns JSON wrapped in markdown code blocks
// even when ContentTypeJSON is specified.
func extractJSONFromResponse(text string) string {
	// Remove leading/trailing whitespace
	text = strings.TrimSpace(text)
	
	// Try to extract JSON from markdown code blocks using pre-compiled regex
	matches := codeBlockRegex.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Try to find JSON object boundaries
	start := strings.Index(text, "{")
	if start == -1 {
		return text // No JSON found, return original
	}

	// Find the matching closing brace
	braceCount := 0
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '{':
			braceCount++
		case '}':
			braceCount--
			if braceCount == 0 {
				return text[start : i+1]
			}
		}
	}

	// If no matching brace found, try from start to end
	return text[start:]
}

// processResponseWithContentType converts Claude response to gollem.Response with content type handling
func processResponseWithContentType(resp *anthropic.Message, contentType gollem.ContentType) *gollem.Response {
	if len(resp.Content) == 0 {
		return &gollem.Response{}
	}

	response := &gollem.Response{
		Texts:         make([]string, 0),
		FunctionCalls: make([]*gollem.FunctionCall, 0),
	}

	for _, content := range resp.Content {
		switch content.Type {
		case "text":
			textBlock := content.AsText()
			text := textBlock.Text

			// Apply JSON extraction for Claude when ContentTypeJSON is specified
			if contentType == gollem.ContentTypeJSON {
				text = extractJSONFromResponse(text)
			}

			response.Texts = append(response.Texts, text)
		case "tool_use":
			toolUseBlock := content.AsToolUse()
			var args map[string]interface{}
			if err := json.Unmarshal(toolUseBlock.Input, &args); err != nil {
				response.Error = goerr.Wrap(err, "failed to unmarshal function arguments")
				return response
			}

			response.FunctionCalls = append(response.FunctionCalls, &gollem.FunctionCall{
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
func (s *Session) GenerateContent(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
	logger := gollem.LoggerFromContext(ctx)
	messages, _, err := s.convertInputs(ctx, input...)
	if err != nil {
		return nil, err
	}

	// DEBUG: Log message history for debugging
	logger.Debug("Claude API request",
		"message_count", len(s.messages),
		"input_count", len(input))

	// Log the last few messages to understand the conversation state
	for i, msg := range s.messages[max(0, len(s.messages)-5):] {
		logger.Debug("Claude message",
			"index", i,
			"role", msg.Role,
			"content_blocks", len(msg.Content))
	}

	s.messages = append(s.messages, messages...)

	resp, err := generateClaudeContent(
		ctx,
		s.client,
		s.messages,
		s.defaultModel,
		s.params,
		s.tools,
		s.cfg,
		"Claude",
	)
	if err != nil {
		return nil, err
	}

	// Add assistant's response to message history
	// This is critical for tool_use/tool_result consistency
	s.messages = append(s.messages, resp.ToParam())

	logger.Debug("Added assistant response to message history",
		"content_blocks", len(resp.Content),
		"total_messages", len(s.messages))

	return processResponseWithContentType(resp, s.cfg.ContentType()), nil
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

func (a *FunctionCallAccumulator) accumulate() (*gollem.FunctionCall, error) {
	if a.ID == "" || a.Name == "" {
		return nil, goerr.Wrap(gollem.ErrInvalidParameter, "function call is not complete")
	}

	var args map[string]any
	if a.Arguments != "" {
		if err := json.Unmarshal([]byte(a.Arguments), &args); err != nil {
			return nil, goerr.Wrap(err, "failed to unmarshal function call arguments", goerr.V("accumulator", a))
		}
	}

	return &gollem.FunctionCall{
		ID:        a.ID,
		Name:      a.Name,
		Arguments: args,
	}, nil
}

// GenerateStream processes the input and generates a response stream.
// It handles both text messages and function responses, and returns a channel for streaming responses.
func (s *Session) GenerateStream(ctx context.Context, input ...gollem.Input) (<-chan *gollem.Response, error) {
	messages, _, err := s.convertInputs(ctx, input...)
	if err != nil {
		return nil, err
	}

	s.messages = append(s.messages, messages...)

	return generateClaudeStream(
		ctx,
		s.client,
		s.messages,
		s.defaultModel,
		s.params,
		s.tools,
		s.cfg,
		&s.messages,
	)
}
