package storage

import (
	"sync"
	"sync/atomic"
	"time"

	"aegis/pkg/types"
)

// Stats provides stateless statistical aggregation for events.
// It does not store events, only counts and alerts.
type Stats struct {
	execCount    atomic.Int64
	fileCount    atomic.Int64
	connectCount atomic.Int64

	lastSecExec    atomic.Int64
	lastSecFile    atomic.Int64
	lastSecConnect atomic.Int64
	rateExec       atomic.Int64
	rateFile       atomic.Int64
	rateConnect    atomic.Int64

	alerts      []types.Alert
	alertsMu    sync.RWMutex
	maxAlerts   int
	totalAlerts atomic.Int64
	alertDedup  map[alertKey]time.Time
	dedupWindow time.Duration

	workloadCountFn func() int
}

type alertKey struct {
	RuleName    string
	ProcessName string
	CgroupID    string
	Action      string
}

// NewStats creates a new Stats instance.
func NewStats(maxAlerts int, dedupWindow time.Duration) *Stats {
	s := &Stats{
		alerts:      make([]types.Alert, 0, maxAlerts),
		maxAlerts:   maxAlerts,
		alertDedup:  make(map[alertKey]time.Time),
		dedupWindow: dedupWindow,
	}
	go s.rateLoop()
	return s
}

// SetWorkloadCountFunc sets the function to get workload count.
func (s *Stats) SetWorkloadCountFunc(fn func() int) {
	s.workloadCountFn = fn
}

func (s *Stats) rateLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		exec := s.lastSecExec.Swap(0)
		file := s.lastSecFile.Swap(0)
		net := s.lastSecConnect.Swap(0)

		s.rateExec.Store(exec)
		s.rateFile.Store(file)
		s.rateConnect.Store(net)
	}
}

// RecordExec records an exec event (count only, no storage).
func (s *Stats) RecordExec() {
	s.execCount.Add(1)
	s.lastSecExec.Add(1)
}

// RecordFile records a file event (count only, no storage).
func (s *Stats) RecordFile() {
	s.fileCount.Add(1)
	s.lastSecFile.Add(1)
}

// RecordConnect records a connect event (count only, no storage).
func (s *Stats) RecordConnect() {
	s.connectCount.Add(1)
	s.lastSecConnect.Add(1)
}

// AddAlert adds an alert with deduplication.
func (s *Stats) AddAlert(alert types.Alert) {
	s.alertsMu.Lock()
	now := time.Now()
	if s.dedupWindow > 0 {
		s.purgeDedupLocked(now)
		key := alertKey{
			RuleName:    alert.RuleName,
			ProcessName: alert.ProcessName,
			CgroupID:    alert.CgroupID,
			Action:      alert.Action,
		}
		if last, ok := s.alertDedup[key]; ok && now.Sub(last) < s.dedupWindow {
			s.alertsMu.Unlock()
			return
		}
		s.alertDedup[key] = now
	}
	if len(s.alerts) >= s.maxAlerts {
		s.alerts = s.alerts[1:]
	}
	s.alerts = append(s.alerts, alert)
	s.alertsMu.Unlock()
	s.totalAlerts.Add(1)
}

func (s *Stats) purgeDedupLocked(now time.Time) {
	if len(s.alertDedup) == 0 || s.dedupWindow <= 0 {
		return
	}
	expireBefore := now.Add(-s.dedupWindow)
	for key, ts := range s.alertDedup {
		if ts.Before(expireBefore) {
			delete(s.alertDedup, key)
		}
	}
}

// Rates returns the current event rates per second.
func (s *Stats) Rates() (exec, file, net int64) {
	return s.rateExec.Load(), s.rateFile.Load(), s.rateConnect.Load()
}

// Counts returns the total event counts.
func (s *Stats) Counts() (exec, file, net int64) {
	return s.execCount.Load(), s.fileCount.Load(), s.connectCount.Load()
}

// AlertCount returns the current number of alerts.
func (s *Stats) AlertCount() int {
	s.alertsMu.RLock()
	defer s.alertsMu.RUnlock()
	return len(s.alerts)
}

// TotalAlertCount returns the total number of alerts ever recorded.
func (s *Stats) TotalAlertCount() int64 {
	return s.totalAlerts.Load()
}

// Alerts returns a copy of all alerts.
func (s *Stats) Alerts() []types.Alert {
	s.alertsMu.RLock()
	defer s.alertsMu.RUnlock()
	result := make([]types.Alert, len(s.alerts))
	copy(result, s.alerts)
	return result
}

// WorkloadCount returns the workload count if a function is set.
func (s *Stats) WorkloadCount() int {
	if s.workloadCountFn != nil {
		return s.workloadCountFn()
	}
	return 0
}
