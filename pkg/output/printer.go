package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"eulerguard/pkg/events"
	"eulerguard/pkg/metrics"
	"eulerguard/pkg/proctree"
	"eulerguard/pkg/rules"
)

type Printer struct {
	jsonLines bool
	meter     *metrics.RateMeter
	logFile   *os.File
	writer    io.Writer
}

func NewPrinter(jsonLines bool, meter *metrics.RateMeter, logPath string) (*Printer, error) {
	// Check if log rotation is needed
	if err := rotateLogIfNeeded(logPath); err != nil {
		log.Printf("Warning: log rotation failed: %v", err)
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	p := &Printer{
		jsonLines: jsonLines,
		meter:     meter,
		logFile:   f,
		writer:    io.MultiWriter(os.Stdout, f),
	}

	log.Printf("Logging to file: %s", logPath)
	return p, nil
}

func (p *Printer) Close() error {
	if p.logFile != nil {
		return p.logFile.Close()
	}
	return nil
}

func (p *Printer) Print(ev events.ExecEvent) events.ProcessedEvent {
	// Extract comm from event (null-terminated C string)
	commBytes := ev.Comm[:]
	if idx := bytes.IndexByte(commBytes, 0); idx != -1 {
		commBytes = commBytes[:idx]
	}
	processName := string(commBytes)
	if processName == "" {
		processName = "unknown"
	}

	// Extract parent comm from event
	pcommBytes := ev.PComm[:]
	if idx := bytes.IndexByte(pcommBytes, 0); idx != -1 {
		pcommBytes = pcommBytes[:idx]
	}
	parentName := string(pcommBytes)
	if parentName == "" {
		parentName = "unknown"
	}

	meta := events.ProcessedEvent{
		Event:     ev,
		Timestamp: time.Now().UTC(),
		Process:   processName,
		Parent:    parentName,
		Rate:      p.meter.Tick(),
	}

	if p.jsonLines {
		enc := json.NewEncoder(p.writer)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(meta); err != nil {
			log.Printf("json encode failed: %v", err)
		}
		return meta
	}

	fmt.Fprintf(p.writer, "[%s] Process executed: PID=%d(%s) ← PPID=%d(%s) | Cgroup=%d | %.1f ev/s\n",
		meta.Timestamp.Format(time.RFC3339),
		meta.Event.PID, meta.Process,
		meta.Event.PPID, meta.Parent,
		meta.Event.CgroupID,
		meta.Rate)

	return meta
}

func (p *Printer) PrintAlert(alert rules.Alert) {
	if p.jsonLines {
		alertData := map[string]interface{}{
			"type":        "alert",
			"timestamp":   alert.Event.Timestamp.Format(time.RFC3339),
			"rule_name":   alert.Rule.Name,
			"severity":    alert.Rule.Severity,
			"description": alert.Message,
			"pid":         alert.Event.Event.PID,
			"process":     alert.Event.Process,
			"ppid":        alert.Event.Event.PPID,
			"parent":      alert.Event.Parent,
			"cgroup_id":   alert.Event.Event.CgroupID,
		}
		enc := json.NewEncoder(p.writer)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(alertData); err != nil {
			log.Printf("json encode alert failed: %v", err)
		}
		return
	}

	severityColor := getSeverityColor(alert.Rule.Severity)
	resetColor := "\033[0m"

	fmt.Fprintf(os.Stdout, "%s[Alert!] Rule '%s' triggered [Severity: %s]%s\n",
		severityColor,
		alert.Rule.Name,
		alert.Rule.Severity,
		resetColor)
	fmt.Fprintf(os.Stdout, "  Description: %s\n", alert.Message)
	fmt.Fprintf(os.Stdout, "  Process: PID=%d(%s) ← PPID=%d(%s) | Cgroup=%d\n",
		alert.Event.Event.PID, alert.Event.Process,
		alert.Event.Event.PPID, alert.Event.Parent,
		alert.Event.Event.CgroupID)

	fmt.Fprintf(p.logFile, "[Alert!] Rule '%s' triggered [Severity: %s]\n",
		alert.Rule.Name,
		alert.Rule.Severity)
	fmt.Fprintf(p.logFile, "  Description: %s\n", alert.Message)
	fmt.Fprintf(p.logFile, "  Process: PID=%d(%s) ← PPID=%d(%s) | Cgroup=%d\n",
		alert.Event.Event.PID, alert.Event.Process,
		alert.Event.Event.PPID, alert.Event.Parent,
		alert.Event.Event.CgroupID)
}

func (p *Printer) PrintFileOpenAlert(ev *events.FileOpenEvent, chain []*proctree.ProcessInfo, rule *rules.Rule) {
	filename := extractFilename(ev.Filename)
	severityColor := getSeverityColor(rule.Severity)
	resetColor := "\033[0m"

	if p.jsonLines {
		data := map[string]interface{}{
			"type":        "file_access_alert",
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
			"rule_name":   rule.Name,
			"severity":    rule.Severity,
			"description": rule.Description,
			"pid":         ev.PID,
			"filename":    filename,
			"cgroup_id":   ev.CgroupID,
			"flags":       ev.Flags,
			"chain":       formatChainJSON(chain),
		}
		enc := json.NewEncoder(p.writer)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(data); err != nil {
			log.Printf("json encode file alert failed: %v", err)
		}
		return
	}

	// Terminal output with colors
	fmt.Fprintf(os.Stdout, "%s[ALERT!] Rule '%s' triggered [Severity: %s]%s\n",
		severityColor, rule.Name, rule.Severity, resetColor)
	fmt.Fprintf(os.Stdout, "  Description: %s\n", rule.Description)
	fmt.Fprintf(os.Stdout, "  File: %s\n", filename)
	fmt.Fprintf(os.Stdout, "  PID: %d | Cgroup: %d | Flags: 0x%x\n", ev.PID, ev.CgroupID, ev.Flags)

	if len(chain) > 0 {
		fmt.Fprintf(os.Stdout, "  Attack Chain: %s\n", formatChain(chain))
	}

	// Log file (plain text)
	fmt.Fprintf(p.logFile, "[ALERT!] Rule '%s' triggered [Severity: %s]\n", rule.Name, rule.Severity)
	fmt.Fprintf(p.logFile, "  Description: %s\n", rule.Description)
	fmt.Fprintf(p.logFile, "  File: %s\n", filename)
	fmt.Fprintf(p.logFile, "  PID: %d | Cgroup: %d | Flags: 0x%x\n", ev.PID, ev.CgroupID, ev.Flags)
	if len(chain) > 0 {
		fmt.Fprintf(p.logFile, "  Attack Chain: %s\n", formatChain(chain))
	}
}

func formatChain(chain []*proctree.ProcessInfo) string {
	var parts []string
	for i := len(chain) - 1; i >= 0; i-- {
		info := chain[i]
		parts = append(parts, fmt.Sprintf("%s(%d)", info.Comm, info.PID))
	}
	return strings.Join(parts, " -> ")
}

func formatChainJSON(chain []*proctree.ProcessInfo) []map[string]interface{} {
	var result []map[string]interface{}
	for i := len(chain) - 1; i >= 0; i-- {
		info := chain[i]
		result = append(result, map[string]interface{}{
			"pid":       info.PID,
			"ppid":      info.PPID,
			"comm":      info.Comm,
			"cgroup_id": info.CgroupID,
		})
	}
	return result
}

func extractFilename(filename [256]byte) string {
	for i, b := range filename {
		if b == 0 {
			return string(filename[:i])
		}
	}
	return string(filename[:])
}

func getSeverityColor(severity string) string {
	switch severity {
	case "high", "critical":
		return "\033[1;31m" // Bold Red
	case "warning", "medium":
		return "\033[1;33m" // Bold Yellow
	case "info", "low":
		return "\033[1;36m" // Bold Cyan
	default:
		return "\033[1;37m" // Bold White
	}
}
