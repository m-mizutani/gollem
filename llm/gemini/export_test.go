package gemini

import (
	"github.com/m-mizutani/gollem"
	"google.golang.org/genai"
)

// Export convert functions for testing
var (
	ConvertTool              = convertTool
	ConvertParameterToSchema = convertParameterToSchema
)

// GetGenerationConfig returns the generationConfig for testing
func (c *Client) GetGenerationConfig() *genai.GenerateContentConfig {
	return c.generationConfig
}

// Export for testing
type APIClient = apiClient

// NewSessionWithAPIClient creates a new session with a custom API client for testing
func NewSessionWithAPIClient(client apiClient, cfg gollem.SessionConfig, model string) *Session {
	// Initialize currentHistory from config or create new
	var currentHistory *gollem.History
	if cfg.History() != nil {
		currentHistory = cfg.History()
	} else {
		currentHistory = &gollem.History{}
	}

	// Create generation config
	config := &genai.GenerateContentConfig{}

	return &Session{
		apiClient:      client,
		model:          model,
		config:         config,
		currentHistory: currentHistory,
		cfg:            cfg,
	}
}
