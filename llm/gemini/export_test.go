package gemini

import "google.golang.org/genai"

// Export convert functions for testing
var (
	ConvertTool              = convertTool
	ConvertParameterToSchema = convertParameterToSchema
)

// GetGenerationConfig returns the generationConfig for testing
func (c *Client) GetGenerationConfig() *genai.GenerateContentConfig {
	return c.generationConfig
}
