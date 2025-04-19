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
		defaultModel: anthropic.ModelClaude3_7SonnetLatest,
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
			s.messages = append(s.messages, anthropic.NewAssistantMessage(
				anthropic.NewTextBlock(string(response)),
			))
		default:
			return nil, goerr.Wrap(llm.ErrInvalidParameter, "invalid input")
		}
	}

	var toolChoice anthropic.ToolChoiceUnionParam
	if len(s.tools) > 0 {
		toolChoice = anthropic.ToolChoiceParamOfToolChoiceTool("auto")
	}

	claudeTools := make([]anthropic.ToolUnionParam, len(s.tools))
	for i, tool := range s.tools {
		claudeTools[i] = anthropic.ToolUnionParam{OfTool: tool}
	}

	params := anthropic.MessageNewParams{
		Model:      s.defaultModel,
		MaxTokens:  4096,
		Tools:      claudeTools,
		ToolChoice: toolChoice,
		Messages:   s.messages,
	}

	resp, err := s.client.Messages.New(ctx, params)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create message")
	}

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
				Name:      toolUseBlock.Name,
				Arguments: args,
			})
		}
	}

	return response, nil
}
