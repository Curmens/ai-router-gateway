package router

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/user1024/auto-router/internal/config"
	"github.com/user1024/auto-router/internal/logger"
	"github.com/user1024/auto-router/internal/provider"
	"github.com/user1024/auto-router/pkg/token"
	"go.uber.org/zap"
)

type Classification struct {
	Complexity      string  `json:"complexity"`
	Confidence      float64 `json:"confidence"`
	EstimatedTokens int     `json:"estimated_tokens"`
	Reason          string  `json:"reason"`
}

type Classifier struct {
	cfg config.OllamaConfig
}

func NewClassifier(cfg config.OllamaConfig) *Classifier {
	return &Classifier{cfg: cfg}
}

func (c *Classifier) Classify(ctx context.Context, messages []provider.ChatMessage) Classification {
	if len(messages) == 0 {
		return Classification{Complexity: "low", Confidence: 1.0, EstimatedTokens: 0, Reason: "Empty prompt"}
	}

	lastMessage := messages[len(messages)-1].Content
	estimatedInputTokens := token.EstimateRequestTokens(messages)

	static := c.classifyStatic(lastMessage, estimatedInputTokens)

	// Skip Ollama when static rules are already confident or clearly decisive
	if static.Complexity != "medium" || static.Confidence >= 0.85 {
		return static
	}

	// Only call Ollama for genuinely ambiguous medium cases
	if c.cfg.Enabled {
		ollamaClient, err := provider.GetProvider("ollama")
		if err == nil {
			class, err := c.classifyWithOllama(ctx, ollamaClient, lastMessage, estimatedInputTokens)
			if err == nil {
				return class
			}
			logger.Log.Warn("Dynamic Ollama classification failed, using fallback static rules", zap.Error(err))
		}
	}

	return static
}

func (c *Classifier) classifyWithOllama(ctx context.Context, ollama provider.ChatProvider, prompt string, inputTokens int) (Classification, error) {
	systemPrompt := `You are a prompt complexity classifier. Analyze the user's prompt and respond ONLY with a raw JSON object in this precise format (do not wrap in markdown block, do not write anything else):
{"complexity": "low|medium|high", "confidence": 0.95, "reason": "brief reason why"}

Guidance:
- "low": Simple conversations, brief FAQs, data extraction, word translations, greeting, basic categorization.
- "medium": In-depth analysis, reading summaries, logic comparisons, creative writing, multi-paragraph reasoning.
- "high": Software engineering, system architecture, database design, heavy mathematics, planning, multi-step code debugging.`

	req := provider.ChatRequest{
		Model: c.cfg.ClassifierModel,
		Messages: []provider.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: fmt.Sprintf("Classify this prompt:\n\n%s", prompt)},
		},
	}

	classifyCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	resp, err := ollama.Chat(classifyCtx, req)
	if err != nil {
		return Classification{}, err
	}

	if len(resp.Choices) == 0 {
		return Classification{}, fmt.Errorf("empty choices from classifier")
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var class Classification
	if err := json.Unmarshal([]byte(content), &class); err != nil {
		return Classification{}, fmt.Errorf("failed to unmarshal classifier JSON: %w, content: %s", err, content)
	}

	class.Complexity = strings.ToLower(class.Complexity)
	if class.Complexity != "low" && class.Complexity != "medium" && class.Complexity != "high" {
		class.Complexity = "medium"
	}
	class.EstimatedTokens = inputTokens
	return class, nil
}

var (
	highComplexityRegex   = regexp.MustCompile(`(?i)(architecture|system design|kubernetes|docker|sql injection|refactor|compile|debug|algorithm|complexity|theorem|proof|optimize|database schema|postgres|redis|microservice|concurrency|mutex|race condition|deadlock)`)
	mediumComplexityRegex = regexp.MustCompile(`(?i)(analyze|compare|summarize|critique|review|essay|translate|draft|pros and cons|historical|difference between)`)
)

func (c *Classifier) classifyStatic(prompt string, inputTokens int) Classification {
	length := len(prompt)

	if strings.Contains(prompt, "```") || highComplexityRegex.MatchString(prompt) || length > 2000 {
		return Classification{
			Complexity:      "high",
			Confidence:      0.80,
			EstimatedTokens: inputTokens,
			Reason:          "Static rules: detected code structure, complex engineering keywords, or long prompt length.",
		}
	}

	if mediumComplexityRegex.MatchString(prompt) || length > 500 {
		return Classification{
			Complexity:      "medium",
			Confidence:      0.75,
			EstimatedTokens: inputTokens,
			Reason:          "Static rules: detected comparative/analysis terms or medium prompt length.",
		}
	}

	return Classification{
		Complexity:      "low",
		Confidence:      0.90,
		EstimatedTokens: inputTokens,
		Reason:          "Static rules: short, direct prompt with no complex triggers.",
	}
}
