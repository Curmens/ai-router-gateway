package router

import (
	"context"
	"testing"

	"github.com/user1024/auto-router/internal/config"
	"github.com/user1024/auto-router/internal/provider"
)

func TestClassifierStatic(t *testing.T) {
	c := NewClassifier(config.OllamaConfig{Enabled: false})

	tests := []struct {
		name     string
		prompt   string
		expected string
	}{
		{"Low Complexity Greeting", "Hello! How are you doing today?", "low"},
		{"Medium Complexity Summarization", "Analyze and summarize the key differences between SQL and NoSQL databases.", "medium"},
		{"High Complexity Coding", "Write a thread-safe connection pool in Go using channels and mutex locks.", "high"},
		{"High Complexity Code Blocks", "Here is some code:\n```go\nfunc main() {}\n```", "high"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.Classify(context.Background(), []provider.ChatMessage{
				{Role: "user", Content: tt.prompt},
			})
			if got.Complexity != tt.expected {
				t.Errorf("Classify(%q) complexity = %q; want %q", tt.prompt, got.Complexity, tt.expected)
			}
		})
	}
}
