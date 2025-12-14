package ai

import (
	"context"
	"fmt"
	"time"
	"strings"

	"aegis/pkg/ai/prompt"
	"aegis/pkg/events"
	"aegis/pkg/proc"
	"aegis/pkg/rules"
	"aegis/pkg/storage"
	"aegis/pkg/types"
	"aegis/pkg/utils"
)

// ExplainRequest represents an event explanation request.
type ExplainRequest struct {
	EventID   string      `json:"event_id"`   // 事件 ID
	EventData interface{} `json:"event_data"` // 事件详情（备选）
	Question  string      `json:"question"`   // 用户问题（可选）
}

// Action represents a suggested action.
type Action struct {
	Label    string                 `json:"label"`    // "转正", "调查", "忽略"
	ActionID string                 `json:"action_id"` // "promote", "investigate", "dismiss"
	Params   map[string]interface{} `json:"params"`   // 操作参数
}

// ExplainResponse contains the explanation and related information.
type ExplainResponse struct {
	Explanation     string           `json:"explanation"`     // 自然语言解释
	RootCause       string           `json:"root_cause"`   // 根本原因
	MatchedRule     *types.Rule      `json:"matched_rule"` // 触发的规则
	RelatedEvents   []*storage.Event `json:"related_events"` // 相关事件
	SuggestedActions []Action        `json:"suggested_actions"` // 建议操作
}

// ExplainEvent explains an event using AI analysis.
func (s *Service) ExplainEvent(
	ctx context.Context,
	req *ExplainRequest,
	event *storage.Event,
	ruleEngine *rules.Engine,
	store storage.EventStore,
	profileReg *proc.ProfileRegistry,
) (*ExplainResponse, error) {
	if !s.IsEnabled() {
		return nil, fmt.Errorf("AI service is not available")
	}

	// Get related events using indexer
	var relatedEvents []*storage.Event
	var pid uint32
	
	// Extract PID from event
	switch ev := event.Data.(type) {
	case *events.ExecEvent:
		pid = ev.Hdr.PID
	case *events.FileOpenEvent:
		pid = ev.Hdr.PID
	case *events.ConnectEvent:
		pid = ev.Hdr.PID
	case map[string]interface{}:
		// Frontend-normalized event shape
		if v, ok := ev["pid"].(float64); ok {
			pid = uint32(v)
		} else if hdr, ok := ev["header"].(map[string]interface{}); ok {
			if v, ok := hdr["pid"].(float64); ok {
				pid = uint32(v)
			}
		}
	}
	
	if store != nil && pid != 0 {
		// Query related events (simplified - would use indexer in real implementation)
		relatedEvents = []*storage.Event{event} // Placeholder
	}

	// Get process profile
	var profile *proc.ProcessProfile
	if profileReg != nil && pid != 0 {
		profile, _ = profileReg.GetProfile(pid)
	}

	// Build prompt (simplified - would use template engine in production)
	eventDesc := formatEventForExplanation(event, profile)
	question := req.Question
	if question == "" {
		question = "Explain this event"
	}
	userPrompt := fmt.Sprintf("Event details:\n%s\n\nUser question: \"%s\"\n\nExplain:", eventDesc, question)
	systemPrompt := prompt.ExplainSystemPrompt

	// Call AI service
	fullPrompt := systemPrompt + "\n\n" + userPrompt
	response, err := s.provider.SingleChat(ctx, fullPrompt)
	if err != nil {
		return nil, fmt.Errorf("AI inference failed: %w", err)
	}

	// Parse response (simplified - would parse structured response in production)
	explanation := response
	rootCause := extractRootCause(response)

	// Find matched rule (simplified)
	var matchedRule *types.Rule
	if ruleEngine != nil {
		// Would match event against rules here
	}

	// Generate suggested actions
	actions := generateSuggestedActions(event, matchedRule)

	return &ExplainResponse{
		Explanation:      explanation,
		RootCause:         rootCause,
		MatchedRule:       matchedRule,
		RelatedEvents:     relatedEvents,
		SuggestedActions: actions,
	}, nil
}

