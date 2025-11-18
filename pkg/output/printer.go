package output

import (
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
	meta := events.ProcessedEvent{
		Event:  ev,
		Timestamp:  time.Now().UTC(),
		Process:    p.resolver.Lookup(ev.PID),
		Parent: p.resolver.Lookup(ev.PPID),
		Rate:       p.meter.Tick(),
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