package claude

// Export convert functions for testing
var (
	ConvertTool                 = convertTool
	ConvertParameterToSchema    = convertParameterToSchema
	ExtractJSONFromResponse     = extractJSONFromResponse
	ConvertGollemInputsToClaude = convertGollemInputsToClaude
	IsValidJSONPublic           = isValidJSON
)

type JsonSchema = jsonSchema
