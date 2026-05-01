package gollem

import "testing"

func TestMessageContentTypeThinking(t *testing.T) {
	ct := MessageContentTypeThinking
	if ct != "thinking" {
		t.Errorf("Expected 'thinking', got '%s'", ct)
	}
}
