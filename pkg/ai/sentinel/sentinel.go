package sentinel

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"aegis/pkg/ai/insights"
	"aegis/pkg/ai/types"
	"aegis/pkg/proc"
	"aegis/pkg/rules"
	"aegis/pkg/storage"
)

// SentinelAIClient is the minimal AI capability Sentinel depends on.
// It is intentionally smaller than the full Service surface to keep
// background monitoring decoupled from higher-level features.
type SentinelAIClient interface {
	IsEnabled() bool
	SingleChat(ctx context.Context, prompt string) (string, error)
}

type InsightType string

const (
	InsightTypeTestingPromotion InsightType = "testing_promotion"
	InsightTypeAnomaly          InsightType = "anomaly"
	InsightTypeOptimization     InsightType = "optimization"
	InsightTypeDailyReport      InsightType = "daily_report"
)

type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Insight represents a single Sentinel insight item.
//
// Note: The storage/pubsub layer lives in pkg/ai/insights.
type Insight struct {
	ID         string         `json:"id"`
	Type       InsightType    `json:"type"`
	Title      string         `json:"title"`
	Summary    string         `json:"summary"`
	Confidence float64        `json:"confidence"`
	Severity   Severity       `json:"severity"`
	Data       map[string]any `json:"data"`
	Actions    []types.Action `json:"actions"`
	CreatedAt  time.Time      `json:"created_at"`
}

type Sentinel struct {
	service    SentinelAIClient
	ruleEngine *rules.Engine
	store      storage.EventStore
	profileReg *proc.ProfileRegistry
	schedule   ScheduleConfig

	insights *insights.Store[*Insight]

	stopChan chan struct{}
	wg       sync.WaitGroup
}

func NewSentinel(
	service SentinelAIClient,
	ruleEngine *rules.Engine,
	store storage.EventStore,
	profileReg *proc.ProfileRegistry,
) *Sentinel {
	return &Sentinel{
		service:    service,
		ruleEngine: ruleEngine,
		store:      store,
		profileReg: profileReg,
		schedule:   defaultSchedule(),
		insights:   insights.NewStore[*Insight](),
		stopChan:   make(chan struct{}),
	}
}

// WithSchedule overrides the default schedule. If zero values are provided,
// the corresponding defaults are kept.
func (s *Sentinel) WithSchedule(cfg ScheduleConfig) *Sentinel {
	if cfg.TestingPromotion != 0 {
		s.schedule.TestingPromotion = cfg.TestingPromotion
	}
	if cfg.Anomaly != 0 {
		s.schedule.Anomaly = cfg.Anomaly
	}
	if cfg.RuleOptimization != 0 {
		s.schedule.RuleOptimization = cfg.RuleOptimization
	}
	if cfg.DailyReport != 0 {
		s.schedule.DailyReport = cfg.DailyReport
	}
	return s
}

func (s *Sentinel) Start() {
	// Clear any old insights when starting fresh
	s.insights.Reset()

	// Generate initial welcome insight with fresh timestamp
	s.generateWelcomeInsight()

	s.wg.Add(4)
	go s.runTask(s.checkTestingPromotion, s.schedule.TestingPromotion)
	go s.runTask(s.checkAnomalies, s.schedule.Anomaly)
	go s.runTask(s.checkRuleOptimization, s.schedule.RuleOptimization)
	go s.runTask(s.generateDailyReport, s.schedule.DailyReport)
}

func (s *Sentinel) generateWelcomeInsight() {
	now := time.Now()
	raw := insights.NewInsight(
		now,
		insights.NewInsightID("welcome", now),
		InsightTypeDailyReport,
		"AI Sentinel Active",
		"AI Sentinel is now monitoring your system. It will analyze security events, detect anomalies, and provide optimization suggestions. Insights will appear here as they are discovered.",
		SeverityLow,
	)
	insight := &Insight{
		ID:         raw.ID,
		Type:       raw.Type.(InsightType),
		Title:      raw.Title,
		Summary:    raw.Summary,
		Confidence: raw.Confidence,
		Severity:   raw.Severity.(Severity),
		Data:       raw.Data,
		Actions:    raw.Actions,
		CreatedAt:  raw.CreatedAt,
	}
	insight.Data["type"] = "welcome"
	s.addInsights([]*Insight{insight})
}

