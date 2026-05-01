package gollem

import "testing"

func TestMessageContentTypeThinking(t *testing.T) {
	ct := MessageContentTypeThinking
	if ct != "thinking" {
		t.Errorf("Expected 'thinking', got '%s'", ct)
	}
}

func TestThinkingContent(t *testing.T) {
	content := ThinkingContent{Text: "Let me think..."}
	if content.Text != "Let me think..." {
		t.Errorf("Expected 'Let me think...', got '%s'", content.Text)
	}
}

func TestNewThinkingContent(t *testing.T) {
	mc, err := NewThinkingContent("test thinking")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if mc.Type != MessageContentTypeThinking {
		t.Errorf("Expected type 'thinking', got '%s'", mc.Type)
	}
}
