package service

import (
	"fmt"
	"time"

	"aegis/pkg/ai/providers"
	"aegis/pkg/ai/snapshot"
	"aegis/pkg/ai/types"
	"aegis/pkg/metrics"
	"aegis/pkg/proc"
	"aegis/pkg/storage"
	"aegis/pkg/workload"
)

func (s *Service) requireProvider() (providers.Provider, error) {
	if s == nil || s.provider == nil {
		return nil, fmt.Errorf("AI service is not available")
	}
	return s.provider, nil
}

func (s *Service) buildSnapshot(
	statsProvider metrics.StatsProvider,
	workloadReg *workload.Registry,
	store storage.EventStore,
	processTree *proc.ProcessTree,
) snapshot.Result {
	return snapshot.NewSnapshot(statsProvider, workloadReg, store, processTree).Build()
}

func (s *Service) appendUserAndAssistant(sessionID, userMessage, assistantMessage string, ts int64) {
	if s == nil || s.conversations == nil {
		return
	}
	s.conversations.AddMessage(sessionID, types.Message{Role: "user", Content: userMessage, Timestamp: ts})
	s.conversations.AddMessage(sessionID, types.Message{Role: "assistant", Content: assistantMessage, Timestamp: ts})
}

func nowMs() int64 {
	return time.Now().UnixMilli()
}
