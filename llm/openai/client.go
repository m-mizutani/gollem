package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/internal/schema"
	"github.com/sashabaranov/go-openai"
)

var (
	// openaiPromptScope is the logging scope for OpenAI prompts
	openaiPromptScope = ctxlog.NewScope("openai_prompt", ctxlog.EnabledBy("GOLLEM_LOGGING_OPENAI_PROMPT"))

	// openaiResponseScope is the logging scope for OpenAI responses
	openaiResponseScope = ctxlog.NewScope("openai_response", ctxlog.EnabledBy("GOLLEM_LOGGING_OPENAI_RESPONSE"))
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

	// ReasoningEffort tunes how much reasoning time the model spends ("minimal", "medium", "high").
	ReasoningEffort string

	// Verbosity controls the amount of output tokens generated ("low", "medium", "high").
	Verbosity string
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
	DefaultModel          = "gpt-5"
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

// WithReasoningEffort sets the reasoning_effort parameter for GPT-5 models.
// Supported values (as of 2025-10-04): "minimal", "medium", "high".
func WithReasoningEffort(effort string) Option {
	return func(c *Client) {
		c.params.ReasoningEffort = effort
	}
}

// WithVerbosity sets the verbosity parameter for GPT-5 models.
// Supported values (as of 2025-10-04): "low", "medium", "high".
func WithVerbosity(verbosity string) Option {
	return func(c *Client) {
		c.params.Verbosity = verbosity
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
		params: generationParameters{
			ReasoningEffort: "minimal",
			Verbosity:       "low",
		},
		contentType: gollem.ContentTypeText,
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
	// apiClient is the API client interface for dependency injection.
	apiClient apiClient

	// defaultModel is the model to use for chat completions.
	defaultModel string

	// tools are the available tools for the session.
	tools []openai.Tool

	// currentHistory maintains the gollem.History for middleware access.
	historyMessages []openai.ChatCompletionMessage

	// generation parameters
	params generationParameters

	cfg gollem.SessionConfig

	// strictMode enables OpenAI's strict schema adherence (default: false)
	strictMode bool
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

	// Initialize history from config (convert to OpenAI native format)
	var historyMessages []openai.ChatCompletionMessage
	if cfg.History() != nil {
		var err error
		historyMessages, err = ToMessages(cfg.History())
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert history to OpenAI format")
		}
	}

	session := &Session{
		apiClient:       &realAPIClient{client: c.client},
		defaultModel:    c.defaultModel,
		tools:           openaiTools,
		params:          c.params,
		historyMessages: historyMessages,
		cfg:             cfg,
	}

	return session, nil
}

func (s *Session) History() (*gollem.History, error) {
	return NewHistory(s.historyMessages)
}

// getMessages returns history messages (already in OpenAI format)
func (s *Session) getMessages() ([]openai.ChatCompletionMessage, error) {
	if len(s.historyMessages) == 0 {
		return []openai.ChatCompletionMessage{}, nil
	}
	messages := s.historyMessages

	return messages, nil
}

// updateHistoryWithResponse updates the current history with an assistant response
func (s *Session) updateHistoryWithResponse(assistantMessage openai.ChatCompletionMessage) error {
	// Get current messages and append the assistant response
	currentMessages, err := s.getMessages()
	if err != nil {
		return goerr.Wrap(err, "failed to get current messages")
	}
	allMessages := append(currentMessages, assistantMessage)

	// DEBUG: Debug logging can be enabled here for troubleshooting tool_call_id issues

	// Create new history from all messages
	s.historyMessages = allMessages
	return nil
}

// convertInputs converts gollem.Input to OpenAI messages and updates currentHistory
func (s *Session) convertInputs(input ...gollem.Input) error {
	// Work with a local messages array that we'll integrate into history at the end
	var newMessages []openai.ChatCompletionMessage

	// Accumulate consecutive user content (Text/Image) into a single message
	var userContentParts []openai.ChatMessagePart

	for _, in := range input {
		switch v := in.(type) {
		case gollem.Text:
			userContentParts = append(userContentParts, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeText,
				Text: string(v),
			})

		case gollem.Image:
			// Create image URL in data format for OpenAI
			imageURL := fmt.Sprintf("data:%s;base64,%s", v.MimeType(), v.Base64())
			userContentParts = append(userContentParts, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeImageURL,
				ImageURL: &openai.ChatMessageImageURL{
					URL: imageURL,
				},
			})

		case gollem.FunctionResponse:
			// If we have accumulated user content, create a message for it
			if len(userContentParts) > 0 {
				newMessages = append(newMessages, openai.ChatCompletionMessage{
					Role:         openai.ChatMessageRoleUser,
					MultiContent: userContentParts,
				})
				userContentParts = nil
			}
			data, err := json.Marshal(v.Data)
			if err != nil {
				return goerr.Wrap(err, "failed to marshal function response")
			}
			response := string(data)
			if v.Error != nil {
				response = fmt.Sprintf(`Error message: %+v`, v.Error)
			}

			// DEBUG: Log tool result creation for OpenAI
			// Note: Debug logging can be enabled here for troubleshooting tool_call_id issues

			newMessages = append(newMessages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    response,
				ToolCallID: v.ID,
			})
		default:
			return goerr.Wrap(gollem.ErrInvalidParameter, "invalid input")
		}
	}

	// Create final user message if there's any remaining user content
	if len(userContentParts) > 0 {
		newMessages = append(newMessages, openai.ChatCompletionMessage{
			Role:         openai.ChatMessageRoleUser,
			MultiContent: userContentParts,
		})
	}

	// Update currentHistory with new messages
	if len(newMessages) > 0 {
		// Get current messages and append new ones
		currentMessages, err := s.getMessages()
		if err != nil {
			return goerr.Wrap(err, "failed to get current messages")
		}
		allMessages := append(currentMessages, newMessages...)

		// Update history with all messages
		s.historyMessages = allMessages
	}

	return nil
}

