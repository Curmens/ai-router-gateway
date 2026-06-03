package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/user1024/auto-router/internal/config"
	"github.com/user1024/auto-router/internal/graph"
	"github.com/user1024/auto-router/internal/logger"
	"github.com/user1024/auto-router/internal/provider"
	"github.com/user1024/auto-router/internal/router"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type SubTask struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Complexity  string   `json:"complexity"`
	Model       string   `json:"model"`
	DependsOn   []string `json:"depends_on"`
}

type Orchestrator struct {
	cfg *config.Config
}

func NewOrchestrator(cfg *config.Config) *Orchestrator {
	return &Orchestrator{cfg: cfg}
}

func (o *Orchestrator) bestPlannerProvider() (provider.ChatProvider, string, error) {
	for _, name := range []string{"subscription", "agy", "gemini", "openai", "ollama"} {
		p, err := provider.GetProvider(name)
		if err == nil {
			return p, name, nil
		}
	}
	return nil, "", errors.New("no provider available for planning")
}

func (o *Orchestrator) Decompose(ctx context.Context, requestPrompt string) ([]SubTask, error) {
	prov, provName, err := o.bestPlannerProvider()
	if err != nil {
		return nil, fmt.Errorf("planner unavailable: %w", err)
	}

	modelName := o.getFirstEnabledModel(provName)

	systemPrompt := `You are an AI Task Decomposer and Planner. Your role is to break down a high-complexity user request into a dependency tree of distinct sub-tasks that can be executed in parallel or sequence.

Assign the most appropriate "model" to each sub-task from this list of premium available models:
1. "claude-3-5-opus" or "claude-opus-4-6": Best for structural architecture design, advanced reasoning, complex planning, and demanding logic checks.
2. "claude-3-7-sonnet" or "gemini-3.1-pro": Best for core coding, complex code writing, refactoring, and logic debugging.
3. "gemini-3.5-flash" or "claude-3-5-haiku": Best for rapid execution, writing simple boilerplate, creating basic unit tests, and structural setups.

Respond ONLY with a raw JSON array matching this schema (do not wrap in markdown):
[
  {
    "id": "subtask_1",
    "name": "design",
    "description": "description of the sub-task focusing on what to generate",
    "complexity": "low|medium|high",
    "model": "model_name_from_list",
    "depends_on": []
  }
]`

	// Retrieve graph context from context project path
	graphPath := ""
	if projPath := graph.GetProjectPath(ctx); projPath != "" {
		graphPath = filepath.Join(projPath, "graphify-out", "graph.json")
	} else if o.cfg.Routing.GraphPath != "" {
		graphPath = o.cfg.Routing.GraphPath
	} else {
		graphPath = "graphify-out/graph.json"
	}

	var graphContext string
	if g, err := graph.LoadGraph(graphPath); err == nil {
		graphContext = g.QueryContext(requestPrompt)
		if graphContext != "" {
			logger.Log.Info("Injected graphify codebase context into orchestrator planner", zap.String("path", graphPath))
		}
	} else {
		logger.Log.Debug("Codebase graph not found or failed to load, skipping graph context injection", zap.Error(err))
	}

	userPrompt := fmt.Sprintf("Decompose this request:\n\n%s", requestPrompt)
	if graphContext != "" {
		userPrompt = fmt.Sprintf("Decompose this request:\n\n%s\n\n%s", requestPrompt, graphContext)
	}

	req := provider.ChatRequest{
		Model: modelName,
		Messages: []provider.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	resp, err := prov.Chat(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, errors.New("empty planner choices")
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var subtasks []SubTask
	if err := json.Unmarshal([]byte(content), &subtasks); err != nil {
		return nil, fmt.Errorf("failed to parse sub-tasks list: %w, content: %s", err, content)
	}

	return subtasks, nil
}

func (o *Orchestrator) ExecuteOrchestrated(ctx context.Context, originalReq provider.ChatRequest, requestID string) (*provider.ChatResponse, error) {
	originalPrompt := originalReq.Messages[len(originalReq.Messages)-1].Content

	subtasks, err := o.Decompose(ctx, originalPrompt)
	if err != nil {
		logger.Log.Error("Task decomposition failed, falling back to direct OpenAI execution", zap.Error(err))
		prov, chatErr := provider.GetProvider("openai")
		if chatErr != nil {
			return nil, chatErr
		}
		return prov.Chat(ctx, originalReq)
	}

	logger.Log.Info("Decomposed request into sub-tasks", zap.String("request_id", requestID), zap.Int("subtask_count", len(subtasks)))

	bb := NewBlackboard(requestID, originalPrompt)
	bb.AddLog(fmt.Sprintf("Initialized Blackboard for task with %d sub-tasks", len(subtasks)))
	_ = bb.Save(ctx)

	taskMap := make(map[string]SubTask)
	completedMap := make(map[string]bool)
	var mapMu sync.RWMutex

	for _, task := range subtasks {
		taskMap[task.ID] = task
		completedMap[task.ID] = false
	}

	g, gCtx := errgroup.WithContext(ctx)

	var dispatchWg sync.WaitGroup
	dispatchWg.Add(1)

	go func() {
		defer dispatchWg.Done()
		for {
			mapMu.Lock()
			allFinished := true
			for _, completed := range completedMap {
				if !completed {
					allFinished = false
					break
				}
			}
			mapMu.Unlock()

			if allFinished {
				return
			}

			mapMu.Lock()
			for id, task := range taskMap {
				if completedMap[id] {
					continue
				}

				depsFinished := true
				for _, depID := range task.DependsOn {
					if !completedMap[depID] {
						depsFinished = false
						break
					}
				}

				if depsFinished {
					completedMap[id] = true
					mapMu.Unlock()

					targetTask := task
					g.Go(func() error {
						taskCtx, cancel := context.WithTimeout(gCtx, 60*time.Second)
						defer cancel()
						return o.executeSubTask(taskCtx, requestID, targetTask, bb)
					})

					mapMu.Lock()
				}
			}
			mapMu.Unlock()

			select {
			case <-gCtx.Done():
				return
			case <-time.After(200 * time.Millisecond):
			}
		}
	}()

	dispatchWg.Wait()
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("orchestrated execution sub-task failure: %w", err)
	}

	bbLoaded, err := LoadBlackboard(ctx, requestID)
	if err != nil {
		bbLoaded = bb
	}

	return o.Synthesize(ctx, bbLoaded)
}

func (o *Orchestrator) executeSubTask(ctx context.Context, requestID string, task SubTask, bb *Blackboard) error {
	bb.AddLog(fmt.Sprintf("Starting sub-task: %s (%s) requested model: %s", task.ID, task.Name, task.Model))

	var primaryProv string
	var primaryModel string

	// Resolve provider from task.Model if specified and found in registry
	if task.Model != "" {
		if regModel, found := router.GetRegistryModel(task.Model); found {
			primaryProv = regModel.Provider
			primaryModel = regModel.Name
		}
	}

	// Fallback to complexity-based provider selection if model is empty or not found in registry
	if primaryProv == "" {
		switch task.Complexity {
		case "low", "medium":
			if _, err := provider.GetProvider("agy"); err == nil {
				primaryProv = "agy"
			} else {
				primaryProv = "gemini"
			}
		case "high":
			if _, err := provider.GetProvider("subscription"); err == nil {
				primaryProv = "subscription"
			} else {
				primaryProv = "gemini"
			}
		default:
			primaryProv = "gemini"
		}
		primaryModel = o.getFirstEnabledModel(primaryProv)
	}

	// Build a fallback chain to try other premium and standard providers on failures
	fallbackChain := []string{primaryProv, "subscription", "agy", "gemini"}
	seen := map[string]bool{}

	var resp *provider.ChatResponse
	var lastErr error
	var executedProv string

	for _, candidate := range fallbackChain {
		if seen[candidate] {
			continue
		}
		seen[candidate] = true

		prov, err := provider.GetProvider(candidate)
		if err != nil {
			continue
		}

		// For fallback providers, use their default first enabled model; for the primary, use the resolved model name
		candidateModel := primaryModel
		if candidate != primaryProv {
			candidateModel = o.getFirstEnabledModel(candidate)
		}

		req := provider.ChatRequest{
			Model: candidateModel,
			Messages: []provider.ChatMessage{
				{Role: "system", Content: fmt.Sprintf("Perform this specific sub-task and return the result. Focus only on this part:\n%s", task.Description)},
				{Role: "user", Content: bb.OriginalPrompt},
			},
		}

		bb.AddLog(fmt.Sprintf("Dispatching task %s to provider %s model %s", task.ID, candidate, candidateModel))
		resp, lastErr = prov.Chat(ctx, req)
		if lastErr == nil {
			executedProv = candidate
			bb.AddLog(fmt.Sprintf("Task %s succeeded on provider %s model %s", task.ID, executedProv, candidateModel))
			break
		}
		bb.AddLog(fmt.Sprintf("Provider %s model %s failed for task %s, trying fallback: %v", candidate, candidateModel, task.ID, lastErr))
	}

	if lastErr != nil || resp == nil {
		return fmt.Errorf("sub-task %s failed across all fallback providers: %w", task.ID, lastErr)
	}

	if len(resp.Choices) == 0 {
		return fmt.Errorf("sub-task %s returned empty choice", task.ID)
	}

	draft := resp.Choices[0].Message.Content
	_ = bb.SetDraft(ctx, task.ID, draft)

	bb.AddLog(fmt.Sprintf("Completed sub-task: %s", task.ID))

	GlobalBus.Publish(Event{
		RequestID: requestID,
		AgentName: executedProv + "-agent",
		Topic:     "subtask_completed",
		Payload:   fmt.Sprintf("Task %s completed", task.ID),
	})

	return nil
}

func (o *Orchestrator) getFirstEnabledModel(prov string) string {
	switch prov {
	case "openai":
		if len(o.cfg.Providers.OpenAI.Models) > 0 {
			return o.cfg.Providers.OpenAI.Models[0].Name
		}
	case "gemini":
		if len(o.cfg.Providers.Gemini.Models) > 0 {
			return o.cfg.Providers.Gemini.Models[0].Name
		}
	case "ollama":
		if len(o.cfg.Providers.Ollama.Models) > 0 {
			return o.cfg.Providers.Ollama.Models[0].Name
		}
	case "subscription":
		if len(o.cfg.Providers.Subscription.Models) > 0 {
			return o.cfg.Providers.Subscription.Models[0].Name
		}
	case "agy":
		if len(o.cfg.Providers.Agy.Models) > 0 {
			return o.cfg.Providers.Agy.Models[0].Name
		}
	}
	return o.cfg.Routing.DefaultModel
}

func (o *Orchestrator) Synthesize(ctx context.Context, bb *Blackboard) (*provider.ChatResponse, error) {
	bb.AddLog("Synthesizing final solution from blackboard drafts...")

	prov, provName, err := o.bestPlannerProvider()
	if err != nil {
		return nil, fmt.Errorf("synthesizer unavailable: %w", err)
	}

	modelName := o.getFirstEnabledModel(provName)

	var draftsBuffer strings.Builder
	for id, draft := range bb.TaskDrafts {
		draftsBuffer.WriteString(fmt.Sprintf("### Output of Sub-Task %s:\n%s\n\n", id, draft))
	}

	systemPrompt := `You are an Expert Solution Synthesizer. Your role is to merge multiple structural, independent drafts generated by sub-tasks into a single coherent, production-grade comprehensive solution.
Audits the sections for code duplication, inconsistencies, or gaps and consolidate them beautifully.
Deliver the final completed answer.`

	req := provider.ChatRequest{
		Model: modelName,
		Messages: []provider.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: fmt.Sprintf("Original Request:\n%s\n\nCompleted Sub-Task Drafts:\n%s", bb.OriginalPrompt, draftsBuffer.String())},
		},
	}

	start := time.Now()
	resp, err := prov.Chat(ctx, req)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		return nil, fmt.Errorf("synthesis failed: %w", err)
	}

	bb.AddLog(fmt.Sprintf("Final solution synthesized successfully in %d ms", duration))
	return resp, nil
}
