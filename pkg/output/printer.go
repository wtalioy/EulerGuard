package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"eulerguard/pkg/events"
	"eulerguard/pkg/metrics"
	"eulerguard/pkg/proc"
)

type Printer struct {
	jsonLines bool
	resolver  *proc.Resolver
	meter     *metrics.RateMeter
}

func NewPrinter(jsonLines bool, resolver *proc.Resolver, meter *metrics.RateMeter) *Printer {
	return &Printer{
		jsonLines: jsonLines,
		resolver:  resolver,
		meter:     meter,
	}
}

func (p *Printer) Print(ev events.ExecEvent) {
	// Extract comm from event (null-terminated C string)
	commBytes := ev.Comm[:]
	if idx := bytes.IndexByte(commBytes, 0); idx != -1 {
		commBytes = commBytes[:idx]
	}
	processName := string(commBytes)

	// Fallback to resolver if comm is empty
	if processName == "" {
		processName = p.resolver.Lookup(ev.PID)
	}

	// Extract parent comm from event
	pcommBytes := ev.PComm[:]
	if idx := bytes.IndexByte(pcommBytes, 0); idx != -1 {
		pcommBytes = pcommBytes[:idx]
	}
	parentName := string(pcommBytes)

	// Fallback to resolver if pcomm is empty
	if parentName == "" {
		parentName = p.resolver.Lookup(ev.PPID)
	}

	meta := events.ProcessedEvent{
		Event:     ev,
		Timestamp: time.Now().UTC(),
		Process:   processName,
		Parent:    parentName,
		Rate:      p.meter.Tick(),
	}

	if p.jsonLines {
		enc := json.NewEncoder(os.Stdout)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(meta); err != nil {
			log.Printf("json encode failed: %v", err)
		}
		return
	}

	fmt.Printf("[%s] Process executed: PID=%d(%s) ← PPID=%d(%s) | %.1f ev/s\n",
		meta.Timestamp.Format(time.RFC3339),
		meta.Event.PID, meta.Process,
		meta.Event.PPID, meta.Parent,
		meta.Rate)
}
