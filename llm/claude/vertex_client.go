package claude

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/vertexai/genai"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"google.golang.org/api/option"
)

const (
	// Default Claude models available in Vertex AI
	DefaultVertexClaudeModel          = "claude-3-5-sonnet@20241022"
	DefaultVertexClaudeEmbeddingModel = "claude-3-sonnet-20240229"
)

// VertexClient is a client for Claude models via Google Vertex AI.
// It provides methods to interact with Anthropic's Claude models through Vertex AI.
type VertexClient struct {
	projectID string
	location  string

	// client is the underlying Vertex AI client.
	client *genai.Client

	// defaultModel is the model to use for chat completions.
	// It can be overridden using WithModel option.
	defaultModel string

	// embeddingModel is the model to use for embeddings.
	// It can be overridden using WithEmbeddingModel option.
	embeddingModel string

	// gcpOptions are additional options for Google Cloud Platform.
	// They can be set using WithGoogleCloudOptions.
	gcpOptions []option.ClientOption

	// generation parameters
	params generationParameters

	// systemPrompt is the system prompt to use for chat completions.
	systemPrompt string

	// contentType is the type of content to be generated.
	contentType gollem.ContentType
}

// VertexOption is a function that configures a VertexClient.
type VertexOption func(*VertexClient)

// WithVertexModel sets the default model to use for chat completions.
// The model name should be a valid Claude model identifier available in Vertex AI.
// Default: DefaultVertexClaudeModel
func WithVertexModel(modelName string) VertexOption {
	return func(c *VertexClient) {
		c.defaultModel = modelName
	}
}

// WithVertexEmbeddingModel sets the embedding model to use for embeddings.
// The model name should be a valid Claude model identifier available in Vertex AI.
// Default: DefaultVertexClaudeEmbeddingModel
func WithVertexEmbeddingModel(modelName string) VertexOption {
	return func(c *VertexClient) {
		c.embeddingModel = modelName
	}
}

// WithVertexTemperature sets the temperature parameter for text generation.
// Higher values make the output more random, lower values make it more focused.
// Range: 0.0 to 1.0
// Default: 0.7
func WithVertexTemperature(temp float64) VertexOption {
	return func(c *VertexClient) {
		c.params.Temperature = temp
	}
}

// WithVertexTopP sets the top_p parameter for text generation.
// Controls diversity via nucleus sampling.
// Range: 0.0 to 1.0
// Default: 1.0
func WithVertexTopP(topP float64) VertexOption {
	return func(c *VertexClient) {
		c.params.TopP = topP
	}
}

// WithVertexMaxTokens sets the maximum number of tokens to generate.
// Default: 4096
func WithVertexMaxTokens(maxTokens int64) VertexOption {
	return func(c *VertexClient) {
		c.params.MaxTokens = maxTokens
	}
}

// WithVertexSystemPrompt sets the system prompt for the client
func WithVertexSystemPrompt(prompt string) VertexOption {
	return func(c *VertexClient) {
		c.systemPrompt = prompt
	}
}

// WithVertexContentType sets the content type for text generation.
// This determines the format of the generated content.
func WithVertexContentType(contentType gollem.ContentType) VertexOption {
	return func(c *VertexClient) {
		c.contentType = contentType
	}
}

// WithVertexGoogleCloudOptions sets additional options for Google Cloud Platform.
// These options are passed to the underlying Vertex AI client.
func WithVertexGoogleCloudOptions(options ...option.ClientOption) VertexOption {
	return func(c *VertexClient) {
		c.gcpOptions = options
	}
}

// NewWithVertexAI creates a new client for Claude models via Google Vertex AI.
// It requires a project ID and location, and can be configured with additional options.
func NewWithVertexAI(ctx context.Context, projectID, location string, options ...VertexOption) (*VertexClient, error) {
	if projectID == "" {
		return nil, goerr.New("projectID is required")
	}
	if location == "" {
		return nil, goerr.New("location is required")
	}

	client := &VertexClient{
		projectID:      projectID,
		location:       location,
		defaultModel:   DefaultVertexClaudeModel,
		embeddingModel: DefaultVertexClaudeEmbeddingModel,
		params: generationParameters{
			Temperature: 0.7,
			TopP:        1.0,
			MaxTokens:   4096,
		},
		contentType: gollem.ContentTypeText,
	}

	for _, option := range options {
		option(client)
	}

	newClient, err := genai.NewClient(ctx, projectID, location, client.gcpOptions...)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create Vertex AI client")
	}

	client.client = newClient

	return client, nil
}

