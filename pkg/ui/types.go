package ui

import "eulerguard/pkg/types"

type (
	FrontendExecEvent    = types.ExecEvent
	FrontendConnectEvent = types.ConnectEvent
	FrontendFileEvent    = types.FileEvent
	FrontendAlert        = types.Alert
	ProcessInfoDTO       = types.ProcessInfo
	WorkloadDTO          = types.Workload
)

type SystemStatsDTO struct {
	ProcessCount  int     `json:"processCount"`
	WorkloadCount int     `json:"workloadCount"`
	EventsPerSec  float64 `json:"eventsPerSec"`
	AlertCount    int     `json:"alertCount"`
	ProbeStatus   string  `json:"probeStatus"` // "active", "error", "starting"
}

type RuleDTO struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Severity    string            `json:"severity"`
	Action      string            `json:"action"`
	Type        string            `json:"type"` // "exec", "file", "connect"
	Match       map[string]string `json:"match,omitempty"`
	YAML        string            `json:"yaml"`
	Selected    bool              `json:"selected,omitempty"`
}

type LearningStatusDTO struct {
	Active           bool  `json:"active"`
	StartTime        int64 `json:"startTime"`
	Duration         int   `json:"duration"` // seconds
	PatternCount     int   `json:"patternCount"`
	ExecCount        int   `json:"execCount"`
	FileCount        int   `json:"fileCount"`
	ConnectCount     int   `json:"connectCount"`
	RemainingSeconds int   `json:"remainingSeconds"`
}

type ProbeStatsDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Tracepoint string `json:"tracepoint"`
	Active     bool   `json:"active"`
	EventsRate int64  `json:"eventsRate"`
	TotalCount int64  `json:"totalCount"`
}