func (s *Sentinel) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}

func (s *Sentinel) Subscribe() insights.Subscription[*Insight] {
	return s.insights.Subscribe(100)
}

// Unsubscribe is kept for API compatibility with older callers.
// Prefer using the Subscription.Cancel() returned by Subscribe().

func (s *Sentinel) GetInsights(limit int) []*Insight {
	return s.insights.List(limit)
}

func (s *Sentinel) runTask(task func(context.Context) []*Insight, interval time.Duration) {
	defer s.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately on start
	ctx := context.Background()
	s.addInsights(task(ctx))

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			ctx := context.Background()
			s.addInsights(task(ctx))
		}
	}
}

func (s *Sentinel) addInsights(insights []*Insight) {
	s.insights.Add(insights, func(a, b *Insight) bool {
		// newest first
		return a.CreatedAt.After(b.CreatedAt)
	})
}

func (s *Sentinel) checkTestingPromotion(ctx context.Context) []*Insight {
	if s.ruleEngine == nil || !s.service.IsEnabled() {
		return nil
	}

	testingBuffer := s.ruleEngine.GetTestingBuffer()
	if testingBuffer == nil {
		return nil
	}

	allRules := s.ruleEngine.GetRules()
	out := make([]*Insight, 0)

	for _, rule := range allRules {
		if !rule.IsTesting() {
			continue
		}

		stats := testingBuffer.GetStats(rule.Name)
		if stats.Hits < 10 {
			continue // Not enough data
		}

		observationHours := float64(stats.ObservationMinutes) / 60.0
		if observationHours >= 1 && stats.Hits >= 5 {
			now := time.Now()
			id := fmt.Sprintf("testing-promotion-%s-%d", rule.Name, now.Unix())
			raw := insights.NewInsight(
				now,
				id,
				InsightTypeTestingPromotion,
				fmt.Sprintf("Testing Rule Ready for Promotion: %s", rule.Name),
				fmt.Sprintf("Rule '%s' has been running in Testing mode for %.1f hours with %d hits. Consider promoting it to Production mode.", rule.Name, observationHours, stats.Hits),
				SeverityMedium,
			)
			insight := &Insight{
				ID:         raw.ID,
				Type:       raw.Type.(InsightType),
				Title:      raw.Title,
				Summary:    raw.Summary,
				Confidence: raw.Confidence,
				Severity:   raw.Severity.(Severity),
				Data:       raw.Data,
				Actions:    raw.Actions,
				CreatedAt:  raw.CreatedAt,
			}
			insight.Confidence = 0.8
			insight.Data["rule_name"] = rule.Name
			insight.Data["hits"] = stats.Hits
			insight.Data["observation_hours"] = observationHours
			insight.Actions = []types.Action{
				{Label: "Promote to Production", ActionID: "promote", Params: map[string]any{"rule_name": rule.Name}},
				{Label: "Dismiss", ActionID: "dismiss", Params: map[string]any{"insight_id": id}},
			}
			out = append(out, insight)
		}
	}

	return out
}

