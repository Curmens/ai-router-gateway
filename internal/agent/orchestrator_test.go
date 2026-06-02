package agent

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/user1024/auto-router/internal/cache"
	"github.com/user1024/auto-router/internal/config"
	"github.com/user1024/auto-router/internal/logger"
	"github.com/user1024/auto-router/internal/provider"
	"github.com/user1024/auto-router/internal/router"
	"go.uber.org/zap"
)

// MockProvider implements ChatProvider for testing
type MockProvider struct {
	name string
}

func (m *MockProvider) Name() string { return m.name }
func (m *MockProvider) Chat(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	return &provider.ChatResponse{
		ID:      "mock-id",
		Object:  "chat.completion",
		Created: 123456789,
		Model:   req.Model,
		Choices: []provider.ChatChoice{
			{Index: 0, Message: provider.ChatMessage{Role: "assistant", Content: "mock-result"}, FinishReason: "stop"},
		},
	}, nil
}
func (m *MockProvider) Stream(ctx context.Context, req provider.ChatRequest, sseChan chan<- string) error {
	return nil
}
func (m *MockProvider) Models(ctx context.Context) ([]provider.Model, error) {
	return nil, nil
}

func TestExecuteSubTaskRegistryRouting(t *testing.T) {
	// Initialize custom configuration for test
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			Agy: config.AgyConfig{
				Enabled: true,
				Models: []config.ModelConfig{
					{Name: "gemini-3.5-flash"},
					{Name: "gemini-3.1-pro"},
				},
			},
			Subscription: config.ClaudeConfig{
				Enabled: true,
				Models: []config.ModelConfig{
					{Name: "claude-3-7-sonnet"},
				},
			},
		},
	}

	// Register the models into the router registry
	router.InitRegistry(cfg)

	// Set up mock provider mappings in the global registry
	provider.Registry["agy"] = &MockProvider{name: "agy"}
	provider.Registry["subscription"] = &MockProvider{name: "subscription"}

	// Set up cache Client to a non-nil dummy to prevent nil pointer panics on SetDraft
	cache.Client = redis.NewClient(&redis.Options{Addr: "localhost:16379"})

	// Set up logger Log to a no-op logger to prevent SEGV nil pointer panic
	logger.Log = zap.NewNop()

	orch := NewOrchestrator(cfg)
	bb := NewBlackboard("test-req-id", "mock original prompt")

	tests := []struct {
		name          string
		taskModel     string
		complexity    string
		expectedModel string
	}{
		{
			name:          "Explicit model routing to agy provider",
			taskModel:     "gemini-3.1-pro",
			complexity:    "medium",
			expectedModel: "gemini-3.1-pro",
		},
		{
			name:          "Explicit model routing to subscription provider",
			taskModel:     "claude-3-7-sonnet",
			complexity:    "high",
			expectedModel: "claude-3-7-sonnet",
		},
		{
			name:          "Empty model fallback to high complexity subscription default",
			taskModel:     "",
			complexity:    "high",
			expectedModel: "claude-3-7-sonnet",
		},
		{
			name:          "Empty model fallback to low complexity agy default",
			taskModel:     "",
			complexity:    "low",
			expectedModel: "gemini-3.5-flash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := SubTask{
				ID:          "task-1",
				Name:        "test-task",
				Description: "test task execution description",
				Complexity:  tt.complexity,
				Model:       tt.taskModel,
			}

			err := orch.executeSubTask(context.Background(), "test-req-id", task, bb)
			if err != nil {
				t.Fatalf("executeSubTask failed: %v", err)
			}

			// Verify that the task draft was correctly written to the blackboard map
			draft, exists := bb.TaskDrafts["task-1"]
			if !exists {
				t.Fatalf("expected draft to exist in blackboard for task-1")
			}

			if draft != "mock-result" {
				t.Errorf("expected draft content %q, got %q", "mock-result", draft)
			}
		})
	}
}