// formatEventForExplanation formats event data for AI explanation.
func formatEventForExplanation(event *storage.Event, profile *proc.ProcessProfile) string {
	var b strings.Builder
	b.WriteString("Event\n")
	b.WriteString(fmt.Sprintf("- Time: %s\n", event.Timestamp.Format(time.RFC3339)))

	// Prefer detailed formatting by inspecting both backend structs and frontend-normalized maps
	switch ev := event.Data.(type) {
	case *events.ExecEvent:
		b.WriteString("- Type: exec\n")
		b.WriteString(fmt.Sprintf("- PID: %d\n", ev.Hdr.PID))
		b.WriteString(fmt.Sprintf("- PPID: %d\n", ev.PPID))
		b.WriteString(fmt.Sprintf("- Comm: %s\n", strings.TrimRight(string(ev.Hdr.Comm[:]), "\x00")))
		b.WriteString(fmt.Sprintf("- ParentComm: %s\n", strings.TrimRight(string(ev.PComm[:]), "\x00")))
	case *events.FileOpenEvent:
		b.WriteString("- Type: file\n")
		b.WriteString(fmt.Sprintf("- PID: %d\n", ev.Hdr.PID))
		b.WriteString(fmt.Sprintf("- Comm: %s\n", strings.TrimRight(string(ev.Hdr.Comm[:]), "\x00")))
		b.WriteString(fmt.Sprintf("- Filename: %s\n", utils.ExtractCString(ev.Filename[:])))
		b.WriteString(fmt.Sprintf("- Flags: %d, Dev: %d, Ino: %d\n", ev.Flags, ev.Dev, ev.Ino))
	case *events.ConnectEvent:
		b.WriteString("- Type: connect\n")
		b.WriteString(fmt.Sprintf("- PID: %d\n", ev.Hdr.PID))
		b.WriteString(fmt.Sprintf("- Comm: %s\n", strings.TrimRight(string(ev.Hdr.Comm[:]), "\x00")))
		ip := utils.ExtractIP(ev)
		b.WriteString(fmt.Sprintf("- Remote: %s:%d (family=%d)\n", ip, ev.Port, ev.Family))
	case map[string]interface{}:
		// Frontend-normalized event shape
		typeStr, _ := ev["type"].(string)
		if typeStr == "" {
			// Best-effort map from storage.Event.Type
			switch event.Type {
			case events.EventTypeExec:
				typeStr = "exec"
			case events.EventTypeFileOpen:
				typeStr = "file"
			case events.EventTypeConnect:
				typeStr = "connect"
			}
		}
		b.WriteString(fmt.Sprintf("- Type: %s\n", typeStr))
		if hdr, ok := ev["header"].(map[string]interface{}); ok {
			if v, ok := hdr["pid"].(float64); ok { b.WriteString(fmt.Sprintf("- PID: %d\n", uint32(v))) }
			if comm, ok := hdr["comm"].(string); ok { b.WriteString(fmt.Sprintf("- Comm: %s\n", comm)) }
		}
		if v, ok := ev["pid"].(float64); ok { b.WriteString(fmt.Sprintf("- PID: %d\n", uint32(v))) }
		if comm, ok := ev["comm"].(string); ok { b.WriteString(fmt.Sprintf("- Comm: %s\n", comm)) }
		if parentComm, ok := ev["parentComm"].(string); ok { b.WriteString(fmt.Sprintf("- ParentComm: %s\n", parentComm)) }
		if filename, ok := ev["filename"].(string); ok && filename != "" { b.WriteString(fmt.Sprintf("- Filename: %s\n", filename)) }
		if addr, ok := ev["addr"].(string); ok && addr != "" {
			port := 0
			if p, ok := ev["port"].(float64); ok { port = int(p) }
			procName := ""
			if pn, ok := ev["processName"].(string); ok { procName = pn }
			if procName != "" { b.WriteString(fmt.Sprintf("- Process: %s\n", procName)) }
			b.WriteString(fmt.Sprintf("- Remote: %s:%d\n", addr, port))
		}
	default:
		b.WriteString(fmt.Sprintf("- Type: %d\n", int(event.Type)))
	}

	// Include profile summary if available
	if profile != nil {
		b.WriteString("\nProcess Profile\n")
		if !profile.Static.StartTime.IsZero() {
			b.WriteString(fmt.Sprintf("- StartTime: %s\n", profile.Static.StartTime.Format(time.RFC3339)))
		}
		if profile.Static.CommandLine != "" {
			b.WriteString(fmt.Sprintf("- CommandLine: %s\n", profile.Static.CommandLine))
		}
		b.WriteString(fmt.Sprintf("- ExecCount: %d, FileOpenCount: %d, NetConnectCount: %d\n", profile.Dynamic.ExecCount, profile.Dynamic.FileOpenCount, profile.Dynamic.NetConnectCount))
		if !profile.Dynamic.LastExec.IsZero() { b.WriteString(fmt.Sprintf("- LastExec: %s\n", profile.Dynamic.LastExec.Format(time.RFC3339))) }
		if !profile.Dynamic.LastFileOpen.IsZero() { b.WriteString(fmt.Sprintf("- LastFileOpen: %s\n", profile.Dynamic.LastFileOpen.Format(time.RFC3339))) }
		if !profile.Dynamic.LastConnect.IsZero() { b.WriteString(fmt.Sprintf("- LastConnect: %s\n", profile.Dynamic.LastConnect.Format(time.RFC3339))) }
	}

	return b.String()
}

// extractRootCause extracts root cause from AI response.
func extractRootCause(response string) string {
	// Simple extraction - look for "Root cause:" section
	// In production, use structured parsing
	return "Analysis pending"
}

// generateSuggestedActions generates suggested actions based on event and rule.
func generateSuggestedActions(event *storage.Event, rule *types.Rule) []Action {
	actions := []Action{}

	if rule != nil && rule.IsTesting() {
		actions = append(actions, Action{
			Label:    "转正规则",
			ActionID: "promote",
			Params:   map[string]interface{}{"rule_name": rule.Name},
		})
	}

	actions = append(actions, Action{
		Label:    "调查",
		ActionID: "investigate",
		Params:   map[string]interface{}{"event_id": fmt.Sprintf("%v", event)},
	})

	return actions
}

