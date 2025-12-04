package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/internal/schema"
	"github.com/m-mizutani/jsonex"
)

var (
	// claudePromptScope is the logging scope for Claude prompts
	claudePromptScope = ctxlog.NewScope("claude_prompt", ctxlog.EnabledBy("GOLLEM_LOGGING_CLAUDE_PROMPT"))

	// claudeResponseScope is the logging scope for Claude responses
	claudeResponseScope = ctxlog.NewScope("claude_response", ctxlog.EnabledBy("GOLLEM_LOGGING_CLAUDE_RESPONSE"))
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

// setTemperatureAndTopP sets temperature and/or top_p on the request params.
// Claude Sonnet 4.5 does not allow both to be specified simultaneously.
// If both are set, temperature takes priority and a warning is logged.
func setTemperatureAndTopP(ctx context.Context, params *anthropic.MessageNewParams, temperature, topP float64) {
	// Claude Sonnet 4.5 does not allow both temperature and top_p.
	// Set only one, prioritizing temperature if both are set.
	if temperature >= 0 {
		if topP >= 0 {
			logger := ctxlog.From(ctx)
			logger.Warn("Both Temperature and TopP are set for Claude; using Temperature as it is prioritized")
		}
		params.Temperature = anthropic.Float(temperature)
	} else if topP >= 0 {
		params.TopP = anthropic.Float(topP)
	}
}

// Client is a client for the Claude API.
// It provides methods to interact with Anthropic's Claude models.
type Client struct {
	// client is the underlying Claude client.
	client *anthropic.Client

	// defaultModel is the model to use for chat completions.
	// It can be overridden using WithModel option.
	defaultModel string

	// apiKey is the API key for authentication.
	apiKey string

	// baseURL is the custom base URL for the Claude API.
	// If empty, uses the default Anthropic API endpoints.
	baseURL string

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
// Default: 8192
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

// WithBaseURL sets the custom base URL for the Claude API.
// Allows usage with compatible endpoints, proxies, or self-hosted instances.
// If empty, uses the default Anthropic API endpoints.
// Reference: Brain Memory c4705651-435d-4cca-95eb-d39d1ea69a9c
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// New creates a new client for the Claude API.
// It requires an API key and can be configured with additional options.
func New(ctx context.Context, apiKey string, options ...Option) (*Client, error) {
	client := &Client{
		defaultModel: "claude-sonnet-4-5-20250929",
		apiKey:       apiKey,
		baseURL:      "", // Default empty, will be set by options
		params: generationParameters{
			Temperature: -1.0, // -1 indicates not set (0.0 is valid)
			TopP:        -1.0, // -1 indicates not set (0.0 is valid)
			MaxTokens:   8192,
		},
		timeout: 30 * time.Second, // Default timeout
	}

	for _, option := range options {
		option(client)
	}

	clientOptions := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}

	// Add BaseURL if specified
	if client.baseURL != "" {
		clientOptions = append(clientOptions, option.WithBaseURL(client.baseURL))
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
	// apiClient is the API client interface for dependency injection.
	apiClient apiClient

	// defaultModel is the model to use for chat completions.
	defaultModel string

	// tools are the available tools for the session.
	tools []anthropic.ToolUnionParam

	// historyMessages maintains history in Claude native format for efficiency
	historyMessages []anthropic.MessageParam

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

	// Initialize history from config (convert to Claude native format)
	var historyMessages []anthropic.MessageParam
	if cfg.History() != nil {
		var err error
		historyMessages, err = ToMessages(cfg.History())
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert history to Claude format")
		}
	}

	session := &Session{
		apiClient:       &realAPIClient{client: c.client},
		defaultModel:    c.defaultModel,
		tools:           claudeTools,
		params:          c.params,
		historyMessages: historyMessages,
		cfg:             cfg,
	}

	return session, nil
}

func (s *Session) History() (*gollem.History, error) {
	return NewHistory(s.historyMessages)
}

func (s *Session) AppendHistory(h *gollem.History) error {
	if h == nil {
		return nil
	}
	messages, err := ToMessages(h)
	if err != nil {
		return goerr.Wrap(err, "failed to convert history to Claude format")
	}
	s.historyMessages = append(s.historyMessages, messages...)
	return nil
}

// convertInputs converts gollem.Input to Claude messages and tool results
func (s *Session) convertInputs(ctx context.Context, input ...gollem.Input) ([]anthropic.MessageParam, []anthropic.ContentBlockParamUnion, error) {
	return convertGollemInputsToClaude(ctx, input...)
}

// contentBlockToString converts a ContentBlockParamUnion to a string representation for logging
func contentBlockToString(content anthropic.ContentBlockParamUnion) string {
	if content.OfText != nil {
		return fmt.Sprintf("text: %s", content.OfText.Text)
	} else if content.OfImage != nil {
		mediaType := ""
		if content.OfImage.Source.OfBase64 != nil {
			mediaType = string(content.OfImage.Source.OfBase64.MediaType)
		}
		return fmt.Sprintf("image: %s", mediaType)
	} else if content.OfToolUse != nil {
		return fmt.Sprintf("tool_use: %s (input: %v)", content.OfToolUse.Name, content.OfToolUse.Input)
	} else if content.OfToolResult != nil {
		return fmt.Sprintf("tool_result: %s", content.OfToolResult.ToolUseID)
	}
	return "unknown"
}

// convertGollemInputsToClaude is a shared helper function that converts gollem.Input to Claude messages and tool results
// This function is used by both the standard Claude client and the Vertex AI Claude client to avoid code duplication.
// IMPORTANT: Multiple consecutive Text and Image inputs are combined into a single user message with multiple content blocks,
// as per the Anthropic API specification for multi-modal messages.
func convertGollemInputsToClaude(ctx context.Context, input ...gollem.Input) ([]anthropic.MessageParam, []anthropic.ContentBlockParamUnion, error) {
	logger := ctxlog.From(ctx)
	var toolResults []anthropic.ContentBlockParamUnion
	var messages []anthropic.MessageParam

	// Accumulate consecutive user content (Text/Image) into a single message
	var userContentBlocks []anthropic.ContentBlockParamUnion

	for _, in := range input {
		switch v := in.(type) {
		case gollem.Text:
			// Skip empty text blocks
			if string(v) == "" {
				continue
			}
			userContentBlocks = append(userContentBlocks, anthropic.NewTextBlock(string(v)))

		case gollem.Image:
			// Create image block for Claude
			imageBlock := anthropic.NewImageBlock(anthropic.Base64ImageSourceParam{
				Type:      "base64",
				MediaType: anthropic.Base64ImageSourceMediaType(v.MimeType()),
				Data:      v.Base64(),
			})
			userContentBlocks = append(userContentBlocks, imageBlock)

		case gollem.FunctionResponse:
			// If we have accumulated user content, create a message for it
			if len(userContentBlocks) > 0 {
				messages = append(messages, anthropic.NewUserMessage(userContentBlocks...))
				userContentBlocks = nil
			}
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
			toolResult := anthropic.NewToolResultBlock(v.ID, response, isError)

			// Set content
			if response != "" {
				toolResult.OfToolResult.Content = []anthropic.ToolResultBlockParamContentUnion{
					{OfText: &anthropic.TextBlockParam{Text: response}},
				}
			}

			// Set error flag
			if isError {
				toolResult.OfToolResult.IsError = anthropic.Bool(true)
			}

			toolResults = append(toolResults, toolResult)

		default:
			return nil, nil, goerr.Wrap(gollem.ErrInvalidParameter, "invalid input")
		}
	}

	// Create final user message if there's any remaining user content
	if len(userContentBlocks) > 0 {
		messages = append(messages, anthropic.NewUserMessage(userContentBlocks...))
	}

	if len(toolResults) > 0 {
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}

	return messages, toolResults, nil
}

// createSystemPrompt creates system prompt with content type handling
// This is a shared helper function used by both standard Claude client and Vertex AI Claude client.
// Returns []anthropic.TextBlockParam as per anthropic-sdk-go v1.5.0 specification.
// This implementation follows the official SDK format: []anthropic.TextBlockParam{{Text: "..."}}
func createSystemPrompt(ctx context.Context, cfg gollem.SessionConfig) ([]anthropic.TextBlockParam, error) {
	var systemPrompt []anthropic.TextBlockParam
	if cfg.SystemPrompt() != "" {
		systemPrompt = []anthropic.TextBlockParam{
			{Text: cfg.SystemPrompt()},
		}
	}

	// Add content type instruction to system prompt
	if cfg.ContentType() == gollem.ContentTypeJSON {
		jsonInstruction := "\nPlease format your response as valid JSON."

		// Add schema information if provided
		if cfg.ResponseSchema() != nil {
			schemaText, err := schema.ConvertParameterToJSONString(cfg.ResponseSchema())
			if err != nil {
				ctxlog.From(ctx).Warn("Failed to convert response schema to JSON string for Claude prompt", "error", err)
				return nil, goerr.Wrap(err, "failed to convert response schema to JSON string")
			}
			if schemaText != "" {
				jsonInstruction += "\n\nYour response must conform to this JSON Schema:\n" + schemaText
			}
		}

		if len(systemPrompt) > 0 {
			systemPrompt[0].Text += jsonInstruction
		} else {
			systemPrompt = []anthropic.TextBlockParam{
				{Text: jsonInstruction},
			}
		}
	}

	return systemPrompt, nil
}

// extractJSON extracts JSON from noisy text using jsonex library
// It handles both JSON objects and arrays, with proper error handling and logging
func extractJSON(ctx context.Context, text string) string {
	var jsonResult any
	if err := jsonex.Unmarshal([]byte(text), &jsonResult); err != nil {
		// Not valid JSON or does not contain JSON, return original text
		return text
	}

	jsonBytes, err := json.Marshal(jsonResult)
	if err != nil {
		// Log the error if marshalling fails after successful unmarshal
		ctxlog.From(ctx).Warn("Failed to re-marshal extracted JSON, returning original text", "error", err)
		return text
	}

	return string(jsonBytes)
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
	logger := ctxlog.From(ctx)

	// Prepare message parameters
	msgParams := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: params.MaxTokens,
		Messages:  messages,
	}

	// Set temperature and/or top_p (mutually exclusive for Claude Sonnet 4.5)
	setTemperatureAndTopP(ctx, &msgParams, params.Temperature, params.TopP)

	if len(tools) > 0 {
		msgParams.Tools = tools
	}

	// Add system prompt if available
	systemPrompt, err := createSystemPrompt(ctx, cfg)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create system prompt")
	}
	if len(systemPrompt) > 0 {
		msgParams.System = systemPrompt
	}

	logger.Debug(apiName+" API calling",
		"model", model,
		"message_count", len(messages),
		"tools_count", len(tools))

	// Log prompts if GOLLEM_LOGGING_CLAUDE_PROMPT is set
	promptLogger := ctxlog.From(ctx, claudePromptScope)
	// Build messages for logging
	var logMessages []map[string]any
	for _, msg := range messages {
		var contents []string
		for _, content := range msg.Content {
			contents = append(contents, contentBlockToString(content))
		}
		logMessages = append(logMessages, map[string]any{
			"role":     msg.Role,
			"contents": contents,
		})
	}
	promptLogger.Info("Claude prompt",
		"system_prompt", cfg.SystemPrompt(),
		"messages", logMessages,
	)

	resp, err := client.Messages.New(ctx, msgParams)
	if err != nil {
		logger.Debug(apiName+" API request failed", "error", err)
		return nil, goerr.Wrap(err, "failed to create message via "+apiName)
	}

	logger.Debug(apiName+" API response received",
		"content_blocks", len(resp.Content),
		"stop_reason", resp.StopReason)

	// Log responses if GOLLEM_LOGGING_CLAUDE_RESPONSE is set
	responseLogger := ctxlog.From(ctx, claudeResponseScope)
	var logContent []map[string]any
	for _, content := range resp.Content {
		switch content.Type {
		case "text":
			logContent = append(logContent, map[string]any{
				"type": "text",
				"text": content.AsText().Text,
			})
		case "tool_use":
			toolUse := content.AsToolUse()
			logContent = append(logContent, map[string]any{
				"type":  "tool_use",
				"name":  toolUse.Name,
				"input": string(toolUse.Input),
			})
		}
	}
	responseLogger.Info("Claude response",
		"model", resp.Model,
		"stop_reason", resp.StopReason,
		"usage", map[string]any{
			"input_tokens":  resp.Usage.InputTokens,
			"output_tokens": resp.Usage.OutputTokens,
		},
		"content", logContent,
	)

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
		Model:     anthropic.Model(model),
		MaxTokens: params.MaxTokens,
		Messages:  messages,
	}

	// Set temperature and/or top_p (mutually exclusive for Claude Sonnet 4.5)
	setTemperatureAndTopP(ctx, &msgParams, params.Temperature, params.TopP)

	if len(tools) > 0 {
		msgParams.Tools = tools
	}

	// Add system prompt if available
	systemPrompt, err := createSystemPrompt(ctx, cfg)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create system prompt")
	}
	if len(systemPrompt) > 0 {
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
	var totalInputTokens int
	var totalOutputTokens int

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
							finalText = extractJSON(ctx, finalText)
						}
						content = append(content, anthropic.NewTextBlock(finalText))
					}
					content = append(content, toolCalls...)
					*messageHistory = append(*messageHistory, anthropic.NewAssistantMessage(content...))

					// Log streaming response if GOLLEM_LOGGING_CLAUDE_RESPONSE is set
					responseLogger := ctxlog.From(ctx, claudeResponseScope)
					var logContent []map[string]any
					if textContent.Len() > 0 {
						finalText := textContent.String()
						if cfg.ContentType() == gollem.ContentTypeJSON {
							finalText = extractJSON(ctx, finalText)
						}
						logContent = append(logContent, map[string]any{
							"type": "text",
							"text": finalText,
						})
					}
					for _, toolCall := range toolCalls {
						if toolCall.OfToolUse != nil {
							logContent = append(logContent, map[string]any{
								"type":  "tool_use",
								"id":    toolCall.OfToolUse.ID,
								"name":  toolCall.OfToolUse.Name,
								"input": toolCall.OfToolUse.Input,
							})
						}
					}
					responseLogger.Info("Claude streaming response",
						"usage", map[string]any{
							"input_tokens":  totalInputTokens,
							"output_tokens": totalOutputTokens,
						},
						"content", logContent,
					)
				}
				return
			}

			event := stream.Current()
			response := &gollem.Response{
				Texts:         make([]string, 0),
				FunctionCalls: make([]*gollem.FunctionCall, 0),
			}

			switch event.Type {
			case "message_delta":
				messageDelta := event.AsMessageDelta()
				if messageDelta.Usage.OutputTokens > 0 {
					totalOutputTokens = int(messageDelta.Usage.OutputTokens)
				}
			case "message_start":
				messageStart := event.AsMessageStart()
				if messageStart.Message.Usage.InputTokens > 0 {
					totalInputTokens = int(messageStart.Message.Usage.InputTokens)
				}
				if messageStart.Message.Usage.OutputTokens > 0 {
					totalOutputTokens = int(messageStart.Message.Usage.OutputTokens)
				}
			case "content_block_delta":
				deltaEvent := event.AsContentBlockDelta()
				switch deltaEvent.Delta.Type {
				case "text_delta":
					textDelta := deltaEvent.Delta.AsTextDelta()
					response.Texts = append(response.Texts, textDelta.Text)
					response.InputToken = totalInputTokens
					response.OutputToken = totalOutputTokens
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
					response.InputToken = totalInputTokens
					response.OutputToken = totalOutputTokens
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

// processResponseWithContentType converts Claude response to gollem.Response with content type handling
func processResponseWithContentType(ctx context.Context, resp *anthropic.Message, contentType gollem.ContentType, hasResponseSchema bool) *gollem.Response {
	if len(resp.Content) == 0 {
		return &gollem.Response{}
	}

	response := &gollem.Response{
		Texts:         make([]string, 0),
		FunctionCalls: make([]*gollem.FunctionCall, 0),
		InputToken:    int(resp.Usage.InputTokens),
		OutputToken:   int(resp.Usage.OutputTokens),
	}

	for _, content := range resp.Content {
		switch content.Type {
		case "text":
			textBlock := content.AsText()
			text := textBlock.Text

			// Apply JSON extraction for Claude when ContentTypeJSON is specified
			// Even with ResponseSchema and prefill, Claude may still wrap JSON in markdown code blocks
			if contentType == gollem.ContentTypeJSON {
				text = extractJSON(ctx, text)
			}

			response.Texts = append(response.Texts, text)
		case "tool_use":
			toolUseBlock := content.AsToolUse()
			var args map[string]any
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
	// Build the content request for middleware
	// Create a copy of the current history to avoid middleware side effects
	var historyCopy *gollem.History
	if len(s.historyMessages) > 0 {
		var err error
		historyCopy, err = NewHistory(s.historyMessages)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert history from Claude format")
		}
	}

	contentReq := &gollem.ContentRequest{
		Inputs:       input,
		History:      historyCopy,
		SystemPrompt: s.cfg.SystemPrompt(),
	}

	// Create the base handler that performs the actual API call
	baseHandler := func(ctx context.Context, req *gollem.ContentRequest) (*gollem.ContentResponse, error) {
		// Always update history from middleware (even if same address, content may have changed)
		if req.History != nil {
			var err error
			s.historyMessages, err = ToMessages(req.History)
			if err != nil {
				return nil, goerr.Wrap(err, "failed to convert history from middleware")
			}
		}

		messages, _, err := s.convertInputs(ctx, req.Inputs...)
		if err != nil {
			return nil, err
		}

		// Use history messages directly (already in Claude format)
		apiMessages := make([]anthropic.MessageParam, 0, len(s.historyMessages)+len(messages))
		apiMessages = append(apiMessages, s.historyMessages...)
		apiMessages = append(apiMessages, messages...)

		// Create the request and call the API
		systemPrompt, err := createSystemPrompt(ctx, s.cfg)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to create system prompt")
		}
		request := anthropic.MessageNewParams{
			Model:     anthropic.Model(s.defaultModel),
			Messages:  apiMessages,
			MaxTokens: s.params.MaxTokens,
		}

		// Set temperature and/or top_p (mutually exclusive for Claude Sonnet 4.5)
		setTemperatureAndTopP(ctx, &request, s.params.Temperature, s.params.TopP)

		if len(systemPrompt) > 0 {
			request.System = systemPrompt
		}

		if len(s.tools) > 0 {
			request.Tools = s.tools
		}

		// Log prompts if GOLLEM_LOGGING_CLAUDE_PROMPT is set
		promptLogger := ctxlog.From(ctx, claudePromptScope)
		if promptLogger.Enabled(ctx, slog.LevelInfo) {
			promptLogger.Info("Claude prompt",
				"system_prompt", systemPrompt,
				"messages", apiMessages,
			)
		}

		resp, err := s.apiClient.MessagesNew(ctx, request)
		if err != nil {
			opts := tokenLimitErrorOptions(err)
			return nil, goerr.Wrap(err, "failed to create message", opts...)
		}

		// Log response if GOLLEM_LOGGING_CLAUDE_RESPONSE is set
		responseLogger := ctxlog.From(ctx, claudeResponseScope)
		if responseLogger.Enabled(ctx, slog.LevelInfo) {
			var logContent []map[string]any
			for _, content := range resp.Content {
				if content.Type == "text" {
					logContent = append(logContent, map[string]any{
						"type": "text",
						"text": content.Text,
					})
				} else if content.Type == "tool_use" {
					logContent = append(logContent, map[string]any{
						"type":  "tool_use",
						"id":    content.ID,
						"name":  content.Name,
						"input": content.Input,
					})
				}
			}
			responseLogger.Info("Claude response",
				"model", resp.Model,
				"stop_reason", resp.StopReason,
				"usage", map[string]any{
					"input_tokens":  resp.Usage.InputTokens,
					"output_tokens": resp.Usage.OutputTokens,
				},
				"content", logContent,
			)
		}

		// Process response and extract content
		processedResp := processResponseWithContentType(ctx, resp, s.cfg.ContentType(), s.cfg.ResponseSchema() != nil)

		// Update history with new messages (already in Claude format)
		s.historyMessages = append(s.historyMessages, messages...)

		// Only add response to history if it has content
		respParam := resp.ToParam()
		if len(respParam.Content) > 0 {
			s.historyMessages = append(s.historyMessages, respParam)
		}

		return &gollem.ContentResponse{
			Texts:         processedResp.Texts,
			FunctionCalls: processedResp.FunctionCalls,
			InputToken:    processedResp.InputToken,
			OutputToken:   processedResp.OutputToken,
		}, nil
	}

	// Build middleware chain
	handler := gollem.ContentBlockHandler(baseHandler)
	for i := len(s.cfg.ContentBlockMiddlewares()) - 1; i >= 0; i-- {
		handler = s.cfg.ContentBlockMiddlewares()[i](handler)
	}

	// Execute middleware chain
	contentResp, err := handler(ctx, contentReq)
	if err != nil {
		return nil, err
	}

	// Convert ContentResponse back to gollem.Response
	return &gollem.Response{
		Texts:         contentResp.Texts,
		FunctionCalls: contentResp.FunctionCalls,
		InputToken:    contentResp.InputToken,
		OutputToken:   contentResp.OutputToken,
	}, nil
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
	// Build the content request for middleware
	// Create a copy of the current history to avoid middleware side effects
	var historyCopy *gollem.History
	if len(s.historyMessages) > 0 {
		var err error
		historyCopy, err = NewHistory(s.historyMessages)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert history from Claude format")
		}
	}

	contentReq := &gollem.ContentRequest{
		Inputs:       input,
		History:      historyCopy,
		SystemPrompt: s.cfg.SystemPrompt(),
	}

	// Create the base handler that performs the actual API call
	baseHandler := func(ctx context.Context, req *gollem.ContentRequest) (<-chan *gollem.ContentResponse, error) {
		// Update history if modified by middleware
		if req.History != nil {
			var err error
			s.historyMessages, err = ToMessages(req.History)
			if err != nil {
				return nil, goerr.Wrap(err, "failed to convert history from middleware")
			}
		}

		messages, _, err := s.convertInputs(ctx, req.Inputs...)
		if err != nil {
			return nil, err
		}

		// Use history messages directly (already in Claude format) and append new inputs
		allMessages := make([]anthropic.MessageParam, 0, len(s.historyMessages)+len(messages))
		allMessages = append(allMessages, s.historyMessages...)
		allMessages = append(allMessages, messages...)

		// Create request params
		systemPrompt, err := createSystemPrompt(ctx, s.cfg)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to create system prompt")
		}
		request := anthropic.MessageNewParams{
			Model:     anthropic.Model(s.defaultModel),
			Messages:  allMessages,
			MaxTokens: s.params.MaxTokens,
		}

		// Set temperature and/or top_p (mutually exclusive for Claude Sonnet 4.5)
		setTemperatureAndTopP(ctx, &request, s.params.Temperature, s.params.TopP)

		if len(systemPrompt) > 0 {
			request.System = systemPrompt
		}

		if len(s.tools) > 0 {
			request.Tools = s.tools
		}

		// Simplified streaming implementation - full implementation would be complex
		// For now, we'll use non-streaming API and simulate streaming
		resp, err := s.apiClient.MessagesNew(ctx, request)
		if err != nil {
			opts := tokenLimitErrorOptions(err)
			return nil, goerr.Wrap(err, "failed to create message stream", opts...)
		}

		responseChan := make(chan *gollem.ContentResponse)

		go func() {
			defer close(responseChan)

			// Process response and send chunks
			for _, content := range resp.Content {
				if content.Type == "text" {
					textBlock := content.AsText()
					responseChan <- &gollem.ContentResponse{
						Texts:       []string{textBlock.Text},
						InputToken:  int(resp.Usage.InputTokens),
						OutputToken: int(resp.Usage.OutputTokens),
					}
				}
			}

			// Update history after successful streaming (already in Claude format)
			s.historyMessages = append(s.historyMessages, messages...)

			// Only add response to history if it has content
			respParam := resp.ToParam()
			if len(respParam.Content) > 0 {
				s.historyMessages = append(s.historyMessages, respParam)
			}
		}()

		return responseChan, nil
	}

	// Build middleware chain
	handler := gollem.ContentStreamHandler(baseHandler)
	for i := len(s.cfg.ContentStreamMiddlewares()) - 1; i >= 0; i-- {
		handler = s.cfg.ContentStreamMiddlewares()[i](handler)
	}

	// Execute middleware chain
	streamChan, err := handler(ctx, contentReq)
	if err != nil {
		return nil, err
	}

	// Convert ContentResponse channel to Response channel
	responseChan := make(chan *gollem.Response)
	go func() {
		defer close(responseChan)
		for streamResp := range streamChan {
			if streamResp.Error != nil {
				responseChan <- &gollem.Response{
					Error: streamResp.Error,
				}
			} else {
				responseChan <- &gollem.Response{
					Texts:         streamResp.Texts,
					FunctionCalls: streamResp.FunctionCalls,
					InputToken:    streamResp.InputToken,
					OutputToken:   streamResp.OutputToken,
				}
			}
		}
	}()

	return responseChan, nil
}

