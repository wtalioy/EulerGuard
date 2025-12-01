package ai

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"eulerguard/pkg/types"
	"eulerguard/pkg/workload"
)

const (
	MaxAlertSummaries    = 10
	MaxActivitySummaries = 8
)

func BuildSnapshot(stats types.StatsProvider, workloadReg *workload.Registry, procTreeSize int) types.SystemSnapshot {
	execRate, fileRate, netRate := stats.Rates()
	totalRate := execRate + fileRate + netRate

	loadLevel := "normal"
	switch {
	case totalRate > 1000:
		loadLevel = "critical"
	case totalRate > 500:
		loadLevel = "high"
	case totalRate < 50:
		loadLevel = "low"
	}

	snapshot := types.SystemSnapshot{
		Timestamp:     time.Now(),
		LoadLevel:     loadLevel,
		ExecRate:      execRate,
		FileRate:      fileRate,
		NetworkRate:   netRate,
		ProcessCount:  procTreeSize,
		WorkloadCount: stats.WorkloadCount(),
		AlertCount:    int(stats.TotalAlertCount()),
	}

	// Get top workloads
	if workloadReg != nil {
		workloads := workloadReg.List()
		sort.Slice(workloads, func(i, j int) bool {
			totalI := workloads[i].ExecCount + workloads[i].FileCount + workloads[i].ConnectCount
			totalJ := workloads[j].ExecCount + workloads[j].FileCount + workloads[j].ConnectCount
			return totalI > totalJ
		})

		for i := 0; i < len(workloads) && i < 5; i++ {
			w := workloads[i]
			snapshot.TopWorkloads = append(snapshot.TopWorkloads, types.WorkloadSummary{
				ID:          fmt.Sprintf("%d", w.ID),
				CgroupPath:  w.CgroupPath,
				TotalEvents: w.ExecCount + w.FileCount + w.ConnectCount,
				AlertCount:  w.AlertCount,
			})
		}
	}

	snapshot.RecentAlerts = deduplicateAlerts(stats.Alerts())
	snapshot.RecentProcesses = buildProcessActivity(stats.RecentExecs())
	snapshot.RecentConnections = buildConnectionActivity(stats.RecentConnects())
	snapshot.RecentFileAccess = buildFileActivity(stats.RecentFiles())

	return snapshot
}

func buildProcessActivity(execs []types.ExecEvent) []types.ProcessActivity {
	groups := make(map[string]*types.ProcessActivity)

	for _, ev := range execs {
		key := ev.Comm + "|" + ev.ParentComm
		if existing, ok := groups[key]; ok {
			existing.Count++
			if ev.Blocked {
				existing.Blocked = true
			}
		} else {
			groups[key] = &types.ProcessActivity{
				Comm:       ev.Comm,
				ParentComm: ev.ParentComm,
				Count:      1,
				Blocked:    ev.Blocked,
			}
		}
	}

	return finalizeGroup(groups, MaxAlertSummaries, func(a, b types.ProcessActivity) bool {
		if a.Blocked != b.Blocked {
			return a.Blocked
		}
		return a.Count > b.Count
	})
}

func buildConnectionActivity(connects []types.ConnectEvent) []types.ConnectionActivity {
	groups := make(map[string]*types.ConnectionActivity)

	for _, ev := range connects {
		key := ev.Addr
		if existing, ok := groups[key]; ok {
			existing.Count++
			if ev.Blocked {
				existing.Blocked = true
			}
		} else {
			groups[key] = &types.ConnectionActivity{
				Destination: ev.Addr,
				Count:       1,
				Blocked:     ev.Blocked,
			}
		}
	}

	return finalizeGroup(groups, MaxActivitySummaries, func(a, b types.ConnectionActivity) bool {
		if a.Blocked != b.Blocked {
			return a.Blocked
		}
		return a.Count > b.Count
	})
}

func buildFileActivity(files []types.FileEvent) []types.FileActivity {
	groups := make(map[string]*types.FileActivity)

	for _, ev := range files {
		path := simplifyFilePath(ev.Filename)
		if existing, ok := groups[path]; ok {
			existing.Count++
			if ev.Blocked {
				existing.Blocked = true
			}
		} else {
			groups[path] = &types.FileActivity{
				Path:    path,
				Count:   1,
				Blocked: ev.Blocked,
			}
		}
	}

	return finalizeGroup(groups, MaxActivitySummaries, func(a, b types.FileActivity) bool {
		if a.Blocked != b.Blocked {
			return a.Blocked
		}
		return a.Count > b.Count
	})
}

func simplifyFilePath(path string) string {
	if strings.HasPrefix(path, "/etc/") ||
		strings.HasPrefix(path, "/root/") ||
		strings.HasPrefix(path, "/home/") {
		return path
	}

	if strings.HasPrefix(path, "/proc/") {
		parts := strings.Split(path, "/")
		if len(parts) > 3 {
			return "/proc/[pid]/" + strings.Join(parts[3:], "/")
		}
		return path
	}

	if strings.HasPrefix(path, "/tmp/") || strings.HasPrefix(path, "/var/") {
		parts := strings.Split(path, "/")
		if len(parts) > 3 {
			return "/" + parts[1] + "/" + parts[2] + "/..."
		}
	}

	return path
}

func deduplicateAlerts(alerts []types.Alert) []types.AlertSummary {
	groups := make(map[string]*types.AlertSummary)

	for _, alert := range alerts {
		key := alert.RuleName + "|" + alert.ProcessName
		if existing, ok := groups[key]; ok {
			existing.Count++
			if alert.Blocked {
				existing.WasBlocked = true
			}
		} else {
			groups[key] = &types.AlertSummary{
				RuleName:    alert.RuleName,
				Severity:    alert.Severity,
				ProcessName: alert.ProcessName,
				Count:       1,
				WasBlocked:  alert.Blocked,
			}
		}
	}

	return finalizeGroup(groups, MaxAlertSummaries, func(a, b types.AlertSummary) bool {
		if a.Severity != b.Severity {
			return severityOrder(a.Severity) > severityOrder(b.Severity)
		}
		return a.Count > b.Count
	})
}

func finalizeGroup[T any](groups map[string]*T, limit int, less func(a, b T) bool) []T {
	result := make([]T, 0, len(groups))
	for _, v := range groups {
		result = append(result, *v)
	}

	sort.Slice(result, func(i, j int) bool {
		return less(result[i], result[j])
	})

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result
}

func severityOrder(severity string) int {
	switch severity {
	case "critical":
		return 4
	case "high":
		return 3
	case "warning":
		return 2
	default:
		return 1
	}
}