// createRequest creates a chat completion request with the current session state
func (s *Session) createRequest(stream bool) (openai.ChatCompletionRequest, error) {
	messages, err := s.getMessages()
	if err != nil {
		return openai.ChatCompletionRequest{}, goerr.Wrap(err, "failed to get messages for API call")
	}

	req := openai.ChatCompletionRequest{
		Model:            s.defaultModel,
		Messages:         messages,
		Tools:            s.tools,
		Temperature:      s.params.Temperature,
		TopP:             s.params.TopP,
		MaxTokens:        s.params.MaxTokens,
		PresencePenalty:  s.params.PresencePenalty,
		FrequencyPenalty: s.params.FrequencyPenalty,
		Stream:           stream,
	}

	if s.params.ReasoningEffort != "" {
		req.ReasoningEffort = s.params.ReasoningEffort
	}

	if s.params.Verbosity != "" {
		req.Verbosity = s.params.Verbosity
	}

	// Add content type and response schema to the request
	if s.cfg.ContentType() == gollem.ContentTypeJSON {
		if s.cfg.ResponseSchema() != nil {
			// Use structured outputs with schema
			schema, err := convertResponseSchemaToOpenAI(s.cfg.ResponseSchema(), s.strictMode)
			if err != nil {
				return openai.ChatCompletionRequest{}, goerr.Wrap(err, "failed to convert response schema")
			}
			req.ResponseFormat = &openai.ChatCompletionResponseFormat{
				Type:       openai.ChatCompletionResponseFormatTypeJSONSchema,
				JSONSchema: schema,
			}
		} else {
			// Use simple JSON object mode (existing behavior)
			req.ResponseFormat = &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			}
		}
	}

	return req, nil
}

