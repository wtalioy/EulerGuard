package types

type StatsProvider interface {
	Rates() (exec, file, net int64)
	WorkloadCount() int
	TotalAlertCount() int64
	Alerts() []Alert
	// RecentExecs, RecentFiles, RecentConnects have been removed.
	// Event storage will be implemented in Phase 1 with TimeRingBuffer.
}
