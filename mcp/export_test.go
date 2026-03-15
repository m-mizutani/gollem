package mcp

import "os"

var (
	InputSchemaToParameter = convertInputSchemaToParameter
	MCPContentToMap        = convertContentToMap
	ConvertSchemaProperty  = convertSchemaProperty
)

// BuildStdioEnv replicates the environment variable construction logic used in NewStdio
// for testing purposes.
func BuildStdioEnv(envVars []string) []string {
	return append(os.Environ(), envVars...)
}