// logPrompt logs the prompt if GOLLEM_LOGGING_OPENAI_PROMPT is enabled
func (s *Session) logPrompt(ctx context.Context) {
	// Log prompts if GOLLEM_LOGGING_OPENAI_PROMPT is set
	logger := ctxlog.From(ctx, openaiPromptScope)
	if !logger.Enabled(ctx, slog.LevelInfo) {
		return
	}

	// Build messages for logging
	currentMessages, err := s.getMessages()
	if err != nil {
		logger.Error("Failed to get messages for logging", "error", err)
		return
	}

	var messages []map[string]string
	for _, msg := range currentMessages {
		messages = append(messages, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}
	logger.Info("OpenAI prompt",
		"system_prompt", s.cfg.SystemPrompt(),
		"messages", messages,
	)
}

// GenerateContent processes the input and generates a response.
// It handles both text messages and function responses.
func (s *Session) GenerateContent(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
	// Build the content request for middleware
	// Create a copy of the current history to avoid middleware side effects
	var historyCopy *gollem.History
	var err error
	if len(s.historyMessages) > 0 {
		historyCopy, err = NewHistory(s.historyMessages)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to create history copy for middleware")
		}
	}

	contentReq := &gollem.ContentRequest{
		Inputs:  input,
		History: historyCopy,
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

		// Convert inputs and perform the actual API call
		if err := s.convertInputs(req.Inputs...); err != nil {
			return nil, err
		}

		openaiReq, err := s.createRequest(false)
		if err != nil {
			return nil, err
		}
		s.logPrompt(ctx)

		resp, err := s.apiClient.CreateChatCompletion(ctx, openaiReq)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to create chat completion")
		}

		if len(resp.Choices) == 0 {
			return &gollem.ContentResponse{
				Texts:         []string{},
				FunctionCalls: []*gollem.FunctionCall{},
				InputToken:    0,
				OutputToken:   0,
			}, nil
		}

		response := &gollem.Response{
			Texts:         make([]string, 0),
			FunctionCalls: make([]*gollem.FunctionCall, 0),
			InputToken:    resp.Usage.PromptTokens,
			OutputToken:   resp.Usage.CompletionTokens,
		}

		message := resp.Choices[0].Message
		if message.Content != "" {
			response.Texts = append(response.Texts, message.Content)
		}

		if message.ToolCalls != nil {
			for _, toolCall := range message.ToolCalls {
				var args map[string]any
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					return nil, goerr.Wrap(err, "failed to unmarshal tool arguments")
				}

				response.FunctionCalls = append(response.FunctionCalls, &gollem.FunctionCall{
					ID:        toolCall.ID,
					Name:      toolCall.Function.Name,
					Arguments: args,
				})
			}

			// Create assistant message with all tool calls
			assistantMessage := openai.ChatCompletionMessage{
				Role:      openai.ChatMessageRoleAssistant,
				Content:   message.Content,
				ToolCalls: message.ToolCalls,
			}

			// Update history with assistant response
			if err := s.updateHistoryWithResponse(assistantMessage); err != nil {
				return nil, goerr.Wrap(err, "failed to update history with assistant response")
			}
		} else if message.Content != "" {
			// Create assistant message without tool calls
			assistantMessage := openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: message.Content,
			}

			// Update history with assistant response
			if err := s.updateHistoryWithResponse(assistantMessage); err != nil {
				return nil, goerr.Wrap(err, "failed to update history with assistant response")
			}
		}

		// Log responses if GOLLEM_LOGGING_OPENAI_RESPONSE is set
		responseLogger := ctxlog.From(ctx, openaiResponseScope)
		if responseLogger.Enabled(ctx, slog.LevelInfo) {
			var logContent []map[string]any
			if message.Content != "" {
				logContent = append(logContent, map[string]any{
					"type": "text",
					"text": message.Content,
				})
			}
			for _, toolCall := range message.ToolCalls {
				logContent = append(logContent, map[string]any{
					"type":      "tool_use",
					"id":        toolCall.ID,
					"name":      toolCall.Function.Name,
					"arguments": toolCall.Function.Arguments,
				})
			}
			responseLogger.Info("OpenAI response",
				"model", resp.Model,
				"finish_reason", resp.Choices[0].FinishReason,
				"usage", map[string]any{
					"prompt_tokens":     resp.Usage.PromptTokens,
					"completion_tokens": resp.Usage.CompletionTokens,
					"total_tokens":      resp.Usage.TotalTokens,
				},
				"content", logContent,
			)
		}

		// History is already updated by updateHistoryWithResponse above

		return &gollem.ContentResponse{
			Texts:         response.Texts,
			FunctionCalls: response.FunctionCalls,
			InputToken:    response.InputToken,
			OutputToken:   response.OutputToken,
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

	// Update history after middleware execution (history was already updated in baseHandler)
	// Convert ContentResponse back to gollem.Response
	return &gollem.Response{
		Texts:         contentResp.Texts,
		FunctionCalls: contentResp.FunctionCalls,
		InputToken:    contentResp.InputToken,
		OutputToken:   contentResp.OutputToken,
	}, nil
}

