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
func NewSessionWithAPIClient(client apiClient, cfg gollem.SessionConfig, model string) *Session {
	tools := make([]anthropic.ToolUnionParam, 0, len(cfg.Tools()))
	for _, tool := range cfg.Tools() {
		tools = append(tools, convertTool(tool))
	}

	// Initialize currentHistory from config or create new
	var currentHistory *gollem.History
	if cfg.History() != nil {
		currentHistory = cfg.History()
	} else {
		currentHistory = &gollem.History{
			LLType:  gollem.LLMTypeClaude,
			Version: gollem.HistoryVersion,
		}
	}

	return &Session{
		apiClient:      client,
		defaultModel:   model,
		tools:          tools,
		currentHistory: currentHistory,
		params:         generationParameters{},
		cfg:            cfg,
	}
}
