package claude

import (
	"encoding/json"
	"regexp"
	"strings"
)

// extractJSONFromResponse cleans the response text to extract valid JSON
// This function uses multiple extraction strategies in order of preference
// to handle various edge cases and formatting inconsistencies from LLM responses.
func extractJSONFromResponse(text string) string {
	// Remove leading/trailing whitespace
	text = strings.TrimSpace(text)

	if len(text) == 0 {
		return text
	}

	// Strategy 1: Direct JSON validation - check if input is already valid JSON
	if isValidJSON(text) {
		return text
	}

	// Strategy 2: Extract from markdown code blocks (most common case)
	if result := extractFromCodeBlocks(text); result != "" {
		return result
	}

	// Strategy 3: Extract using JSON grammar parsing (most reliable)
	if result := extractUsingJSONGrammar(text); result != "" {
		return result
	}

	// Strategy 4: Simple prefix/suffix trimming
	if result := extractWithTrimming(text); result != "" {
		return result
	}

	// Strategy 5: Multiple JSON object detection
	if result := extractMultipleJSONObjects(text); result != "" {
		return result
	}

	// Strategy 6: Fuzzy extraction for malformed JSON
	if result := extractFuzzyJSON(text); result != "" {
		return result
	}

	// Strategy 7: Last resort - return the longest JSON-like substring
	if result := extractLongestJSONLike(text); result != "" {
		return result
	}

	// If all strategies fail, return original text
	return text
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

// isValidJSON checks if the given text is valid JSON
func isValidJSON(text string) bool {
	var temp any
	return json.Unmarshal([]byte(text), &temp) == nil
}

// extractFromCodeBlocks extracts JSON from markdown code blocks
func extractFromCodeBlocks(text string) string {
	// Try different code block patterns
	patterns := []string{
		`(?s)` + "```" + `json\s*\n?(.*?)\n?` + "```",
		`(?s)` + "```" + `\s*\n?(.*?)\n?` + "```",
		`(?s)` + "`" + `{(.*?)}` + "`",
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) > 1 {
				candidate := strings.TrimSpace(match[1])
				if len(candidate) > 0 && (candidate[0] == '{' || candidate[0] == '[') {
					if isValidJSON(candidate) {
						return candidate
					}
				}
			}
		}
	}
	return ""
}

// extractUsingJSONGrammar uses JSON grammar to find valid JSON boundaries
func extractUsingJSONGrammar(text string) string {
	text = strings.TrimSpace(text)

	// Find potential JSON start positions
	starts := []int{}
	for i, char := range text {
		if char == '{' || char == '[' {
			starts = append(starts, i)
		}
	}

	// Try each potential start position and find the longest valid JSON
	var bestResult string
	maxLength := 0

	for _, start := range starts {
		if result := extractJSONFromPosition(text, start); result != "" && len(result) > maxLength {
			bestResult = result
			maxLength = len(result)
		}
	}

	return bestResult
}

// extractJSONFromPosition attempts to extract JSON starting from a specific position
func extractJSONFromPosition(text string, start int) string {
	if start >= len(text) {
		return ""
	}

	depth := 0
	inString := false
	escaped := false
	braceType := rune(text[start]) // '{' or '['

	for i := start; i < len(text); i++ {
		char := rune(text[i])

		if escaped {
			escaped = false
			continue
		}

		if inString {
			if char == '\\' {
				escaped = true
			} else if char == '"' {
				inString = false
			}
			continue
		}

		switch char {
		case '"':
			inString = true
		case '{':
			if braceType == '{' {
				depth++
			}
		case '}':
			if braceType == '{' {
				depth--
				if depth == 0 {
					candidate := text[start : i+1]
					if isValidJSON(candidate) {
						return candidate
					}
				}
			}
		case '[':
			if braceType == '[' {
				depth++
			}
		case ']':
			if braceType == '[' {
				depth--
				if depth == 0 {
					candidate := text[start : i+1]
					if isValidJSON(candidate) {
						return candidate
					}
				}
			}
		}
	}

	return ""
}

