// Package gollem provides a unified interface for interacting with various LLM services.
package gollem

import (
	"encoding/json"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/m-mizutani/goerr/v2"
	"github.com/sashabaranov/go-openai"
	"google.golang.org/genai"
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

func (x *History) ToGemini() ([]*genai.Content, error) {
	if x.Version != HistoryVersion {
		return nil, goerr.Wrap(ErrHistoryVersionMismatch, "unsupported history version", goerr.V("expected", HistoryVersion), goerr.V("actual", x.Version))
	}
	if len(x.Messages) == 0 {
		return []*genai.Content{}, nil
	}
	return convertMessagesToGemini(x.Messages)
}

func (x *History) ToClaude() ([]anthropic.MessageParam, error) {
	if x.Version != HistoryVersion {
		return nil, goerr.Wrap(ErrHistoryVersionMismatch, "unsupported history version", goerr.V("expected", HistoryVersion), goerr.V("actual", x.Version))
	}
	if len(x.Messages) == 0 {
		return []anthropic.MessageParam{}, nil
	}
	return convertMessagesToClaude(x.Messages)
}

func (x *History) ToOpenAI() ([]openai.ChatCompletionMessage, error) {
	if x.Version != HistoryVersion {
		return nil, goerr.Wrap(ErrHistoryVersionMismatch, "unsupported history version", goerr.V("expected", HistoryVersion), goerr.V("actual", x.Version))
	}
	if len(x.Messages) == 0 {
		return []openai.ChatCompletionMessage{}, nil
	}
	return convertMessagesToOpenAI(x.Messages)
}

func NewHistoryFromOpenAI(messages []openai.ChatCompletionMessage) (*History, error) {
	// Convert to common format
	commonMessages, err := convertOpenAIToMessages(messages)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to convert OpenAI messages to common format")
	}

	return &History{
		LLType:   LLMTypeOpenAI,
		Version:  HistoryVersion,
		Messages: commonMessages,
		Metadata: &HistoryMetadata{
			OriginalProvider: LLMTypeOpenAI,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		},
	}, nil
}

func NewHistoryFromClaude(messages []anthropic.MessageParam) (*History, error) {
	// Convert to common format
	commonMessages, err := convertClaudeToMessages(messages)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to convert Claude messages to common format")
	}

	return &History{
		LLType:   LLMTypeClaude,
		Version:  HistoryVersion,
		Messages: commonMessages,
		Metadata: &HistoryMetadata{
			OriginalProvider: LLMTypeClaude,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		},
	}, nil
}

func NewHistoryFromGemini(messages []*genai.Content) (*History, error) {
	// Convert to common format
	commonMessages, err := convertGeminiToMessages(messages)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to convert Gemini messages to common format")
	}

	return &History{
		LLType:   LLMTypeGemini,
		Version:  HistoryVersion,
		Messages: commonMessages,
		Metadata: &HistoryMetadata{
			OriginalProvider: LLMTypeGemini,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		},
	}, nil
}