// VertexSession is a session for Claude via Vertex AI.
// It maintains the conversation state and handles message generation.
type VertexSession struct {
	// session is the underlying Vertex AI chat session.
	session *genai.ChatSession

	// defaultModel is the model to use for chat completions.
	defaultModel string

	// generation parameters
	params generationParameters

	cfg gollem.SessionConfig
}

// NewSession creates a new session for Claude via Vertex AI.
// It converts the provided tools to Vertex AI's tool format and initializes a new chat session.
func (c *VertexClient) NewSession(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
	cfg := gollem.NewSessionConfig(options...)

	// Convert gollem.Tool to *genai.FunctionDeclaration
	genaiFunctions := make([]*genai.FunctionDeclaration, len(cfg.Tools()))
	for i, tool := range cfg.Tools() {
		converted := convertToolToGenai(tool)
		genaiFunctions[i] = converted
	}

	var messages []*genai.Content

	if cfg.History() != nil {
		history, err := cfg.History().ToGemini()
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert history to genai.Content")
		}
		messages = append(messages, history...)
	}

	// Create model with Claude model name
	model := c.client.GenerativeModel(c.defaultModel)
	
	// Set generation parameters
	temperature := float32(c.params.Temperature)
	topP := float32(c.params.TopP)
	maxTokens := int32(c.params.MaxTokens)
	
	model.GenerationConfig = genai.GenerationConfig{
		Temperature:      &temperature,
		TopP:             &topP,
		MaxOutputTokens:  &maxTokens,
	}

	// Set content type
	switch cfg.ContentType() {
	case gollem.ContentTypeJSON:
		model.GenerationConfig.ResponseMIMEType = "application/json"
	case gollem.ContentTypeText:
		model.GenerationConfig.ResponseMIMEType = "text/plain"
	}

	// Set system prompt
	if cfg.SystemPrompt() != "" {
		model.SystemInstruction = &genai.Content{
			Role:  "system",
			Parts: []genai.Part{genai.Text(cfg.SystemPrompt())},
		}
	}

	// Set tools
	if len(genaiFunctions) > 0 {
		model.Tools = []*genai.Tool{
			{
				FunctionDeclarations: genaiFunctions,
			},
		}
	}

	session := &VertexSession{
		session:      model.StartChat(),
		defaultModel: c.defaultModel,
		params:       c.params,
		cfg:          cfg,
	}

	if len(messages) > 0 {
		session.session.History = messages
	}

	return session, nil
}

// convertToolToGenai converts a gollem.Tool to *genai.FunctionDeclaration
func convertToolToGenai(tool gollem.Tool) *genai.FunctionDeclaration {
	spec := tool.Spec()
	
	// Convert parameters
	properties := make(map[string]*genai.Schema)
	required := make([]string, 0)
	
	for name, param := range spec.Parameters {
		schema := &genai.Schema{
			Type:        convertParameterType(param.Type),
			Description: param.Description,
		}
		
		// Handle enum values
		if len(param.Enum) > 0 {
			schema.Enum = make([]string, len(param.Enum))
			for i, v := range param.Enum {
				schema.Enum[i] = fmt.Sprintf("%v", v)
			}
		}
		
		properties[name] = schema
		
		// Check if parameter is required
		for _, req := range spec.Required {
			if req == name {
				required = append(required, name)
				break
			}
		}
	}
	
	return &genai.FunctionDeclaration{
		Name:        spec.Name,
		Description: spec.Description,
		Parameters: &genai.Schema{
			Type:       genai.TypeObject,
			Properties: properties,
			Required:   required,
		},
	}
}

// convertParameterType converts gollem parameter type to genai schema type
func convertParameterType(paramType gollem.ParameterType) genai.Type {
	switch paramType {
	case gollem.TypeString:
		return genai.TypeString
	case gollem.TypeNumber:
		return genai.TypeNumber
	case gollem.TypeInteger:
		return genai.TypeInteger
	case gollem.TypeBoolean:
		return genai.TypeBoolean
	case gollem.TypeArray:
		return genai.TypeArray
	case gollem.TypeObject:
		return genai.TypeObject
	default:
		return genai.TypeString
	}
}

