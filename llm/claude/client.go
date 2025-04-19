package claude

import (
	"context"
	"encoding/json"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/servant/llm"
)

type Client struct {
	client       *anthropic.Client
	defaultModel string
}

type Option func(*Client)

func WithDefaultModel(modelName string) Option {
	return func(c *Client) {
		c.defaultModel = modelName
	}
}

func New(ctx context.Context, apiKey string, options ...Option) (*Client, error) {
	client := &Client{
		defaultModel: anthropic.ModelClaude3_5SonnetLatest,
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

func (c *Client) NewSession(ctx context.Context, tools []llm.Tool) (llm.Session, error) {
	// Convert llm.Tool to anthropic.ToolUnionParam
	claudeTools := make([]*anthropic.ToolParam, len(tools))
	for i, tool := range tools {
		claudeTools[i] = convertTool(tool)
	}

	session := &Session{
		client:       c.client,
		defaultModel: c.defaultModel,
		tools:        claudeTools,
	}

	return session, nil
}

type Session struct {
	client       *anthropic.Client
	defaultModel string
	tools        []*anthropic.ToolParam
	messages     []anthropic.MessageParam
}

func (s *Session) Generate(ctx context.Context, input ...llm.Input) (*llm.Response, error) {
	var toolResults []anthropic.ContentBlockParamUnion
	// Convert input to messages
	for _, in := range input {
		switch v := in.(type) {
		case llm.Text:
			s.messages = append(s.messages, anthropic.NewUserMessage(
				anthropic.NewTextBlock(string(v)),
			))

		case llm.FunctionResponse:
			response, err := json.Marshal(v.Data)
			if err != nil {
				return nil, goerr.Wrap(err, "failed to marshal function response")
			}
			toolResults = append(toolResults, anthropic.NewToolResultBlock(v.ID, string(response), v.Error != nil))

		default:
			return nil, goerr.Wrap(llm.ErrInvalidParameter, "invalid input")
		}
	}

	if len(toolResults) > 0 {
		s.messages = append(s.messages, anthropic.NewUserMessage(toolResults...))
	}

	claudeTools := make([]anthropic.ToolUnionParam, len(s.tools))
	for i, tool := range s.tools {
		claudeTools[i] = anthropic.ToolUnionParam{OfTool: tool}
	}

	params := anthropic.MessageNewParams{
		Model:     s.defaultModel,
		MaxTokens: 4096,
		Tools:     claudeTools,
		Messages:  s.messages,
	}

	resp, err := s.client.Messages.New(ctx, params)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create message")
	}
	s.messages = append(s.messages, resp.ToParam())

	if len(resp.Content) == 0 {
		return &llm.Response{}, nil
	}

	response := &llm.Response{
		Texts:         make([]string, 0),
		FunctionCalls: make([]*llm.FunctionCall, 0),
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

			response.FunctionCalls = append(response.FunctionCalls, &llm.FunctionCall{
				ID:        toolUseBlock.ID,
				Name:      toolUseBlock.Name,
				Arguments: args,
			})
		}
	}

	return response, nil
}