// extractWithTrimming tries simple prefix/suffix trimming approaches
func extractWithTrimming(text string) string {
	// Remove common prefixes and suffixes
	prefixes := []string{"Here's the JSON:", "JSON:", "Response:", "```json", "```", "{", "["}
	suffixes := []string{"```", "}", "]"}

	for _, prefix := range prefixes {
		for _, suffix := range suffixes {
			candidate := text
			if after, ok := strings.CutPrefix(candidate, prefix); ok {
				candidate = after
			}
			if strings.HasSuffix(candidate, suffix) && suffix != "}" && suffix != "]" {
				candidate = strings.TrimSuffix(candidate, suffix)
			}

			candidate = strings.TrimSpace(candidate)
			if len(candidate) > 0 && (candidate[0] == '{' || candidate[0] == '[') {
				if isValidJSON(candidate) {
					return candidate
				}
			}
		}
	}

	// Try line-by-line trimming
	lines := strings.Split(text, "\n")
	for start := 0; start < len(lines); start++ {
		for end := len(lines); end > start; end-- {
			candidate := strings.Join(lines[start:end], "\n")
			candidate = strings.TrimSpace(candidate)
			if len(candidate) > 0 && (candidate[0] == '{' || candidate[0] == '[') {
				if isValidJSON(candidate) {
					return candidate
				}
			}
		}
	}

	return ""
}

// extractMultipleJSONObjects handles multiple JSON objects in text
func extractMultipleJSONObjects(text string) string {
	// Split by common delimiters and try each part
	delimiters := []string{"\n\n", "\n", "```", "---"}

	for _, delimiter := range delimiters {
		parts := strings.Split(text, delimiter)
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if len(part) > 0 && (part[0] == '{' || part[0] == '[') {
				if isValidJSON(part) {
					return part
				}
			}
		}
	}

	return ""
}

// extractFuzzyJSON attempts fuzzy extraction for malformed JSON
func extractFuzzyJSON(text string) string {
	// Try to fix common JSON issues
	fixes := []func(string) string{
		func(s string) string { return strings.ReplaceAll(s, "'", "\"") },      // Single to double quotes
		func(s string) string { return strings.ReplaceAll(s, "\u201c", "\"") }, // Smart quotes left
		func(s string) string { return strings.ReplaceAll(s, "\u201d", "\"") }, // Smart quotes right
		func(s string) string {
			return regexp.MustCompile(`([{,]\s*)([a-zA-Z_][a-zA-Z0-9_]*)\s*:`).ReplaceAllString(s, `$1"$2":`)
		}, // Unquoted keys
		func(s string) string {
			return regexp.MustCompile(`:\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*([,}])`).ReplaceAllString(s, `: "$1"$2`)
		}, // Unquoted string values
	}

	// Find potential JSON boundaries
	start := strings.Index(text, "{")
	if start == -1 {
		start = strings.Index(text, "[")
	}
	if start == -1 {
		return ""
	}

	// Try applying fixes and extracting
	for _, fix := range fixes {
		fixed := fix(text[start:])
		if result := extractUsingJSONGrammar(fixed); result != "" {
			return result
		}
	}

	return ""
}

// extractLongestJSONLike returns the longest substring that looks like JSON
func extractLongestJSONLike(text string) string {
	var bestCandidate string
	maxLength := 0

	// Try all possible substrings that start with { or [
	for i := 0; i < len(text); i++ {
		if text[i] == '{' || text[i] == '[' {
			for j := i + 1; j <= len(text); j++ {
				candidate := text[i:j]
				if isValidJSON(candidate) && len(candidate) > maxLength {
					bestCandidate = candidate
					maxLength = len(candidate)
				}
			}
		}
	}

	// If no valid JSON found, try to return something that at least looks like JSON
	if bestCandidate == "" {
		start := strings.Index(text, "{")
		if start == -1 {
			start = strings.Index(text, "[")
		}
		if start != -1 {
			// Find the last } or ]
			lastBrace := strings.LastIndex(text, "}")
			lastBracket := strings.LastIndex(text, "]")
			end := lastBrace
			if lastBracket > lastBrace {
				end = lastBracket
			}
			if end > start {
				candidate := text[start : end+1]
				if len(candidate) > maxLength {
					bestCandidate = candidate
				}
			}
		}
	}

	return bestCandidate
}