// History returns the conversation history
func (s *VertexSession) History() *gollem.History {
	return gollem.NewHistoryFromGemini(s.session.History)
}

// convertInputs converts gollem.Input to Vertex AI parts
func (s *VertexSession) convertInputs(input ...gollem.Input) ([]genai.Part, error) {
	parts := make([]genai.Part, len(input))
	for i, in := range input {
		switch v := in.(type) {
		case gollem.Text:
			parts[i] = genai.Text(string(v))
		case gollem.FunctionResponse:
			if v.Error != nil {
				parts[i] = genai.FunctionResponse{
					Name: v.Name,
					Response: map[string]any{
						"error_message": fmt.Sprintf("%+v", v.Error),
					},
				}
			} else {
				parts[i] = genai.FunctionResponse{
					Name:     v.Name,
					Response: v.Data,
				}
			}
		default:
			return nil, goerr.Wrap(gollem.ErrInvalidParameter, "invalid input")
		}
	}
	return parts, nil
}

// processVertexResponse converts Vertex AI response to gollem.Response
func processVertexResponse(resp *genai.GenerateContentResponse) (*gollem.Response, error) {
	if len(resp.Candidates) == 0 {
		return &gollem.Response{}, nil
	}

	response := &gollem.Response{
		Texts:         make([]string, 0),
		FunctionCalls: make([]*gollem.FunctionCall, 0),
	}

	for i, candidate := range resp.Candidates {
		// Check for malformed function call errors
		if candidate.FinishReason.String() == "FinishReasonMalformedFunctionCall" {
			return nil, goerr.New("malformed function call detected",
				goerr.V("candidate_index", i),
				goerr.V("content_parts", len(candidate.Content.Parts)),
				goerr.V("finish_reason", candidate.FinishReason.String()))
		}

		if len(candidate.Content.Parts) == 0 {
			continue
		}

		for _, part := range candidate.Content.Parts {
			switch v := part.(type) {
			case genai.Text:
				response.Texts = append(response.Texts, string(v))
			case genai.FunctionCall:
				response.FunctionCalls = append(response.FunctionCalls, &gollem.FunctionCall{
					Name:      v.Name,
					Arguments: v.Args,
				})
			}
		}
	}

	return response, nil
}

// GenerateContent processes the input and generates a response.
// It handles both text messages and function responses.
func (s *VertexSession) GenerateContent(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
	parts, err := s.convertInputs(input...)
	if err != nil {
		return nil, err
	}

	resp, err := s.session.SendMessage(ctx, parts...)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to send message to Claude via Vertex AI")
	}

	return processVertexResponse(resp)
}

// GenerateStream processes the input and generates a response stream.
// It handles both text messages and function responses, and returns a channel for streaming responses.
func (s *VertexSession) GenerateStream(ctx context.Context, input ...gollem.Input) (<-chan *gollem.Response, error) {
	parts, err := s.convertInputs(input...)
	if err != nil {
		return nil, err
	}

	iter := s.session.SendMessageStream(ctx, parts...)
	responseChan := make(chan *gollem.Response)

	go func() {
		defer close(responseChan)

		for {
			resp, err := iter.Next()
			if err != nil {
				if strings.Contains(err.Error(), "Done") {
					return
				}
				responseChan <- &gollem.Response{
					Error: goerr.Wrap(err, "failed to generate stream"),
				}
				return
			}

			processedResp, err := processVertexResponse(resp)
			if err != nil {
				responseChan <- &gollem.Response{
					Error: goerr.Wrap(err, "failed to process response"),
				}
				return
			}
			responseChan <- processedResp
		}
	}()

	return responseChan, nil
}

// GenerateEmbedding generates embeddings for the given input texts.
// Note: Claude models through Vertex AI may not support embeddings directly.
// This is a placeholder implementation that returns an error.
func (c *VertexClient) GenerateEmbedding(ctx context.Context, dimension int, input []string) ([][]float64, error) {
	return nil, goerr.New("embedding generation not supported for Claude models via Vertex AI")
}