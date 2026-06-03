package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/user1024/auto-router/internal/cache"
	"github.com/user1024/auto-router/internal/logger"
	"go.uber.org/zap"
)

type Blackboard struct {
	RequestID        string            `json:"request_id"`
	OriginalPrompt   string            `json:"original_prompt"`
	TaskDrafts       map[string]string `json:"task_drafts"`
	ConversationTurn int               `json:"conversation_turn"`
	Logs             []string          `json:"logs"`
}

func NewBlackboard(requestID string, originalPrompt string) *Blackboard {
	return &Blackboard{
		RequestID:      requestID,
		OriginalPrompt: originalPrompt,
		TaskDrafts:     make(map[string]string),
		Logs:           make([]string, 0),
	}
}

func LoadBlackboard(ctx context.Context, requestID string) (*Blackboard, error) {
	key := "blackboard:" + requestID
	data, err := cache.LoadBlob(ctx, key)
	if err != nil {
		return nil, err
	}

	var bb Blackboard
	if err := json.Unmarshal(data, &bb); err != nil {
		return nil, err
	}
	return &bb, nil
}

func (bb *Blackboard) Save(ctx context.Context) error {
	key := "blackboard:" + bb.RequestID
	data, err := json.Marshal(bb)
	if err != nil {
		return err
	}
	return cache.StoreBlob(ctx, key, data, time.Hour)
}

func (bb *Blackboard) SetDraft(ctx context.Context, taskID string, draft string) error {
	bb.TaskDrafts[taskID] = draft
	bb.AddLog(fmt.Sprintf("Draft updated for task %s", taskID))
	return bb.Save(ctx)
}

func (bb *Blackboard) IncrementTurn(ctx context.Context) (int, error) {
	bb.ConversationTurn++
	bb.AddLog(fmt.Sprintf("Blackboard conversation turn incremented to %d", bb.ConversationTurn))
	err := bb.Save(ctx)
	return bb.ConversationTurn, err
}

func (bb *Blackboard) AddLog(logLine string) {
	bb.Logs = append(bb.Logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), logLine))
	logger.Log.Debug("Blackboard Log", zap.String("request_id", bb.RequestID), zap.String("entry", logLine))
}
