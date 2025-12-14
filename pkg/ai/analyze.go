package ai

import (
	"context"
	"fmt"
	"strconv"

	"aegis/pkg/ai/prompt"
	"aegis/pkg/proc"
	"aegis/pkg/rules"
	"aegis/pkg/types"
	"aegis/pkg/workload"
)

// AnalyzeRequest represents an analysis request.
type AnalyzeRequest struct {
	Type string `json:"type"` // "process", "workload", "rule"
	ID   string `json:"id"`   // PID, CgroupID, RuleName
}

// Anomaly represents an anomaly detected in analysis.
type Anomaly struct {
	Type        string  `json:"type"`        // "behavior_change", "unusual_pattern", etc.
	Description string  `json:"description"` // Description of the anomaly
	Severity    string  `json:"severity"`   // "low", "medium", "high", "critical"
	Confidence  float64 `json:"confidence"` // 0-1
	Evidence    []string `json:"evidence"`   // Supporting evidence
}

// Recommendation represents a recommendation from analysis.
type Recommendation struct {
	Type        string `json:"type"`        // "rule_creation", "investigation", "baseline_update"
	Description string `json:"description"` // Description of the recommendation
	Priority    string `json:"priority"`    // "low", "medium", "high"
	Action      Action `json:"action"`      // Suggested action
}

// RelatedInsight represents a related insight (different from Sentinel Insight).
type RelatedInsight struct {
	Type    string `json:"type"`    // "correlation", "pattern", "trend"
	Title   string `json:"title"`   // Insight title
	Summary string `json:"summary"` // Insight summary
}

// AnalyzeResponse contains the analysis results.
type AnalyzeResponse struct {
	Summary         string           `json:"summary"`         // 摘要
	Anomalies       []Anomaly        `json:"anomalies"`       // 异常点
	BaselineStatus  string           `json:"baseline_status"` // 基线状态
	Recommendations []Recommendation `json:"recommendations"` // 建议
	RelatedInsights []RelatedInsight `json:"related_insights"` // 相关洞察
}

// Analyze performs context analysis on a process, workload, or rule.
func (s *Service) Analyze(
	ctx context.Context,
	req *AnalyzeRequest,
	profileReg *proc.ProfileRegistry,
	workloadReg *workload.Registry,
	ruleEngine *rules.Engine,
) (*AnalyzeResponse, error) {
	if !s.IsEnabled() {
		return nil, fmt.Errorf("AI service is not available")
	}

	var analysisData string

	switch req.Type {
	case "process":
		pid, err := strconv.ParseUint(req.ID, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid PID: %w", err)
		}
		profile, ok := profileReg.GetProfile(uint32(pid))
		if !ok {
			return nil, fmt.Errorf("process profile not found for PID %d", pid)
		}
		analysisData = formatProcessProfile(profile)

	case "workload":
		cgroupID, err := strconv.ParseUint(req.ID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid CgroupID: %w", err)
		}
		w := workloadReg.Get(cgroupID)
		if w == nil {
			return nil, fmt.Errorf("workload not found for CgroupID %d", cgroupID)
		}
		analysisData = formatWorkloadMetadata(w)

	case "rule":
		allRules := ruleEngine.GetRules()
		var rule *types.Rule
		for i := range allRules {
			if allRules[i].Name == req.ID {
				rule = &allRules[i]
				break
			}
		}
		if rule == nil {
			return nil, fmt.Errorf("rule not found: %s", req.ID)
		}
		analysisData = formatRule(rule, ruleEngine)

	default:
		return nil, fmt.Errorf("unknown analysis type: %s", req.Type)
	}

	// Build prompt (simplified - would use template engine in production)
	userPrompt := fmt.Sprintf("Analysis request:\n- Type: %s\n- ID: %s\n\n%s\n\nAnalyze:", req.Type, req.ID, analysisData)
	systemPrompt := prompt.AnalyzeSystemPrompt

	// Call AI service
	fullPrompt := systemPrompt + "\n\n" + userPrompt
	response, err := s.provider.SingleChat(ctx, fullPrompt)
	if err != nil {
		return nil, fmt.Errorf("AI inference failed: %w", err)
	}

	// Parse response (simplified - would parse structured response in production)
	summary := response
	anomalies := detectAnomalies(req.Type, analysisData)
	recommendations := generateRecommendations(req.Type, analysisData)

	return &AnalyzeResponse{
		Summary:         summary,
		Anomalies:       anomalies,
		BaselineStatus:  "normal", // Would be calculated from baseline
		Recommendations: recommendations,
		RelatedInsights: []RelatedInsight{},
	}, nil
}

// formatProcessProfile formats process profile for AI analysis.
func formatProcessProfile(profile *proc.ProcessProfile) string {
	return fmt.Sprintf("PID: %d, ExecCount: %d, FileCount: %d, NetCount: %d",
		profile.PID, profile.Dynamic.ExecCount, profile.Dynamic.FileOpenCount, profile.Dynamic.NetConnectCount)
}

// formatWorkloadMetadata formats workload metadata for AI analysis.
func formatWorkloadMetadata(w *workload.Metadata) string {
	return fmt.Sprintf("CgroupID: %d, CgroupPath: %s, ExecCount: %d, FileCount: %d, ConnectCount: %d, AlertCount: %d",
		w.ID, w.CgroupPath, w.ExecCount, w.FileCount, w.ConnectCount, w.AlertCount)
}

// formatRule formats rule for AI analysis.
func formatRule(rule *types.Rule, engine *rules.Engine) string {
	testingBuffer := engine.GetTestingBuffer()
	stats := testingBuffer.GetStats(rule.Name)
	return fmt.Sprintf("Rule: %s, Mode: %s, Action: %s, Hits: %d",
		rule.Name, rule.State, rule.Action, stats.Hits)
}

// detectAnomalies detects anomalies in the analysis data.
func detectAnomalies(analysisType, data string) []Anomaly {
	// Simplified anomaly detection - would use more sophisticated logic in production
	return []Anomaly{}
}

// generateRecommendations generates recommendations based on analysis.
func generateRecommendations(analysisType, data string) []Recommendation {
	// Simplified recommendation generation - would use more sophisticated logic in production
	return []Recommendation{}
}

