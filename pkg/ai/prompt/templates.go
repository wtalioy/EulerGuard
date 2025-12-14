package prompt

import (
	"sync"
)

// TokenBudget defines token budgets for different prompt types.
type TokenBudget struct {
	SystemPrompt int // 系统提示词预算
	Context      int // 上下文预算
	UserInput    int // 用户输入预算
	Response     int // 响应预算
}

// DefaultBudgets defines default token budgets for each prompt type.
var DefaultBudgets = map[string]TokenBudget{
	"intent":   {500, 200, 100, 200}, // 快速响应，小上下文
	"rulegen":  {800, 500, 200, 500}, // 中等复杂度
	"explain":  {600, 800, 100, 600}, // 需要较多上下文
	"sentinel": {400, 1000, 0, 400},  // 数据密集型
}

// PromptContext represents the context for prompt generation.
type PromptContext struct {
	CurrentPage        string
	SelectedItem       string
	RecentActions      []string
	Input              string
	ExistingRules      []string
	RecentBlocked      []string
	TargetWorkload     string
	EventType          string
	ProcessName        string
	PID                uint32
	ParentName         string
	Target             string
	Action             string
	RuleName           string
	ProcessHistory     []EventHistory
	RelatedProcesses   []RelatedProcess
	Type               string
	ID                 string
	CommandLine        string
	StartTime          string
	FileOpenCount      int64
	NetConnectCount    int64
	CgroupPath         string
	ProcessCount       int
	TotalEvents        int64
	TestingRuleName     string // For testing rule analysis
	ObservationMinutes int // Observation time in minutes
	TotalHits          int
	HitsByProcess      []ProcessHit
	SampleEvents       []SampleEvent
	BaselineFileRate   float64
	BaselineNetRate    float64
	BaselineFiles      []string
	CurrentFileRate    float64
	CurrentNetRate     float64
	UnusualFiles       []string
	UnusualConnections []string
}

// EventHistory represents a single event in process history.
type EventHistory struct {
	Timestamp   string
	Description string
}

// RelatedProcess represents a related process.
type RelatedProcess struct {
	Comm       string
	EventCount int
}

// ProcessHit represents hits by a process.
type ProcessHit struct {
	ProcessName string
	Count       int
}

// SampleEvent represents a sample matched event.
type SampleEvent struct {
	Timestamp   string
	ProcessName string
	Target      string
}

var (
	promptCache sync.Map // Cache for compiled templates
)

// CompressContext compresses context to fit within token budget.
// Priority: recent alerts > blocked events > regular events
func CompressContext(ctx *PromptContext, budget int) string {
	// Simplified implementation - Phase 3 will add full token counting
	// For now, just return a summary
	return "Context compressed for token budget"
}

// BatchAnalyze batches multiple analysis items into a single request.
func BatchAnalyze(items []AnalysisItem) []AnalysisResult {
	// Simplified implementation - Phase 3 will add full batching
	results := make([]AnalysisResult, len(items))
	for i := range items {
		results[i] = AnalysisResult{
			Item: items[i],
		}
	}
	return results
}

// AnalysisItem represents an item to analyze.
type AnalysisItem struct {
	Type string
	ID   string
}

// AnalysisResult represents the result of an analysis.
type AnalysisResult struct {
	Item AnalysisItem
	// Additional fields will be added in Phase 3
}
