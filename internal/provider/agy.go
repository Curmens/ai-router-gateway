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

type AgyProvider struct {
	cfg config.AgyConfig
}

func NewAgyProvider(cfg config.AgyConfig) *AgyProvider {
	return &AgyProvider{cfg: cfg}
}

func (p *AgyProvider) Name() string {
	return "agy"
}

func (p *AgyProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	prompt := p.buildPrompt(req)

	binary := p.cfg.BinaryPath
	if binary == "" {
		binary = "agy"
	}

	cmd := exec.CommandContext(ctx, binary, "-p", prompt, "--dangerously-skip-permissions")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("agy cli error: %w — stderr: %s", err, stderr.String())
	}

	content := strings.TrimSpace(stdout.String())
	if content == "" {
		return nil, fmt.Errorf("agy returned empty response")
	}

	return &ChatResponse{
		ID:      fmt.Sprintf("agy-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []ChatChoice{
			{
				Index:        0,
				Message:      ChatMessage{Role: "assistant", Content: content},
				FinishReason: "stop",
			},
		},
	}, nil
}

func (p *AgyProvider) Stream(ctx context.Context, req ChatRequest, sseChan chan<- string) error {
	resp, err := p.Chat(ctx, req)
	if err != nil {
		return err
	}

	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	chunk := ChatResponseStream{
		ID:    resp.ID,
		Model: resp.Model,
		Choices: []ChatChoiceStream{
			{Delta: ChatMessage{Role: "assistant", Content: content}},
		},
	}
	b, _ := json.Marshal(chunk)
	sseChan <- "data: " + string(b)
	sseChan <- "data: [DONE]"
	return nil
}

func (p *AgyProvider) Models(ctx context.Context) ([]Model, error) {
	models := make([]Model, len(p.cfg.Models))
	for i, m := range p.cfg.Models {
		models[i] = Model{
			ID:      m.Name,
			Object:  "model",
			Created: time.Now().Unix(),
			OwnedBy: "agy",
		}
	}
	return models, nil
}

func (p *AgyProvider) buildPrompt(req ChatRequest) string {
	var parts []string
	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			parts = append([]string{"[System: " + m.Content + "]"}, parts...)
		case "user":
			parts = append(parts, m.Content)
		case "assistant":
			parts = append(parts, "Assistant: "+m.Content)
		}
	}
	return strings.Join(parts, "\n\n")
}
