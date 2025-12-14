package output

import (
	"aegis/pkg/events"
	"aegis/pkg/proc"
	"aegis/pkg/types"
	"time"
)

func (p *Printer) PrintAlert(alert types.MatchedAlert) {
	blocked := alert.Event.Event.Hdr.Blocked == 1
	action := string(alert.Rule.Action)

	if p.jsonLines {
		p.writeJSON(map[string]any{
			"type":        "alert",
			"timestamp":   alert.Event.Timestamp.Format(time.RFC3339),
			"rule_name":   alert.Rule.Name,
			"severity":    alert.Rule.Severity,
			"action":      action,
			"blocked":     blocked,
			"description": alert.Message,
			"pid":         alert.Event.Event.Hdr.PID,
			"process":     alert.Event.Process,
			"ppid":        alert.Event.Event.PPID,
			"parent":      alert.Event.Parent,
			"cgroup_id":   alert.Event.Event.Hdr.CgroupID,
		}, "exec alert")
		return
	}

	alertText := formatAlertText(alert.Rule.Name, alert.Rule.Severity, alert.Message,
		alert.Event.Event.Hdr.PID, alert.Event.Process,
		alert.Event.Event.PPID, alert.Event.Parent,
		alert.Event.Event.Hdr.CgroupID)

	if blocked {
		alertText = "[BLOCKED] " + alertText
	}

	p.emitColoredAlert(alert.Rule.Severity, alertText)
}

func (p *Printer) PrintFileOpenAlert(ev *events.FileOpenEvent, chain []*proc.ProcessInfo, rule *types.Rule, filename string) {
	blocked := ev.Hdr.Blocked == 1
	action := string(rule.Action)

	if p.jsonLines {
		p.writeJSON(map[string]any{
			"type":        "file_access_alert",
			"timestamp":   ev.Hdr.Timestamp().Format(time.RFC3339),
			"rule_name":   rule.Name,
			"severity":    rule.Severity,
			"action":      action,
			"blocked":     blocked,
			"description": rule.Description,
			"pid":         ev.Hdr.PID,
			"filename":    filename,
			"cgroup_id":   ev.Hdr.CgroupID,
			"flags":       ev.Flags,
			"chain":       formatChainJSON(chain),
		}, "file alert")
		return
	}

	alertText := formatFileAlertText(rule.Name, rule.Severity, rule.Description,
		filename, ev.Hdr.PID, ev.Hdr.CgroupID, ev.Flags, chain)

	if blocked {
		alertText = "[BLOCKED] " + alertText
	}

	p.emitColoredAlert(rule.Severity, alertText)
}

func (p *Printer) PrintConnectAlert(ev *events.ConnectEvent, chain []*proc.ProcessInfo, rule *types.Rule) {
	destAddr := formatAddress(ev)
	blocked := ev.Hdr.Blocked == 1
	action := string(rule.Action)

	if p.jsonLines {
		p.writeJSON(map[string]any{
			"type":        "network_connect_alert",
			"timestamp":   ev.Hdr.Timestamp().Format(time.RFC3339),
			"rule_name":   rule.Name,
			"severity":    rule.Severity,
			"action":      action,
			"blocked":     blocked,
			"description": rule.Description,
			"pid":         ev.Hdr.PID,
			"dest_addr":   destAddr,
			"dest_port":   ev.Port,
			"family":      ev.Family,
			"cgroup_id":   ev.Hdr.CgroupID,
			"chain":       formatChainJSON(chain),
		}, "connect alert")
		return
	}

	alertText := formatConnectAlertText(rule.Name, rule.Severity, rule.Description,
		destAddr, ev.Hdr.PID, ev.Hdr.CgroupID, chain)

	if blocked {
		alertText = "[BLOCKED] " + alertText
	}

	p.emitColoredAlert(rule.Severity, alertText)
}
