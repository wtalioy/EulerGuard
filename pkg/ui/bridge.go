package ui

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"eulerguard/pkg/events"
	"eulerguard/pkg/proctree"
	"eulerguard/pkg/rules"
	"eulerguard/pkg/utils"
)

type EventEmitter interface {
	Emit(eventName string, data any)
}
type NoopEmitter struct{}
func (n *NoopEmitter) Emit(string, any) {}

type Bridge struct {
	emitter     EventEmitter
	stats       *Stats
	processTree *proctree.ProcessTree
	ruleEngine  *rules.Engine
	mu          sync.RWMutex
}

var _ events.EventHandler = (*Bridge)(nil)

func NewBridge(stats *Stats) *Bridge {
	b := &Bridge{
		emitter: &NoopEmitter{},
		stats:   stats,
	}
	stats.SetRateCallback(func(exec, file, net int64) {
		b.emit("stats:rate", map[string]int64{
			"exec": exec, "file": file, "network": net,
		})
	})
	return b
}

func (b *Bridge) SetEmitter(emitter EventEmitter) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.emitter = emitter
}

func (b *Bridge) SetRuleEngine(pt *proctree.ProcessTree, re *rules.Engine) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.processTree = pt
	b.ruleEngine = re
}

func (b *Bridge) emit(name string, data any) {
	b.mu.RLock()
	e := b.emitter
	b.mu.RUnlock()
	if e != nil {
		e.Emit(name, data)
	}
}

func (b *Bridge) HandleExec(ev events.ExecEvent) {
	b.stats.RecordExec(ev)
	b.emit("event:exec", ExecToFrontend(ev))

	b.mu.RLock()
	re := b.ruleEngine
	b.mu.RUnlock()

	if re == nil {
		return
	}

	comm := utils.ExtractCString(ev.Comm[:])
	pcomm := utils.ExtractCString(ev.PComm[:])

	processed := events.ProcessedEvent{
		Event:     ev,
		Timestamp: time.Now().UTC(),
		Process:   comm,
		Parent:    pcomm,
	}

	for _, alert := range re.Match(processed) {
		b.emitAlert(FrontendAlert{
			ID:          fmt.Sprintf("exec-%d-%d", ev.PID, time.Now().UnixNano()),
			Timestamp:   time.Now().UnixMilli(),
			Severity:    alert.Rule.Severity,
			RuleName:    alert.Rule.Name,
			Description: alert.Rule.Description,
			PID:         ev.PID,
			ProcessName: comm,
			ParentName:  pcomm,
			CgroupID:    strconv.FormatUint(ev.CgroupID, 10),
			InContainer: ev.CgroupID > 1,
		})
	}
}

func (b *Bridge) HandleFileOpen(ev events.FileOpenEvent, filename string) {
	b.stats.RecordFile()
	b.emit("event:file", FileToFrontend(ev, filename))

	b.mu.RLock()
	re, pt := b.ruleEngine, b.processTree
	b.mu.RUnlock()

	if re == nil {
		return
	}

	matched, rule := re.MatchFile(filename, ev.PID, ev.CgroupID)
	if !matched || rule == nil {
		return
	}

	var processName string
	if pt != nil {
		if info, ok := pt.GetProcess(ev.PID); ok {
			processName = info.Comm
		}
	}

	b.emitAlert(FrontendAlert{
		ID:          fmt.Sprintf("file-%d-%d", ev.PID, time.Now().UnixNano()),
		Timestamp:   time.Now().UnixMilli(),
		Severity:    rule.Severity,
		RuleName:    rule.Name,
		Description: fmt.Sprintf("%s: %s", rule.Description, filename),
		PID:         ev.PID,
		ProcessName: processName,
		CgroupID:    strconv.FormatUint(ev.CgroupID, 10),
		InContainer: ev.CgroupID > 1,
	})
}

func (b *Bridge) HandleConnect(ev events.ConnectEvent) {
	b.stats.RecordConnect()
	b.emit("event:connect", ConnectToFrontend(ev, formatAddr(ev)))

	b.mu.RLock()
	re, pt := b.ruleEngine, b.processTree
	b.mu.RUnlock()

	if re == nil {
		return
	}

	matched, rule := re.MatchConnect(&ev)
	if !matched || rule == nil {
		return
	}

	var processName string
	if pt != nil {
		if info, ok := pt.GetProcess(ev.PID); ok {
			processName = info.Comm
		}
	}

	b.emitAlert(FrontendAlert{
		ID:          fmt.Sprintf("net-%d-%d", ev.PID, time.Now().UnixNano()),
		Timestamp:   time.Now().UnixMilli(),
		Severity:    rule.Severity,
		RuleName:    rule.Name,
		Description: rule.Description,
		PID:         ev.PID,
		ProcessName: processName,
		CgroupID:    strconv.FormatUint(ev.CgroupID, 10),
		InContainer: ev.CgroupID > 1,
	})
}

func (b *Bridge) emitAlert(alert FrontendAlert) {
	b.stats.AddAlert(alert)
	b.emit("alert:new", alert)
}

func formatAddr(ev events.ConnectEvent) string {
	switch ev.Family {
	case 2:
		ip := make(net.IP, 4)
		binary.LittleEndian.PutUint32(ip, ev.AddrV4)
		return fmt.Sprintf("%s:%d", ip.String(), ev.Port)
	case 10:
		return fmt.Sprintf("[%s]:%d", net.IP(ev.AddrV6[:]).String(), ev.Port)
	default:
		return fmt.Sprintf("unknown:%d", ev.Port)
	}
}