// countTokensWithParams is a helper function that builds the count tokens parameters
// and calls the API.
func countTokensWithParams(
	ctx context.Context,
	model string,
	historyMessages []anthropic.MessageParam,
	newMessages []anthropic.MessageParam,
	systemPrompt string,
	tools []anthropic.ToolUnionParam,
	apiClient apiClient,
) (int, error) {
	// Build complete messages list: history + new inputs
	apiMessages := make([]anthropic.MessageParam, 0, len(historyMessages)+len(newMessages))
	apiMessages = append(apiMessages, historyMessages...)
	apiMessages = append(apiMessages, newMessages...)

	// Prepare count tokens parameters
	params := anthropic.MessageCountTokensParams{
		Model:    anthropic.Model(model),
		Messages: apiMessages,
	}

	// Add system prompt if available
	if systemPrompt != "" {
		params.System = anthropic.MessageCountTokensParamsSystemUnion{
			OfString: anthropic.String(systemPrompt),
		}
	}

	// Add tools if available
	if len(tools) > 0 {
		// Convert ToolUnionParam to MessageCountTokensToolUnionParam
		countTools := make([]anthropic.MessageCountTokensToolUnionParam, 0, len(tools))
		for _, tool := range tools {
			if tool.OfTool != nil {
				countTools = append(countTools, anthropic.MessageCountTokensToolUnionParam{
					OfTool: tool.OfTool,
				})
			}
		}
		params.Tools = countTools
	}

	// Call the CountTokens API
	result, err := apiClient.MessagesCountTokens(ctx, params)
	if err != nil {
		return 0, goerr.Wrap(err, "failed to count tokens")
	}

	return int(result.InputTokens), nil
}

