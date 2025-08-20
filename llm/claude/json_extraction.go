package claude

import (
	"encoding/json"
	"strings"
)

// extractJSONFromResponse cleans the response text to extract valid JSON
// This is necessary because Claude returns JSON wrapped in markdown code blocks
// even when ContentTypeJSON is specified.
//
// This function uses heuristics to find JSON boundaries and is not a full JSON parser.
// It handles basic cases like { and } inside string literals, but has limitations:
// - Does not handle all JSON escape sequences perfectly
// - May struggle with complex nested structures in unusual formatting
// - Works well for typical LLM-generated JSON responses
//
// For most Claude responses, this pragmatic approach provides reliable JSON extraction.
func extractJSONFromResponse(text string) string {
	// Remove leading/trailing whitespace
	text = strings.TrimSpace(text)

	// Try to extract JSON from markdown code blocks using pre-compiled regex
	matches := codeBlockRegex.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Try to find JSON object or array boundaries
	firstBrace := strings.Index(text, "{")
	firstBracket := strings.Index(text, "[")

	if firstBrace == -1 && firstBracket == -1 {
		return text // No JSON found, return original
	}

	var start int
	var expectedClosing rune
	if firstBracket == -1 || (firstBrace != -1 && firstBrace < firstBracket) {
		start = firstBrace
		expectedClosing = '}'
	} else {
		start = firstBracket
		expectedClosing = ']'
	}

	// Find the matching closing brace/bracket with improved string literal handling
	// This handles JSON escape sequences more robustly
	depth := 0
	inString := false
	i := start

	for i < len(text) {
		char := rune(text[i])

		if inString {
			// Handle escape sequences in strings
			if char == '\\' {
				// Skip the escaped character
				i++
				if i < len(text) {
					i++ // Skip both the backslash and the escaped character
				}
				continue
			} else if char == '"' {
				inString = false
			}
		} else {
			switch char {
			case '"':
				inString = true
			case '{':
				if expectedClosing == '}' {
					depth++
				}
			case '}':
				if expectedClosing == '}' {
					depth--
					if depth == 0 {
						// Found the matching closing brace
						candidate := text[start : i+1]
						// Validate that this is likely complete JSON
						if isLikelyCompleteJSON(candidate) {
							return candidate
						}
					}
				}
			case '[':
				if expectedClosing == ']' {
					depth++
				}
			case ']':
				if expectedClosing == ']' {
					depth--
					if depth == 0 {
						// Found the matching closing bracket
						candidate := text[start : i+1]
						// Validate that this is likely complete JSON
						if isLikelyCompleteJSON(candidate) {
							return candidate
						}
					}
				}
			}
		}
		i++
	}

	// If we reach here, no complete JSON was found
	// Check if the text might be truncated/incomplete
	if depth > 0 || inString {
		// JSON appears incomplete (unbalanced braces/brackets or unclosed string)
		// Return original text to avoid breaking incomplete streaming responses
		return text
	}

	// Return from start position if we found the beginning but no proper end
	return text[start:]
}

// isLikelyCompleteJSON performs basic validation to check if the extracted text
// looks like complete JSON (not a rigorous JSON parser, just basic heuristics)
func isLikelyCompleteJSON(text string) bool {
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return false
	}

	// Check if it starts and ends with matching delimiters
	if (text[0] == '{' && text[len(text)-1] == '}') ||
		(text[0] == '[' && text[len(text)-1] == ']') {
		// Quick validation: try to unmarshal to see if it's valid JSON
		var temp any
		return json.Unmarshal([]byte(text), &temp) == nil
	}

	return false
}
