package claude

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/m-mizutani/gollem"
)

// Export convert functions for testing
var (
	ConvertTool                 = convertTool
	ConvertParameterToSchema    = convertParameterToSchema
	ConvertGollemInputsToClaude = convertGollemInputsToClaude
	CreateSystemPrompt          = createSystemPrompt
)

type JsonSchema = jsonSchema

// Export for testing
type APIClient = apiClient

// NewSessionWithAPIClient creates a new session with a custom API client for testing
func NewSessionWithAPIClient(client apiClient, cfg gollem.SessionConfig, model string) (*Session, error) {
	tools := make([]anthropic.ToolUnionParam, 0, len(cfg.Tools()))
	for _, tool := range cfg.Tools() {
		tools = append(tools, convertTool(tool))
	}

	// Initialize historyMessages from config
	var historyMessages []anthropic.MessageParam
	if cfg.History() != nil {
		var err error
		historyMessages, err = ToMessages(cfg.History())
		if err != nil {
			return nil, err
		}
	}

	return &Session{
		apiClient:       client,
		defaultModel:    model,
		tools:           tools,
		historyMessages: historyMessages,
		params:          generationParameters{},
		cfg:             cfg,
	}, nil
}
