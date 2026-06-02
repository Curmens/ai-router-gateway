package token

import (
	"testing"

	"github.com/user1024/auto-router/internal/provider"
)

func TestEstimateTextTokens(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{"empty string", "", 0},
		{"short word", "hello", 2},
		{"sentence", "Hello world, this is a test prompt.", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTextTokens(tt.text)
			if got != tt.expected {
				t.Errorf("EstimateTextTokens(%q) = %d; want %d", tt.text, got, tt.expected)
			}
		})
	}
}

func TestEstimateRequestTokens(t *testing.T) {
	messages := []provider.ChatMessage{
		{Role: "user", Content: "Hello world!"},
		{Role: "assistant", Content: "Hello! How can I help you?"},
	}

	got := EstimateRequestTokens(messages)
	if got < 20 {
		t.Errorf("EstimateRequestTokens returned too low value: %d", got)
	}
}
