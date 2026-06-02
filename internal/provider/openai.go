package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/user1024/auto-router/internal/config"
	"github.com/user1024/auto-router/internal/logger"
	"go.uber.org/zap"
)

type OpenAIProvider struct {
	cfg config.OpenAIConfig
}

// responsesRequest is the body shape for the /v1/responses endpoint (codex models).
type responsesRequest struct {
	Model           string             `json:"model"`
	Instructions    string             `json:"instructions,omitempty"`
	Input           []responsesMessage `json:"input"`
	MaxOutputTokens *int               `json:"max_output_tokens,omitempty"`
}

type responsesMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responsesContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type responsesOutputItem struct {
	Type    string             `json:"type"`
	Content []responsesContent `json:"content"`
}

type responsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type responsesAPIResponse struct {
	ID     string                `json:"id"`
	Model  string                `json:"model"`
	Output []responsesOutputItem `json:"output"`
	Usage  responsesUsage        `json:"usage"`
}

func isCodexModel(model string) bool {
	return strings.Contains(model, "codex")
}

func NewOpenAIProvider(cfg config.OpenAIConfig) *OpenAIProvider {
	return &OpenAIProvider{cfg: cfg}
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) chatViaResponses(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	var instructions string
	var inputMessages []responsesMessage
	for _, m := range req.Messages {
		if m.Role == "system" {
			instructions = m.Content
		} else {
			inputMessages = append(inputMessages, responsesMessage{Role: m.Role, Content: m.Content})
		}
	}

	rReq := responsesRequest{
		Model:           req.Model,
		Instructions:    instructions,
		Input:           inputMessages,
		MaxOutputTokens: req.MaxTokens,
	}

	reqBody, err := json.Marshal(rReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal responses request: %w", err)
	}

	url := fmt.Sprintf("%s/responses", p.cfg.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.cfg.APIKey))

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai responses error (status %d): %s", resp.StatusCode, string(body))
	}

	var rResp responsesAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&rResp); err != nil {
		return nil, fmt.Errorf("failed to decode responses response: %w", err)
	}

	var content string
	for _, item := range rResp.Output {
		if item.Type == "message" {
			for _, c := range item.Content {
				if c.Type == "output_text" {
					content += c.Text
				}
			}
		}
	}
	if content == "" {
		return nil, fmt.Errorf("model %s returned empty content via /v1/responses", req.Model)
	}

	return &ChatResponse{
		ID:    rResp.ID,
		Model: rResp.Model,
		Choices: []ChatChoice{
			{
				Index:        0,
				Message:      ChatMessage{Role: "assistant", Content: content},
				FinishReason: "stop",
			},
		},
		Usage: ChatUsage{
			PromptTokens:     rResp.Usage.InputTokens,
			CompletionTokens: rResp.Usage.OutputTokens,
			TotalTokens:      rResp.Usage.InputTokens + rResp.Usage.OutputTokens,
		},
	}, nil
}

func (p *OpenAIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if isCodexModel(req.Model) {
		return p.chatViaResponses(ctx, req)
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", p.cfg.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.cfg.APIKey))

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai error (status %d): %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &chatResp, nil
}

func (p *OpenAIProvider) Stream(ctx context.Context, req ChatRequest, sseChan chan<- string) error {
	if isCodexModel(req.Model) {
		resp, err := p.chatViaResponses(ctx, req)
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
		chunkBytes, err := json.Marshal(chunk)
		if err != nil {
			return fmt.Errorf("failed to marshal codex stream chunk: %w", err)
		}
		sseChan <- "data: " + string(chunkBytes)
		sseChan <- "data: [DONE]"
		return nil
	}

	req.Stream = true
	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", p.cfg.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.cfg.APIKey))
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("Connection", "keep-alive")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http stream request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("openai stream error (status %d): %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		sseChan <- line
		if strings.HasSuffix(line, "[DONE]") {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		logger.Log.Error("OpenAI stream scan error", zap.Error(err))
		return err
	}

	return nil
}

func (p *OpenAIProvider) Models(ctx context.Context) ([]Model, error) {
	models := make([]Model, len(p.cfg.Models))
	for i, m := range p.cfg.Models {
		models[i] = Model{
			ID:      m.Name,
			Object:  "model",
			Created: time.Now().Unix(),
			OwnedBy: "openai",
		}
	}
	return models, nil
}
