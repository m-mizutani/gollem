package claude_test

import (
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := claude.ExtractJSONFromResponse(tt.input)
			gt.Equal(t, tt.expected, result)
		})
	}
}