// GenerateStream processes the input and generates a response stream.
// It handles both text messages and function responses, and returns a channel for streaming responses.
func (s *Session) GenerateStream(ctx context.Context, input ...gollem.Input) (<-chan *gollem.Response, error) {
	// Build the content request for middleware
	var historyCopy *gollem.History
	var err error
	if len(s.historyMessages) > 0 {
		historyCopy, err = NewHistory(s.historyMessages)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to create history copy for middleware")
		}
	}

	contentReq := &gollem.ContentRequest{
		Inputs:  input,
		History: historyCopy,
	}

	// Create the base handler that performs the actual API call
	baseHandler := func(ctx context.Context, req *gollem.ContentRequest) (<-chan *gollem.ContentResponse, error) {
		// Always update history from middleware (even if same address, content may have changed)
		if req.History != nil {
			var err error
			s.historyMessages, err = ToMessages(req.History)
			if err != nil {
				return nil, goerr.Wrap(err, "failed to convert history from middleware")
			}
		}

		// Convert inputs and perform the actual API call
		if err := s.convertInputs(req.Inputs...); err != nil {
			return nil, err
		}

		openaiReq, err := s.createRequest(true)
		if err != nil {
			return nil, err
		}
		s.logPrompt(ctx)

		// Enable stream options to get usage data
		openaiReq.StreamOptions = &openai.StreamOptions{
			IncludeUsage: true,
		}
		stream, err := s.apiClient.CreateChatCompletionStream(ctx, openaiReq)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to create chat completion stream")
		}

		responseChan := make(chan *gollem.ContentResponse)

		go func() {
			defer close(responseChan)
			defer stream.Close()

			var textContent string
			var toolCalls []openai.ToolCall
			var totalInputTokens int
			var totalOutputTokens int

			// Process streaming chunks
			for {
				select {
				case <-ctx.Done():
					responseChan <- &gollem.ContentResponse{
						Error: goerr.Wrap(ctx.Err(), "context cancelled during streaming"),
					}
					return
				default:
				}

				resp, err := stream.Recv()
				if err != nil {
					if err == io.EOF {
						break
					}
					responseChan <- &gollem.ContentResponse{
						Error: goerr.Wrap(err, "failed to receive chat completion stream"),
					}
					return
				}

				// Handle token usage if available (comes in final chunk)
				if resp.Usage != nil {
					totalInputTokens = resp.Usage.PromptTokens
					totalOutputTokens = resp.Usage.CompletionTokens
				}

				if len(resp.Choices) == 0 {
					continue
				}

				choice := resp.Choices[0]
				delta := choice.Delta

				// Handle text content
				if delta.Content != "" {
					textContent += delta.Content
					responseChan <- &gollem.ContentResponse{
						Texts:       []string{delta.Content},
						InputToken:  totalInputTokens,
						OutputToken: totalOutputTokens,
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
							responseChan <- &gollem.ContentResponse{
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
					responseChan <- &gollem.ContentResponse{
						FunctionCalls: functionCalls,
						InputToken:    totalInputTokens,
						OutputToken:   totalOutputTokens,
					}
				}

				// Create assistant message with tool calls
				assistantMessage := openai.ChatCompletionMessage{
					Role:      openai.ChatMessageRoleAssistant,
					ToolCalls: toolCalls,
				}
				// Update history with assistant response
				if err := s.updateHistoryWithResponse(assistantMessage); err != nil {
					responseChan <- &gollem.ContentResponse{
						Error: goerr.Wrap(err, "failed to update history with assistant response"),
					}
					return
				}
			} else if textContent != "" {
				// Create assistant message with text content
				assistantMessage := openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleAssistant,
					Content: textContent,
				}
				// Update history with assistant response
				if err := s.updateHistoryWithResponse(assistantMessage); err != nil {
					responseChan <- &gollem.ContentResponse{
						Error: goerr.Wrap(err, "failed to update history with assistant response"),
					}
					return
				}
			}

			// Log streaming response if GOLLEM_LOGGING_OPENAI_RESPONSE is set
			responseLogger := ctxlog.From(ctx, openaiResponseScope)
			var logContent []map[string]any
			if textContent != "" {
				logContent = append(logContent, map[string]any{
					"type": "text",
					"text": textContent,
				})
			}
			for _, toolCall := range toolCalls {
				logContent = append(logContent, map[string]any{
					"type":      "tool_use",
					"id":        toolCall.ID,
					"name":      toolCall.Function.Name,
					"arguments": toolCall.Function.Arguments,
				})
			}
			responseLogger.Info("OpenAI streaming response",
				"usage", map[string]any{
					"prompt_tokens":     totalInputTokens,
					"completion_tokens": totalOutputTokens,
				},
				"content", logContent,
			)

			// Send final response with complete token usage if available
			if totalInputTokens > 0 || totalOutputTokens > 0 {
				responseChan <- &gollem.ContentResponse{
					InputToken:  totalInputTokens,
					OutputToken: totalOutputTokens,
				}
			}

			// History is already updated by updateHistoryWithResponse above
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

	// Sanity check: streamChan should not be nil if err is nil
	if streamChan == nil {
		return nil, goerr.New("middleware returned nil channel without error")
	}

	// Convert ContentStreamResponse channel to Response channel
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

// convertResponseSchemaToOpenAI converts gollem.ResponseSchema to OpenAI's JSONSchemaParams
func convertResponseSchemaToOpenAI(rs *gollem.ResponseSchema, strict bool) (*openai.ChatCompletionResponseFormatJSONSchema, error) {
	if rs == nil || rs.Schema == nil {
		return nil, nil
	}

	// Validate schema
	if err := rs.Schema.Validate(); err != nil {
		return nil, goerr.Wrap(err, "invalid response schema")
	}

	// Convert Parameter to JSON Schema format
	// If strict mode is enabled, we need to adjust the schema to make all properties required
	// This is a limitation of OpenAI's strict mode implementation
	schemaObj := convertParameterToJSONSchemaWithStrict(rs.Schema, strict)

	// Marshal to JSON for OpenAI API
	schemaJSON, err := json.Marshal(schemaObj)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to marshal schema")
	}

	name := rs.Name
	if name == "" {
		name = "response"
	}

	result := &openai.ChatCompletionResponseFormatJSONSchema{
		Name:        name,
		Description: rs.Description,
		Schema:      json.RawMessage(schemaJSON),
		Strict:      strict,
	}

	return result, nil
}

// convertParameterToJSONSchemaWithStrict converts gollem.Parameter to JSON Schema map
// with optional strict mode handling for OpenAI
func convertParameterToJSONSchemaWithStrict(param *gollem.Parameter, strict bool) map[string]any {
	// For non-strict mode, use the shared conversion function
	if !strict {
		return schema.ConvertParameterToJSONSchema(param)
	}

	// Strict mode: OpenAI-specific handling
	// In strict mode, all properties must be in the required array
	result := map[string]any{
		"type": string(param.Type),
	}

	if param.Description != "" {
		result["description"] = param.Description
	}

	if param.Type == gollem.TypeObject && param.Properties != nil {
		props := make(map[string]any)
		for name, prop := range param.Properties {
			props[name] = convertParameterToJSONSchemaWithStrict(prop, strict)
		}
		result["properties"] = props
		result["additionalProperties"] = false

		// In strict mode, OpenAI requires all properties to be in the required array
		// This is a limitation of OpenAI's strict mode, not a general JSON Schema requirement
		allKeys := make([]string, 0, len(param.Properties))
		for key := range param.Properties {
			allKeys = append(allKeys, key)
		}
		result["required"] = allKeys
	}

	if param.Type == gollem.TypeArray && param.Items != nil {
		result["items"] = convertParameterToJSONSchemaWithStrict(param.Items, strict)
	}

	if param.Enum != nil {
		result["enum"] = param.Enum
	}

	// Add constraints
	if param.Minimum != nil {
		result["minimum"] = *param.Minimum
	}
	if param.Maximum != nil {
		result["maximum"] = *param.Maximum
	}
	if param.MinLength != nil {
		result["minLength"] = *param.MinLength
	}
	if param.MaxLength != nil {
		result["maxLength"] = *param.MaxLength
	}
	if param.Pattern != "" {
		result["pattern"] = param.Pattern
	}
	if param.MinItems != nil {
		result["minItems"] = *param.MinItems
	}
	if param.MaxItems != nil {
		result["maxItems"] = *param.MaxItems
	}

	return result
}