func (s *Sentinel) checkAnomalies(ctx context.Context) []*Insight {
	if s.profileReg == nil || !s.service.IsEnabled() {
		return nil
	}

	// Simplified anomaly detection: emit a basic system status insight if rules exist.
	if s.ruleEngine == nil {
		return nil
	}

	rulesList := s.ruleEngine.GetRules()
	if len(rulesList) == 0 {
		return nil
	}

	// Avoid spamming: if we already have a recent status insight (within 5 minutes), skip.
	existing := s.insights.List(0)
	for _, in := range existing {
		if in == nil {
			continue
		}
		if in.Type == InsightTypeAnomaly && in.Title == "System Monitoring Active" {
			if time.Since(in.CreatedAt) < 5*time.Minute {
				return nil
			}
		}
	}

	now := time.Now()
	raw := insights.NewInsight(
		now,
		insights.NewInsightID("system-status", now),
		InsightTypeAnomaly,
		"System Monitoring Active",
		fmt.Sprintf("AI Sentinel is actively monitoring %d security rules. The system is being analyzed for anomalies and security events.", len(rulesList)),
		SeverityLow,
	)
	insight := &Insight{
		ID:         raw.ID,
		Type:       raw.Type.(InsightType),
		Title:      raw.Title,
		Summary:    raw.Summary,
		Confidence: 0.8,
		Severity:   raw.Severity.(Severity),
		Data:       raw.Data,
		Actions:    raw.Actions,
		CreatedAt:  raw.CreatedAt,
	}
	insight.Data["rule_count"] = len(rulesList)
	return []*Insight{insight}
}

func (s *Sentinel) checkRuleOptimization(ctx context.Context) []*Insight {
	if s.ruleEngine == nil || !s.service.IsEnabled() {
		return nil
	}

	allRules := s.ruleEngine.GetRules()
	if len(allRules) != 0 {
		return nil
	}

	now := time.Now()
	raw := insights.NewInsight(
		now,
		insights.NewInsightID("optimization-no-rules", now),
		InsightTypeOptimization,
		"No Security Rules Detected",
		"Your system currently has no active security rules. Consider creating rules to monitor and protect your system. You can use Policy Studio to create rules based on your security requirements.",
		SeverityMedium,
	)
	insight := &Insight{
		ID:         raw.ID,
		Type:       raw.Type.(InsightType),
		Title:      raw.Title,
		Summary:    raw.Summary,
		Confidence: raw.Confidence,
		Severity:   raw.Severity.(Severity),
		Data:       raw.Data,
		Actions:    raw.Actions,
		CreatedAt:  raw.CreatedAt,
	}
	insight.Actions = []types.Action{{Label: "Go to Policy Studio", ActionID: "navigate", Params: map[string]any{"page": "policy-studio"}}}
	insight.Data["rule_count"] = 0
	return []*Insight{insight}
}

func (s *Sentinel) generateDailyReport(ctx context.Context) []*Insight {
	if !s.service.IsEnabled() {
		return nil
	}

	reportPrompt := `Generate a daily security summary for the Aegis system.

Provide a concise, human-readable summary (not JSON) covering:
1. Overall system security status
2. Key security events or patterns observed
3. Any notable anomalies or concerns
4. Recommendations for the security team

Format your response as a clear, readable report using markdown. Use headers, bullet points, and clear language. Do not output JSON.

Keep it under 300 words and focus on actionable insights.`

	response, err := s.service.SingleChat(ctx, reportPrompt)
	if err != nil {
		return nil
	}

	summary := response
	if strings.Contains(summary, "```json") {
		parts := strings.Split(summary, "```json")
		if len(parts) > 0 {
			summary = strings.TrimSpace(parts[0])
		}
	}

	now := time.Now()
	id := fmt.Sprintf("daily-report-%d", now.Unix()/86400)
	raw := insights.NewInsight(
		now,
		id,
		InsightTypeDailyReport,
		"Daily Security Report",
		summary,
		SeverityLow,
	)
	insight := &Insight{
		ID:         raw.ID,
		Type:       raw.Type.(InsightType),
		Title:      raw.Title,
		Summary:    raw.Summary,
		Confidence: 0.9,
		Severity:   raw.Severity.(Severity),
		Data:       raw.Data,
		Actions:    raw.Actions,
		CreatedAt:  raw.CreatedAt,
	}
	return []*Insight{insight}
}
