package runtime

import (
	"testing"

	"github.com/yinhe/devclaw/internal/provider"
)

func TestEstimateTokens(t *testing.T) {
	msgs := []provider.ChatMessage{
		{Role: "user", Content: "Hello world"},       // ~7 chars → ~1+4 = 5
		{Role: "assistant", Content: "Hi there back"}, // ~13 chars → ~3+4 = 7
	}
	tokens := EstimateTokens(msgs)
	if tokens <= 0 {
		t.Fatalf("expected positive tokens, got %d", tokens)
	}
}

func TestShouldCompress(t *testing.T) {
	// Small messages should not need compression
	small := []provider.ChatMessage{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}
	if ShouldCompress(small, 32000) {
		t.Fatal("small messages should not trigger compression")
	}

	// Large messages should trigger compression
	bigContent := make([]byte, 100000)
	for i := range bigContent {
		bigContent[i] = 'a'
	}
	big := []provider.ChatMessage{
		{Role: "user", Content: string(bigContent)},
	}
	if !ShouldCompress(big, 32000) {
		t.Fatal("large messages should trigger compression")
	}
}

func TestSimpleCompress(t *testing.T) {
	msgs := make([]provider.ChatMessage, 10)
	for i := range msgs {
		msgs[i] = provider.ChatMessage{Role: "user", Content: "msg"}
	}
	result := simpleCompress(msgs, 4)
	// Should have: 1 summary + 1 ack + 4 recent = 6
	if len(result) != 6 {
		t.Fatalf("expected 6 messages, got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Fatalf("expected user summary, got %s", result[0].Role)
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`no json here`, ""},
		{`{"name":"test"}`, `{"name":"test"}`},
		{"```json\n{\"key\":\"val\"}\n```", `{"key":"val"}`},
		{`some text {"a":1} more`, `{"a":1}`},
	}
	for _, tt := range tests {
		got := extractJSON(tt.input)
		if got != tt.want {
			t.Errorf("extractJSON(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
