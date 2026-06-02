package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/user1024/auto-router/internal/config"
)

type ClaudeProvider struct {
	cfg config.ClaudeConfig
}

func NewClaudeProvider(cfg config.ClaudeConfig) *ClaudeProvider {
	return &ClaudeProvider{cfg: cfg}
}

func (p *ClaudeProvider) Name() string {
	return "subscription"
}

type claudeCLIUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type claudeCLIResult struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype"`
	Result  string          `json:"result"`
	IsError bool            `json:"is_error"`
	CostUSD float64         `json:"total_cost_usd"`
	Usage   claudeCLIUsage  `json:"usage"`
}

func (p *ClaudeProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	userPrompt, systemPrompt := extractPrompts(req)

	binary := p.cfg.BinaryPath
	if binary == "" {
		binary = "claude"
	}

	args := []string{
		"-p", userPrompt,
		"--output-format", "json",
		"--model", req.Model,
	}
	if systemPrompt != "" {
		args = append(args, "--system-prompt", systemPrompt)
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("claude cli error: %w — stderr: %s", err, stderr.String())
	}

	var result claudeCLIResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse claude output: %w — raw: %s", err, stdout.String())
	}
	if result.IsError {
		return nil, fmt.Errorf("claude returned error: %s", result.Result)
	}

	return &ChatResponse{
		ID:      fmt.Sprintf("claude-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []ChatChoice{
			{
				Index:        0,
				Message:      ChatMessage{Role: "assistant", Content: result.Result},
				FinishReason: "stop",
			},
		},
		Usage: ChatUsage{
			PromptTokens:     result.Usage.InputTokens,
			CompletionTokens: result.Usage.OutputTokens,
			TotalTokens:      result.Usage.InputTokens + result.Usage.OutputTokens,
		},
	}, nil
}

func (p *ClaudeProvider) Stream(ctx context.Context, req ChatRequest, sseChan chan<- string) error {
	userPrompt, systemPrompt := extractPrompts(req)

	binary := p.cfg.BinaryPath
	if binary == "" {
		binary = "claude"
	}

	args := []string{
		"-p", userPrompt,
		"--output-format", "stream-json",
		"--model", req.Model,
	}
	if systemPrompt != "" {
		args = append(args, "--system-prompt", systemPrompt)
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start claude cli: %w", err)
	}

	reqID := fmt.Sprintf("claude-%d", time.Now().UnixNano())
	decoder := json.NewDecoder(stdout)

	for decoder.More() {
		var event map[string]any
		if err := decoder.Decode(&event); err != nil {
			break
		}

		evType, _ := event["type"].(string)

		// assistant content block delta → emit as SSE chunk
		if evType == "assistant" {
			if msg, ok := event["message"].(map[string]any); ok {
				if content, ok := msg["content"].(string); ok && content != "" {
					chunk := ChatResponseStream{
						ID:    reqID,
						Model: req.Model,
						Choices: []ChatChoiceStream{
							{Delta: ChatMessage{Role: "assistant", Content: content}},
						},
					}
					b, _ := json.Marshal(chunk)
					sseChan <- "data: " + string(b)
				}
			}
		}

		// result event → final token, emit stop chunk then DONE
		if evType == "result" {
			chunk := ChatResponseStream{
				ID:    reqID,
				Model: req.Model,
				Choices: []ChatChoiceStream{
					{FinishReason: "stop"},
				},
			}
			b, _ := json.Marshal(chunk)
			sseChan <- "data: " + string(b)
			sseChan <- "data: [DONE]"
			break
		}
	}

	if err := cmd.Wait(); err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText != "" {
			return fmt.Errorf("claude cli exited with error: %s", errText)
		}
	}
	return nil
}

func (p *ClaudeProvider) Models(ctx context.Context) ([]Model, error) {
	models := make([]Model, len(p.cfg.Models))
	for i, m := range p.cfg.Models {
		models[i] = Model{
			ID:      m.Name,
			Object:  "model",
			Created: time.Now().Unix(),
			OwnedBy: "subscription",
		}
	}
	return models, nil
}

func extractPrompts(req ChatRequest) (userPrompt, systemPrompt string) {
	var userParts []string
	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			systemPrompt = m.Content
		case "user":
			userParts = append(userParts, m.Content)
		case "assistant":
			userParts = append(userParts, "Assistant: "+m.Content)
		}
	}
	userPrompt = strings.Join(userParts, "\n\n")
	return
}
