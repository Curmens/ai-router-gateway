package token

import (
	"math"
	"strings"

	"github.com/user1024/auto-router/internal/provider"
)

func EstimateTextTokens(text string) int {
	if len(text) == 0 {
		return 0
	}

	charCount := float64(len(text))
	wordCount := float64(len(strings.Fields(text)))

	tokensByChars := charCount / 4.0
	tokensByWords := wordCount / 0.75

	est := math.Max(tokensByChars, tokensByWords)
	return int(math.Ceil(est))
}

func EstimateRequestTokens(messages []provider.ChatMessage) int {
	tokens := 0
	for _, msg := range messages {
		tokens += 4
		tokens += EstimateTextTokens(msg.Content)
		tokens += EstimateTextTokens(msg.Role)
		if msg.Name != "" {
			tokens += EstimateTextTokens(msg.Name)
			tokens += 1
		}
	}
	tokens += 3
	return tokens
}
