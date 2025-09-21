package claude

// Export convert functions for testing
var (
	ConvertTool                 = convertTool
	ConvertParameterToSchema    = convertParameterToSchema
	ConvertGollemInputsToClaude = convertGollemInputsToClaude
	CreateSystemPrompt          = createSystemPrompt
)

type JsonSchema = jsonSchema
