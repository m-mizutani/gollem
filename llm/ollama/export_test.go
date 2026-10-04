package ollama

import (
	"encoding/json"

	"github.com/gollem-dev/gollem"
)

// TestIssuer is the issuer HistoryRoundTrip and HistoryFromWire record and
// send to.
var TestIssuer = gollem.Issuer{Provider: gollem.LLMTypeOllama, Model: "test-model"}

// HistoryRoundTrip converts h to the Ollama format and back, returning the
// intermediate messages as JSON so tests can inspect the wire format.
func HistoryRoundTrip(h *gollem.History) (json.RawMessage, *gollem.History, error) {
	return HistoryRoundTripWith(h, TestIssuer, TestIssuer)
}

// HistoryRoundTripWith converts h to the Ollama format to send to dest, and
// back to a gollem.History recorded as issued by issuer.
func HistoryRoundTripWith(h *gollem.History, dest, issuer gollem.Issuer) (json.RawMessage, *gollem.History, error) {
	messages, err := toMessages(h, dest)
	if err != nil {
		return nil, nil, err
	}
	data, err := json.Marshal(messages)
	if err != nil {
		return nil, nil, err
	}
	back, err := newHistory(messages, issuer)
	if err != nil {
		return nil, nil, err
	}
	return data, back, nil
}

// HistoryFromWire decodes Ollama messages given as JSON into a gollem.History.
func HistoryFromWire(data []byte) (*gollem.History, error) {
	var messages []message
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}
	return newHistory(messages, TestIssuer)
}

// ToolJSON returns the Ollama tool definition of t as JSON.
func ToolJSON(t gollem.Tool) (json.RawMessage, error) {
	return json.Marshal(convertTool(t))
}

// ResponseFormat exposes responseFormat for testing.
var ResponseFormat = responseFormat