// CountToken calculates the total number of tokens for the given inputs,
// including system prompt, history messages, and new inputs.
// This uses Anthropic's Messages Count Tokens API.
func (s *Session) CountToken(ctx context.Context, input ...gollem.Input) (int, error) {
	// Convert inputs to Claude messages
	messages, _, err := s.convertInputs(ctx, input...)
	if err != nil {
		return 0, goerr.Wrap(err, "failed to convert inputs for token counting")
	}

	// Create copies of historyMessages and tools to avoid race conditions
	// This ensures thread safety when reading session state
	historyMessagesCopy := make([]anthropic.MessageParam, len(s.historyMessages))
	copy(historyMessagesCopy, s.historyMessages)

	toolsCopy := make([]anthropic.ToolUnionParam, len(s.tools))
	copy(toolsCopy, s.tools)

	return countTokensWithParams(
		ctx,
		s.defaultModel,
		historyMessagesCopy,
		messages,
		s.cfg.SystemPrompt(),
		toolsCopy,
		s.apiClient,
	)
}

// tokenLimitErrorOptions checks if the error is a token limit exceeded error
// and returns goerr.Option to tag the error with ErrTagTokenExceeded.
// Returns nil if the error is not a token limit exceeded error.
//
// Detection logic:
// - Error must be *anthropic.Error
// - StatusCode must be 400 or 413 (Request Entity Too Large)
// - Parse RawJSON() to get error structure
// - error.type must be "invalid_request_error"
// - error.message must contain "prompt is too long" (case-insensitive)
func tokenLimitErrorOptions(err error) []goerr.Option {
	var apiErr *anthropic.Error
	if !errors.As(err, &apiErr) {
		return nil
	}

	if apiErr.StatusCode != 400 && apiErr.StatusCode != 413 {
		return nil
	}

	// Parse RawJSON to get error details
	type claudeErrorWrapper struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	var wrapper claudeErrorWrapper
	if err := json.Unmarshal([]byte(apiErr.RawJSON()), &wrapper); err != nil {
		return nil
	}

	if wrapper.Error.Type != "invalid_request_error" {
		return nil
	}

	// Check for token limit error message (case-insensitive)
	lowerMessage := strings.ToLower(wrapper.Error.Message)
	if strings.Contains(lowerMessage, "prompt is too long") {
		return []goerr.Option{goerr.Tag(gollem.ErrTagTokenExceeded)}
	}

	return nil
}
