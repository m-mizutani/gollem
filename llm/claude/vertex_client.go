package claude

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
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
	return convertGollemInputsToClaude(ctx, input...)
}

// GenerateContent processes the input and generates a response.
func (s *VertexAnthropicSession) GenerateContent(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
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

	resp, err := generateClaudeContent(
		ctx,
		s.client,
		s.messages,
		s.defaultModel,
		s.params,
		tools,
		s.cfg,
		"Claude Vertex",
	)
	if err != nil {
		return nil, err
	}

	// Add assistant's response to message history
	s.messages = append(s.messages, resp.ToParam())

	return processResponse(resp), nil
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

	return generateClaudeStream(
		ctx,
		s.client,
		s.messages,
		s.defaultModel,
		s.params,
		tools,
		s.cfg,
		&s.messages,
	)
}

// GenerateEmbedding generates embeddings for the given input texts.
func (c *VertexClient) GenerateEmbedding(ctx context.Context, dimension int, input []string) ([][]float64, error) {
	return nil, goerr.New("embedding generation not supported for Claude models via Vertex AI")
}
