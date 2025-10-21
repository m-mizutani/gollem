package gemini

import (
	"github.com/m-mizutani/gollem"
	"google.golang.org/genai"
)

// Export convert functions for testing
var (
	ConvertTool              = convertTool
	ConvertParameterToSchema = convertParameterToSchema
	TokenLimitErrorOptions   = tokenLimitErrorOptions
)

// GetGenerationConfig returns the generationConfig for testing
func (c *Client) GetGenerationConfig() *genai.GenerateContentConfig {
	return c.generationConfig
}

// Export for testing
type APIClient = apiClient

// NewSessionWithAPIClient creates a new session with a custom API client for testing
func NewSessionWithAPIClient(client apiClient, cfg gollem.SessionConfig, model string) (*Session, error) {
	// Initialize historyContents from config
	var historyContents []*genai.Content
	if cfg.History() != nil {
		var err error
		historyContents, err = ToContents(cfg.History())
		if err != nil {
			return nil, err
		}
	}

	// Create generation config
	config := &genai.GenerateContentConfig{}

	return &Session{
		apiClient:       client,
		model:           model,
		config:          config,
		historyContents: historyContents,
		cfg:             cfg,
	}, nil
}
