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
	HistoryVersion = 2 // Unified format version
)

type History struct {
	LLType  LLMType `json:"type"`
	Version int     `json:"version"`

	// Unified format fields
	Messages []Message        `json:"messages"`
	Metadata *HistoryMetadata `json:"metadata,omitempty"`

	// Compaction related fields
	Summary     string `json:"summary,omitempty"`      // Summary information
	Compacted   bool   `json:"compacted,omitempty"`    // Compaction flag
	OriginalLen int    `json:"original_len,omitempty"` // Original length
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
