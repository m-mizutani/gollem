package claude_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/m-mizutani/gollem/llm/claude"
	"github.com/m-mizutani/gt"
)

func TestExtractJSONFromResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple JSON",
			input:    `{"action": "continue", "reason": "test"}`,
			expected: `{"action": "continue", "reason": "test"}`,
		},
		{
			name:     "JSON in markdown code block",
			input:    "```json\n{\"action\": \"continue\", \"reason\": \"test\"}\n```",
			expected: `{"action": "continue", "reason": "test"}`,
		},
		{
			name:     "JSON in code block without json tag",
			input:    "```\n{\"action\": \"continue\", \"reason\": \"test\"}\n```",
			expected: `{"action": "continue", "reason": "test"}`,
		},
		{
			name:     "JSON with braces in string literals",
			input:    `{"reason": "The process failed because of an unexpected character: '}'."}`,
			expected: `{"reason": "The process failed because of an unexpected character: '}'."}`,
		},
		{
			name:     "JSON with escaped quotes",
			input:    `{"message": "He said \"Hello {world}\" to me."}`,
			expected: `{"message": "He said \"Hello {world}\" to me."}`,
		},
		{
			name:     "nested JSON objects",
			input:    `{"outer": {"inner": {"value": "contains } brace"}}}`,
			expected: `{"outer": {"inner": {"value": "contains } brace"}}}`,
		},
		{
			name:     "JSON with text before and after",
			input:    `Some text before {"action": "complete"} some text after`,
			expected: `{"action": "complete"}`,
		},
		{
			name: "complex JSON with multiple string issues",
			input: `{
				"action": "continue",
				"reason": "The function call failed with error: '{\"code\": 500}'",
				"next_prompt": "Let's try a different approach with \"escapes\" and {nested} content"
			}`,
			expected: `{
				"action": "continue",
				"reason": "The function call failed with error: '{\"code\": 500}'",
				"next_prompt": "Let's try a different approach with \"escapes\" and {nested} content"
			}`,
		},
		{
			name:     "simple JSON array",
			input:    `[{"id": 1, "name": "test"}]`,
			expected: `[{"id": 1, "name": "test"}]`,
		},
		{
			name:     "JSON array in markdown code block",
			input:    "```json\n[{\"id\": 1, \"name\": \"test\"}]\n```",
			expected: `[{"id": 1, "name": "test"}]`,
		},
		{
			name:     "JSON array with brackets in string literals",
			input:    `[{"message": "The process failed because of an unexpected character: ']'."}]`,
			expected: `[{"message": "The process failed because of an unexpected character: ']'."}]`,
		},
		{
			name:     "nested JSON arrays",
			input:    `[{"items": [1, 2, 3], "name": "test"}]`,
			expected: `[{"items": [1, 2, 3], "name": "test"}]`,
		},
		{
			name:     "JSON array with text before and after",
			input:    `Some text before [{"id": 1}] some text after`,
			expected: `[{"id": 1}]`,
		},
		{
			name:     "mixed JSON object and array",
			input:    `{"data": [{"id": 1}, {"id": 2}], "count": 2}`,
			expected: `{"data": [{"id": 1}, {"id": 2}], "count": 2}`,
		},
		{
			name:     "brace before bracket",
			input:    `{"items": [1, 2, 3]} and [{"id": 1}]`,
			expected: `{"items": [1, 2, 3]}`,
		},
		{
			name:     "bracket before brace",
			input:    `[{"id": 1}] and {"items": [1, 2, 3]}`,
			expected: `[{"id": 1}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := claude.ExtractJSONFromResponse(tt.input)
			gt.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractJSONFromResponse_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no JSON content",
			input:    "This is just plain text",
			expected: "This is just plain text",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    "   \n\t  ",
			expected: "",
		},
		{
			name:     "incomplete JSON",
			input:    `{"action": "continue", "reason": "incomplete`,
			expected: `{"action": "continue", "reason": "incomplete`,
		},
		{
			name:     "incomplete JSON array",
			input:    `[{"id": 1, "name": "incomplete`,
			expected: `[{"id": 1, "name": "incomplete`,
		},
		{
			name:     "JSON array with object inside",
			input:    `[{"nested": {"key": "value"}}]`,
			expected: `[{"nested": {"key": "value"}}]`,
		},
		{
			name:     "long string with newlines like the bug case",
			input:    `{"goal": "Create query for project data", "approach": "direct", "reasoning": "Simple task based on known structure", "response": "Here is the query:\\n\\nSELECT * FROM table"}`,
			expected: `{"goal": "Create query for project data", "approach": "direct", "reasoning": "Simple task based on known structure", "response": "Here is the query:\\n\\nSELECT * FROM table"}`,
		},
		{
			name:     "truncated JSON like the bug case",
			input:    `{"goal": "Create query for project data", "approach": "direct", "reasoning": "Simple task based on known structure", "response": "Here is the query:\\n\\n`,
			expected: `{"goal": "Create query for project data", "approach": "direct", "reasoning": "Simple task based on known structure", "response": "Here is the query:\\n\\n`,
		},
		{
			name:     "deeply nested JSON with many escapes",
			input:    `{"data": {"nested": {"value": "string with \\"quotes\\" and {braces} and [brackets]", "array": [{"item": "value with \\n newline"}]}}}`,
			expected: `{"data": {"nested": {"value": "string with \\"quotes\\" and {braces} and [brackets]", "array": [{"item": "value with \\n newline"}]}}}`,
		},
		{
			name:     "JSON with unicode characters",
			input:    `{"message": "こんにちは世界 🌍", "emoji": "😀", "special": "Iñtërnâtiônàlizætiøn"}`,
			expected: `{"message": "こんにちは世界 🌍", "emoji": "😀", "special": "Iñtërnâtiônàlizætiøn"}`,
		},
		{
			name:     "malformed JSON with missing quote",
			input:    `{"key": "value, "missing": "quote"}`,
			expected: `{"key": "value, "missing": "quote"}`,
		},
		{
			name:     "JSON with only opening brace",
			input:    `{"incomplete": true`,
			expected: `{"incomplete": true`,
		},
		{
			name:     "JSON with extra closing brace",
			input:    `{"complete": true}}`,
			expected: `{"complete": true}`,
		},
		{
			name:     "multiple JSON objects",
			input:    `{"first": "object"} {"second": "object"}`,
			expected: `{"first": "object"}`,
		},
		{
			name:     "JSON with complex escaping",
			input:    `{"path": "C:\\\\Users\\\\test\\\\file.txt", "regex": "\\\\d{4}-\\\\d{2}-\\\\d{2}", "quote": "\\"nested \\\\\\"quote\\\\\\"\""}`,
			expected: `{"path": "C:\\\\Users\\\\test\\\\file.txt", "regex": "\\\\d{4}-\\\\d{2}-\\\\d{2}", "quote": "\\"nested \\\\\\"quote\\\\\\"\""}`,
		},
		{
			name:     "streaming response simulation - incomplete",
			input:    `{"streaming": "this is a partial response that gets cut off in the middle of`,
			expected: `{"streaming": "this is a partial response that gets cut off in the middle of`,
		},
		{
			name:     "streaming response simulation - with newlines",
			input:    `{"multiline": "line1\\nline2\\nline3`,
			expected: `{"multiline": "line1\\nline2\\nline3`,
		},
		{
			name:     "very large JSON response",
			input:    generateLargeJSONInput(),
			expected: generateLargeJSONInput(),
		},
		{
			name:     "JSON with all escape sequences",
			input:    `{"escapes": "quote:\" backslash:\\\\ tab:\\t newline:\\n return:\\r formfeed:\\f backspace:\\b"}`,
			expected: `{"escapes": "quote:\" backslash:\\\\ tab:\\t newline:\\n return:\\r formfeed:\\f backspace:\\b"}`,
		},
		{
			name:     "JSON with control characters",
			input:    "{\"control\": \"line1\\u000Aline2\\u000Dline3\\u0009tab\"}",
			expected: "{\"control\": \"line1\\u000Aline2\\u000Dline3\\u0009tab\"}",
		},
		{
			name:     "partial JSON in streaming scenario",
			input:    `{"partial": "response`, // intentionally incomplete
			expected: `{"partial": "response`, // should return as-is for incomplete
		},
		{
			name:     "JSON with SQL injection attempt",
			input:    `{"query": "SELECT * FROM users WHERE id = '1' OR '1'='1'; DROP TABLE users; --"}`,
			expected: `{"query": "SELECT * FROM users WHERE id = '1' OR '1'='1'; DROP TABLE users; --"}`,
		},
		{
			name:     "JSON in response with markdown and extra text",
			input:    "Here's the JSON response:\n```json\n{\"result\": \"success\", \"data\": {\"key\": \"value\"}}\n```\nThat's the response.",
			expected: `{"result": "success", "data": {"key": "value"}}`,
		},
		{
			name:     "empty JSON object and array",
			input:    `{} []`,
			expected: `{}`,
		},
		{
			name:     "JSON with null values",
			input:    `{"null_value": null, "empty_string": "", "zero": 0, "false": false}`,
			expected: `{"null_value": null, "empty_string": "", "zero": 0, "false": false}`,
		},
		{
			name:     "JSON with numbers and scientific notation",
			input:    `{"int": 123, "float": 12.34, "scientific": 1.23e-4, "negative": -456}`,
			expected: `{"int": 123, "float": 12.34, "scientific": 1.23e-4, "negative": -456}`,
		},
		{
			name:     "deeply nested with unbalanced quotes in streaming",
			input:    `{"level1": {"level2": {"level3": {"data": "incomplete response with quote and missing end`,
			expected: `{"level1": {"level2": {"level3": {"data": "incomplete response with quote and missing end`,
		},
		{
			name:     "JSON with extreme nesting",
			input:    buildDeeplyNestedJSON(50),
			expected: buildDeeplyNestedJSON(50),
		},
		{
			name:     "JSON with multiple consecutive escapes",
			input:    `{"path": "C:\\\\\\\\server\\\\\\\\share\\\\\\\\file.txt"}`,
			expected: `{"path": "C:\\\\\\\\server\\\\\\\\share\\\\\\\\file.txt"}`,
		},
		{
			name:     "JSON with mixed quotes and braces",
			input:    `{"message": "User said: \"Please process {data: [1,2,3]} now\" but {error: true}"}`,
			expected: `{"message": "User said: \"Please process {data: [1,2,3]} now\" but {error: true}"}`,
		},
		{
			name:     "streaming incomplete with escape at end",
			input:    `{"field": "value with backslash at end\\`,
			expected: `{"field": "value with backslash at end\\`,
		},
		{
			name:     "JSON with array of objects containing special chars",
			input:    `[{"msg": "Hello {world}"}, {"msg": "Quote: \"test\""}, {"msg": "Backslash: \\test"}]`,
			expected: `[{"msg": "Hello {world}"}, {"msg": "Quote: \"test\""}, {"msg": "Backslash: \\test"}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := claude.ExtractJSONFromResponse(tt.input)
			gt.Equal(t, tt.expected, result)
		})
	}
}

// generateLargeJSONInput generates a large JSON input for testing
func generateLargeJSONInput() string {
	var sb strings.Builder
	sb.WriteString(`{"large_data": {`)

	for i := range 100 {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf(`"field_%d": "This is a very long string with special characters: {}[]\"\\and newlines\\nand tabs\\tfield %d"`, i, i))
	}

	sb.WriteString(`}, "array": [`)
	for i := range 50 {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf(`{"item": %d, "description": "Item %d with {braces} and [brackets] and \"quotes\""}`, i, i))
	}
	sb.WriteString(`]}`)

	return sb.String()
}

// buildDeeplyNestedJSON creates a deeply nested JSON structure for testing
func buildDeeplyNestedJSON(depth int) string {
	if depth <= 0 {
		return `{"value": "deep"}`
	}

	return fmt.Sprintf(`{"level_%d": %s}`, depth, buildDeeplyNestedJSON(depth-1))
}

func TestExtractJSONFromResponse_ErrorHandling(t *testing.T) {
	// Test cases that might cause issues in the original implementation
	tests := []struct {
		name     string
		input    string
		expected string // What we expect our improved function to return
	}{
		{
			name:     "string with only opening quote",
			input:    `{"field": "unclosed string`,
			expected: `{"field": "unclosed string`,
		},
		{
			name:     "multiple opening braces",
			input:    `{{{{"too": "many"}`,
			expected: `{{{{"too": "many"}`,
		},
		{
			name:     "escaped quote at string end",
			input:    `{"field": "ends with quote\\"`,
			expected: `{"field": "ends with quote\\"`,
		},
		{
			name:     "nested quotes with braces",
			input:    `{"data": "string with {\"nested\": \"value\"} object"}`,
			expected: `{"data": "string with {\"nested\": \"value\"} object"}`,
		},
		{
			name:     "streaming cut off at worst possible place",
			input:    `{"important": "data", "response": "The SQL query is: SELECT * FROM table WHERE condition = 'some very long condition that gets cut off in the middle of streaming because the response is too long and Claude decides to truncate here`,
			expected: `{"important": "data", "response": "The SQL query is: SELECT * FROM table WHERE condition = 'some very long condition that gets cut off in the middle of streaming because the response is too long and Claude decides to truncate here`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := claude.ExtractJSONFromResponse(tt.input)
			gt.Equal(t, tt.expected, result)
		})
	}
}
