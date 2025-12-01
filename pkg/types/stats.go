package types

type StatsProvider interface {
	Rates() (exec, file, net int64)
	WorkloadCount() int
	TotalAlertCount() int64
	Alerts() []Alert
	RecentExecs() []ExecEvent
	RecentFiles() []FileEvent
	RecentConnects() []ConnectEvent
}
