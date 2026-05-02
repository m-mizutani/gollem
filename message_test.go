package gollem

import "testing"

func TestMessageContentTypeReasoning(t *testing.T) {
	ct := MessageContentTypeReasoning
	if ct != "reasoning" {
		t.Errorf("Expected 'reasoning', got '%s'", ct)
	}
}

func TestThinkingContent(t *testing.T) {
	content := ReasoningContent{Text: "Let me think..."}
	if content.Text != "Let me think..." {
		t.Errorf("Expected 'Let me think...', got '%s'", content.Text)
	}
}

func TestNewThinkingContent(t *testing.T) {
	mc, err := NewReasoningContent("test thinking")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if mc.Type != MessageContentTypeReasoning {
		t.Errorf("Expected type 'reasoning', got '%s'", mc.Type)
	}
}

func TestGetReasoningContent(t *testing.T) {
	mc, _ := NewReasoningContent("test thinking")
	tc, err := mc.GetReasoningContent()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if tc.Text != "test thinking" {
		t.Errorf("Expected 'test thinking', got '%s'", tc.Text)
	}
}
