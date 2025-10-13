// Package gollem provides a unified interface for interacting with various LLM services.
package gollem

import (
	"encoding/json"
)

// History represents a conversation history that can be used across different LLM sessions.
// It stores messages in a format specific to each LLM type (OpenAI, Claude, or Gemini).
//
// For detailed documentation, see doc/history.md
type LLMType string

const (
	LLMTypeOpenAI LLMType = "OpenAI"
	LLMTypeGemini LLMType = "gemini"
	LLMTypeClaude LLMType = "claude"
)

const (
	HistoryVersion = 3 // Unified format version (v3: removed legacy function calls and provider dialects)
)

type History struct {
	LLType   LLMType   `json:"type"`
	Version  int       `json:"version"`
	Messages []Message `json:"messages"`
}

func (x *History) ToCount() int {
	if x == nil {
		return 0
	}
	return len(x.Messages)
}

func (x *History) Clone() *History {
	if x == nil {
		return nil
	}

	// Use JSON marshal/unmarshal for deep copy to avoid field-specific code
	// This ensures all fields are copied correctly even when structs are modified
	data, err := json.Marshal(x)
	if err != nil {
		// If marshaling fails, return a basic clone with empty messages
		// This should not happen in practice as History is designed to be JSON-serializable
		return &History{
			LLType:  x.LLType,
			Version: x.Version,
		}
	}

	var clone History
	if err := json.Unmarshal(data, &clone); err != nil {
		// If unmarshaling fails, return a basic clone with empty messages
		return &History{
			LLType:  x.LLType,
			Version: x.Version,
		}
	}

	return &clone
}
